/*
Copyright The KubeDB Authors.

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
package framework

import (
	"fmt"
	"strings"
	"time"

	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha1"

	"github.com/appscode/go/types"
	shell "github.com/codeskyblue/go-sh"
	core "k8s.io/api/core/v1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	core_util "kmodules.xyz/client-go/core/v1"
)

const (
	updateRetryInterval = 10 * 1000 * 1000 * time.Nanosecond
	maxAttempts         = 5
)

func deleteInBackground() *metav1.DeleteOptions {
	policy := metav1.DeletePropagationBackground
	return &metav1.DeleteOptions{PropagationPolicy: &policy}
}

func deleteInForeground() *metav1.DeleteOptions {
	policy := metav1.DeletePropagationForeground
	return &metav1.DeleteOptions{PropagationPolicy: &policy}
}

func (f *Framework) CleanWorkloadLeftOvers() {
	// delete statefulset
	if err := f.kubeClient.AppsV1().StatefulSets(f.namespace).DeleteCollection(deleteInForeground(), metav1.ListOptions{
		LabelSelector: labels.SelectorFromSet(map[string]string{
			api.LabelDatabaseKind: api.ResourceKindMySQL,
		}).String(),
	}); err != nil && !kerr.IsNotFound(err) {
		fmt.Printf("error in deletion of Statefulset. Error: %v", err)
	}

	// delete pvc
	if err := f.kubeClient.CoreV1().PersistentVolumeClaims(f.namespace).DeleteCollection(deleteInForeground(), metav1.ListOptions{
		LabelSelector: labels.SelectorFromSet(map[string]string{
			api.LabelDatabaseKind: api.ResourceKindMySQL,
		}).String(),
	}); err != nil && !kerr.IsNotFound(err) {
		fmt.Printf("error in deletion of PVC. Error: %v", err)
	}

	// delete secret
	if err := f.kubeClient.CoreV1().Secrets(f.namespace).DeleteCollection(deleteInForeground(), metav1.ListOptions{
		LabelSelector: labels.SelectorFromSet(map[string]string{
			api.LabelDatabaseKind: api.ResourceKindMySQL,
		}).String(),
	}); err != nil && !kerr.IsNotFound(err) {
		fmt.Printf("error in deletion of Secret. Error: %v", err)
	}
}

func (f *Framework) WaitUntilPodRunningBySelector(mysql *api.MySQL) error {
	return core_util.WaitUntilPodRunningBySelector(
		f.kubeClient,
		mysql.Namespace,
		&metav1.LabelSelector{
			MatchLabels: mysql.OffshootSelectors(),
		},
		int(types.Int32(mysql.Spec.Replicas)),
	)
}

func isDebugTarget(containers []core.Container) (bool, []string) {
	for _, c := range containers {
		if c.Name == "stash" || c.Name == "stash-init" {
			return true, []string{"-c", c.Name}
		} else if strings.HasPrefix(c.Name, "update-status") {
			return true, []string{"--all-containers"}
		}
	}
	return false, nil
}

func (f *Framework) PrintDebugHelpers(mysqlName string, replicas int) {
	fmt.Println("\n============================== mysql replicas ========================")
	fmt.Println("\n=================================", replicas, "==============================")
	fmt.Println("\n======================================================================")

	sh := shell.NewSession()
	fmt.Println("\n======================================[ Describe Job ]===================================================")
	if err := sh.Command("/usr/bin/kubectl", "describe", "job", "-n", f.Namespace()).Run(); err != nil {
		fmt.Println(err)
	}

	fmt.Println("\n===============[ Debug info for Stash sidecar/init-container/backup job/restore job ]===================")
	if pods, err := f.kubeClient.CoreV1().Pods(f.Namespace()).List(metav1.ListOptions{}); err == nil {
		for _, pod := range pods.Items {
			debugTarget, containerArgs := isDebugTarget(append(pod.Spec.InitContainers, pod.Spec.Containers...))
			if debugTarget {
				fmt.Printf("\n--------------- Describe Pod: %s -------------------\n", pod.Name)
				if err := sh.Command("/usr/bin/kubectl", "describe", "po", "-n", f.Namespace(), pod.Name).Run(); err != nil {
					fmt.Println(err)
				}

				fmt.Printf("\n---------------- Log from Pod: %s ------------------\n", pod.Name)
				logArgs := []interface{}{"logs", "-n", f.Namespace(), pod.Name}
				for i := range containerArgs {
					logArgs = append(logArgs, containerArgs[i])
				}
				err = sh.Command("/usr/bin/kubectl", logArgs...).
					Command("cut", "-f", "4-", "-d ").
					Command("awk", `{$2=$2;print}`).
					Command("uniq").Run()
				if err != nil {
					fmt.Println(err)
				}
			}
		}
	} else {
		fmt.Println(err)
	}

	fmt.Println("\n======================================[ Describe Pod ]===================================================")
	if err := sh.Command("/usr/bin/kubectl", "describe", "po", "-n", f.Namespace()).Run(); err != nil {
		fmt.Println(err)
	}

	fmt.Println("\n======================================[ Describe MySQL ]===================================================")
	if err := sh.Command("/usr/bin/kubectl", "describe", "mysql", mysqlName, "-n", f.Namespace()).Run(); err != nil {
		fmt.Println(err)
	}

	fmt.Println("\n======================================[ MySQL Server Log ]===================================================")
	for i := 0; i < replicas; i++ {
		if err := sh.Command("/usr/bin/kubectl", "logs", fmt.Sprintf("%s-%d", mysqlName, i), "-n", f.Namespace()).Run(); err != nil {
			fmt.Println(err)
		}
	}

	fmt.Println("\n======================================[ Describe BackupSession ]==========================================")
	if err := sh.Command("/usr/bin/kubectl", "describe", "backupsession", "-n", f.Namespace()).Run(); err != nil {
		fmt.Println(err)
	}

	fmt.Println("\n======================================[ Describe RestoreSession ]==========================================")
	if err := sh.Command("/usr/bin/kubectl", "describe", "restoresession", "-n", f.Namespace()).Run(); err != nil {
		fmt.Println(err)
	}

	fmt.Println("\n======================================[ Describe Nodes ]===================================================")
	if err := sh.Command("/usr/bin/kubectl", "describe", "nodes").Run(); err != nil {
		fmt.Println(err)
	}
}
