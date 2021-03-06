/*
Copyright The Kmodules Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1

import (
	"context"

	meta_util "kmodules.xyz/client-go/meta"

	"github.com/golang/glog"
	"github.com/pkg/errors"
	core "k8s.io/api/core/v1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/pager"
	kutil "kmodules.xyz/client-go"
)

func CreateOrPatchNode(c kubernetes.Interface, meta metav1.ObjectMeta, transform func(*core.Node) *core.Node) (*core.Node, kutil.VerbType, error) {
	cur, err := c.CoreV1().Nodes().Get(meta.Name, metav1.GetOptions{})
	if kerr.IsNotFound(err) {
		glog.V(3).Infof("Creating Node %s", meta.Name)
		out, err := c.CoreV1().Nodes().Create(transform(&core.Node{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Node",
				APIVersion: core.SchemeGroupVersion.String(),
			},
			ObjectMeta: meta,
		}))
		return out, kutil.VerbCreated, err
	} else if err != nil {
		return nil, kutil.VerbUnchanged, err
	}
	return PatchNode(c, cur, transform)
}

func PatchNode(c kubernetes.Interface, cur *core.Node, transform func(*core.Node) *core.Node) (*core.Node, kutil.VerbType, error) {
	return PatchNodeObject(c, cur, transform(cur.DeepCopy()))
}

func PatchNodeObject(c kubernetes.Interface, cur, mod *core.Node) (*core.Node, kutil.VerbType, error) {
	curJson, err := json.Marshal(cur)
	if err != nil {
		return nil, kutil.VerbUnchanged, err
	}

	modJson, err := json.Marshal(mod)
	if err != nil {
		return nil, kutil.VerbUnchanged, err
	}

	patch, err := strategicpatch.CreateTwoWayMergePatch(curJson, modJson, core.Node{})
	if err != nil {
		return nil, kutil.VerbUnchanged, err
	}
	if len(patch) == 0 || string(patch) == "{}" {
		return cur, kutil.VerbUnchanged, nil
	}
	glog.V(3).Infof("Patching Node %s with %s", cur.Name, string(patch))
	out, err := c.CoreV1().Nodes().Patch(cur.Name, types.StrategicMergePatchType, patch)
	return out, kutil.VerbPatched, err
}

func TryUpdateNode(c kubernetes.Interface, meta metav1.ObjectMeta, transform func(*core.Node) *core.Node) (result *core.Node, err error) {
	attempt := 0
	err = wait.PollImmediate(kutil.RetryInterval, kutil.RetryTimeout, func() (bool, error) {
		attempt++
		cur, e2 := c.CoreV1().Nodes().Get(meta.Name, metav1.GetOptions{})
		if kerr.IsNotFound(e2) {
			return false, e2
		} else if e2 == nil {
			result, e2 = c.CoreV1().Nodes().Update(transform(cur.DeepCopy()))
			return e2 == nil, nil
		}
		glog.Errorf("Attempt %d failed to update Node %s due to %v.", attempt, cur.Name, e2)
		return false, nil
	})

	if err != nil {
		err = errors.Errorf("failed to update Node %s after %d attempts due to %v", meta.Name, attempt, err)
	}
	return
}

// NodeReady returns whether a node is ready.
func NodeReady(node core.Node) bool {
	for _, cond := range node.Status.Conditions {
		if cond.Type != core.NodeReady {
			continue
		}
		return cond.Status == core.ConditionTrue
	}
	return false
}

// IsMaster returns whether a node is a master.
func IsMaster(node core.Node) bool {
	_, ok17 := node.Labels["node-role.kubernetes.io/master"]
	role16, ok16 := node.Labels["kubernetes.io/role"]
	return ok17 || (ok16 && role16 == "master")
}

func Topology(kc kubernetes.Interface) (regions map[string][]string, instances map[string]int, err error) {
	// TODO: Use https://github.com/kubernetes/client-go/blob/kubernetes-1.17.0/metadata/interface.go once upgraded to 1.17

	mapRegion := make(map[string]sets.String)
	instances = make(map[string]int)

	lister := pager.New(pager.SimplePageFunc(func(opts metav1.ListOptions) (runtime.Object, error) {
		return kc.CoreV1().Nodes().List(opts)
	}))
	err = lister.EachListItem(context.Background(), metav1.ListOptions{Limit: 100}, func(obj runtime.Object) error {
		m, err := meta.Accessor(obj)
		if err != nil {
			return err
		}

		annotations := m.GetAnnotations()

		os, _ := meta_util.GetStringVaultForKeys(annotations, "kubernetes.io/os", "beta.kubernetes.io/os")
		if os != "linux" {
			return nil
		}
		arch, _ := meta_util.GetStringVaultForKeys(annotations, "kubernetes.io/arch", "beta.kubernetes.io/arch")
		if arch != "amd64" {
			return nil
		}

		region, _ := meta_util.GetStringVaultForKeys(annotations, "topology.kubernetes.io/region", "failure-domain.beta.kubernetes.io/region")
		zone, _ := meta_util.GetStringVaultForKeys(annotations, "topology.kubernetes.io/zone", "failure-domain.beta.kubernetes.io/zone")
		if _, ok := mapRegion[region]; !ok {
			mapRegion[region] = sets.NewString()
		}
		mapRegion[region].Insert(zone)

		instance, _ := meta_util.GetStringVaultForKeys(annotations, "node.kubernetes.io/instance-type", "beta.kubernetes.io/instance-type")
		if n, ok := instances[instance]; ok {
			instances[instance] = n + 1
		} else {
			instances[instance] = 1
		}

		return nil
	})
	if err != nil {
		return nil, nil, err
	}

	regions = make(map[string][]string)
	for k, v := range mapRegion {
		regions[k] = v.List()
	}
	return regions, instances, nil
}
