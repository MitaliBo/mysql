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
	"time"

	api "kubedb.dev/apimachinery/apis/kubedb/v1alpha1"
	"kubedb.dev/apimachinery/pkg/controller"

	"github.com/appscode/go/wait"
	. "github.com/onsi/gomega"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	kutil "kmodules.xyz/client-go"
	appcat_api "kmodules.xyz/custom-resources/apis/appcatalog/v1alpha1"
	stash_util "stash.appscode.dev/stash/apis"
	"stash.appscode.dev/stash/apis/stash/v1alpha1"
	stashV1alpha1 "stash.appscode.dev/stash/apis/stash/v1alpha1"
	stashv1beta1 "stash.appscode.dev/stash/apis/stash/v1beta1"
	"stash.appscode.dev/stash/pkg/docker"
)

var (
	//StashMySQLBackupTask  = "my-backup-8.0.14"
	//StashMySQLRestoreTask = "my-restore-8.0.14"
	StashMySQLBackupTask  = "my-backup-" + DBCatalogName
	StashMySQLRestoreTask = "my-restore-" + DBCatalogName
)

const (
	MySQLBackupTask      = "mysql-backup-8.0.14"
	MySQLRestoreTask     = "mysql-restore-8.0.14"
	MySQLBackupFunction  = "mysql-backup-8.0.14"
	MySQLRestoreFunction = "mysql-restore-8.0.14"
)

func (f *Framework) FoundStashCRDs() bool {
	return controller.FoundStashCRDs(f.apiExtKubeClient)
}

func (f *Invocation) BackupConfiguration(meta metav1.ObjectMeta) *stashv1beta1.BackupConfiguration {
	return &stashv1beta1.BackupConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name:      meta.Name,
			Namespace: f.namespace,
		},
		Spec: stashv1beta1.BackupConfigurationSpec{
			Repository: core.LocalObjectReference{
				Name: meta.Name,
			},
			Schedule: "*/3 * * * *",
			RetentionPolicy: v1alpha1.RetentionPolicy{
				KeepLast: 5,
				Prune:    true,
			},
			BackupConfigurationTemplateSpec: stashv1beta1.BackupConfigurationTemplateSpec{
				Task: stashv1beta1.TaskRef{
					Name: MySQLBackupTask,
				},
				Target: &stashv1beta1.BackupTarget{
					Ref: stashv1beta1.TargetRef{
						APIVersion: appcat_api.SchemeGroupVersion.String(),
						Kind:       appcat_api.ResourceKindApp,
						Name:       meta.Name,
					},
				},
			},
		},
	}
}

func (f *Framework) CreateBackupConfiguration(backupCfg *stashv1beta1.BackupConfiguration) error {
	_, err := f.stashClient.StashV1beta1().BackupConfigurations(backupCfg.Namespace).Create(backupCfg)
	return err
}

func (f *Framework) DeleteBackupConfiguration(meta metav1.ObjectMeta) error {
	return f.stashClient.StashV1beta1().BackupConfigurations(meta.Namespace).Delete(meta.Name, &metav1.DeleteOptions{})
}

func (f *Framework) WaitUntilBackkupSessionBeCreated(bcMeta metav1.ObjectMeta) (bs *stashv1beta1.BackupSession, err error) {
	err = wait.PollImmediate(kutil.RetryInterval, kutil.ReadinessTimeout, func() (bool, error) {
		bsList, err := f.stashClient.StashV1beta1().BackupSessions(bcMeta.Namespace).List(metav1.ListOptions{
			LabelSelector: labels.Set{
				stash_util.LabelInvokerType: stashv1beta1.ResourceKindBackupConfiguration,
				stash_util.LabelInvokerName: bcMeta.Name,
			}.String(),
		})
		if err != nil {
			return false, err
		}
		if len(bsList.Items) == 0 {
			return false, nil
		}

		bs = &bsList.Items[0]

		return true, nil
	})

	return
}

func (f *Framework) EventuallyBackupSessionPhase(meta metav1.ObjectMeta) GomegaAsyncAssertion {
	return Eventually(
		func() (phase stashv1beta1.BackupSessionPhase) {
			bs, err := f.stashClient.StashV1beta1().BackupSessions(meta.Namespace).Get(meta.Name, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			return bs.Status.Phase
		},
	)
}

func (f *Invocation) Repository(meta metav1.ObjectMeta, secretName string) *stashV1alpha1.Repository {
	return &stashV1alpha1.Repository{
		ObjectMeta: metav1.ObjectMeta{
			Name:      meta.Name,
			Namespace: f.namespace,
		},
	}
}

func (f *Framework) CreateRepository(repo *stashV1alpha1.Repository) error {
	_, err := f.stashClient.StashV1alpha1().Repositories(repo.Namespace).Create(repo)
	return err
}

func (f *Framework) DeleteRepository(meta metav1.ObjectMeta) error {
	err := f.stashClient.StashV1alpha1().Repositories(meta.Namespace).Delete(meta.Name, deleteInBackground())
	return err
}

func (f *Invocation) RestoreSession(meta, oldMeta metav1.ObjectMeta) *stashv1beta1.RestoreSession {
	return &stashv1beta1.RestoreSession{
		ObjectMeta: metav1.ObjectMeta{
			Name:      meta.Name,
			Namespace: f.namespace,
			Labels: map[string]string{
				"app":                 f.app,
				api.LabelDatabaseKind: api.ResourceKindMySQL,
			},
		},
		Spec: stashv1beta1.RestoreSessionSpec{
			Task: stashv1beta1.TaskRef{
				Name: MySQLRestoreTask,
			},
			Repository: core.LocalObjectReference{
				Name: oldMeta.Name,
			},
			Rules: []stashv1beta1.Rule{
				{
					Snapshots: []string{"latest"},
				},
			},
			Target: &stashv1beta1.RestoreTarget{
				Ref: stashv1beta1.TargetRef{
					APIVersion: appcat_api.SchemeGroupVersion.String(),
					Kind:       appcat_api.ResourceKindApp,
					Name:       meta.Name,
				},
			},
		},
	}
}

func (f *Framework) CreateRestoreSession(restoreSession *stashv1beta1.RestoreSession) error {
	_, err := f.stashClient.StashV1beta1().RestoreSessions(restoreSession.Namespace).Create(restoreSession)
	return err
}

func (f *Framework) DeleteRestoreSession(meta metav1.ObjectMeta) error {
	err := f.stashClient.StashV1beta1().RestoreSessions(meta.Namespace).Delete(meta.Name, &metav1.DeleteOptions{})
	return err
}

func (f *Framework) EventuallyRestoreSessionPhase(meta metav1.ObjectMeta) GomegaAsyncAssertion {
	return Eventually(func() stashv1beta1.RestoreSessionPhase {
		restoreSession, err := f.stashClient.StashV1beta1().RestoreSessions(meta.Namespace).Get(meta.Name, metav1.GetOptions{})
		Expect(err).NotTo(HaveOccurred())
		return restoreSession.Status.Phase
	},
		time.Minute*7,
		time.Second*7,
	)
}

// ===========================

func (f *Framework) InstallMySQLAddon() error {
	image := docker.Docker{
		Image: "stash-mysql",
		//Registry: DockerRegistry,
		Registry: "appscodeci",
		Tag:      "8.0.14",
	}
	// create MySQL backup Function
	backupFunc := mysqlBackupFunction(image)
	_, err := f.stashClient.StashV1beta1().Functions().Create(backupFunc)
	if err != nil {
		return err
	}
	// create MySQL restore function
	restoreFunc := mysqlRestoreFunction(image)
	_, err = f.stashClient.StashV1beta1().Functions().Create(restoreFunc)
	if err != nil {
		return err
	}
	// create MySQL backup Task
	backupTask := mysqlBackupTask()
	_, err = f.stashClient.StashV1beta1().Tasks().Create(backupTask)
	if err != nil {
		return err
	}
	// create MySQL restore Task
	restoreTask := mysqlRestoreTask()
	_, err = f.stashClient.StashV1beta1().Tasks().Create(restoreTask)
	if err != nil {
		return err
	}
	return nil
}

func (f *Framework) UninstallMySQLAddon() error {
	// delete MySQL backup Function
	err := f.stashClient.StashV1beta1().Functions().Delete(MySQLBackupFunction, deleteInBackground())
	if err != nil {
		return err
	}
	// delete MySQL restore Function
	err = f.stashClient.StashV1beta1().Functions().Delete(MySQLRestoreFunction, deleteInBackground())
	if err != nil {
		return err
	}
	// delete MySQL backup Task
	err = f.stashClient.StashV1beta1().Tasks().Delete(MySQLBackupTask, deleteInBackground())
	if err != nil {
		return err
	}
	// delete MySQL restore Task
	return f.stashClient.StashV1beta1().Functions().Delete(MySQLRestoreTask, deleteInBackground())
}

func (f *Invocation) CheckMySQLAddonBeInstalled() error {
	_, err := f.stashClient.StashV1beta1().Functions().Get(MySQLBackupFunction, metav1.GetOptions{})
	if err != nil {
		return err
	}
	_, err = f.stashClient.StashV1beta1().Functions().Get(MySQLRestoreFunction, metav1.GetOptions{})
	if err != nil {
		return err
	}
	_, err = f.stashClient.StashV1beta1().Tasks().Get(MySQLBackupTask, metav1.GetOptions{})
	if err != nil {
		return err
	}
	_, err = f.stashClient.StashV1beta1().Tasks().Get(MySQLRestoreTask, metav1.GetOptions{})
	return err
}

func mysqlBackupFunction(image docker.Docker) *stashv1beta1.Function {
	return &stashv1beta1.Function{
		ObjectMeta: metav1.ObjectMeta{
			Name: MySQLBackupFunction,
		},
		Spec: stashv1beta1.FunctionSpec{
			Image: image.ToContainerImage(),
			Args: []string{
				"backup-mysql",
				// setup information
				"--provider=${REPOSITORY_PROVIDER:=}",
				"--bucket=${REPOSITORY_BUCKET:=}",
				"--endpoint=${REPOSITORY_ENDPOINT:=}",
				"--path=${REPOSITORY_PREFIX:=}",
				"--secret-dir=/etc/repository/secret",
				"--scratch-dir=/tmp",
				"--enable-cache=${ENABLE_CACHE:=true}",
				"--max-connections=${MAX_CONNECTIONS:=0}",
				"--hostname=${HOSTNAME:=}",
				"--mysql-args=${myArgs:=--all-databases}",
				// target information
				"--appbinding=${TARGET_NAME:=}",
				"--namespace=${NAMESPACE:=default}",
				// cleanup information
				"--retention-keep-last=${RETENTION_KEEP_LAST:=0}",
				"--retention-keep-hourly=${RETENTION_KEEP_HOURLY:=0}",
				"--retention-keep-daily=${RETENTION_KEEP_DAILY:=0}",
				"--retention-keep-weekly=${RETENTION_KEEP_WEEKLY:=0}",
				"--retention-keep-monthly=${RETENTION_KEEP_MONTHLY:=0}",
				"--retention-keep-yearly=${RETENTION_KEEP_YEARLY:=0}",
				"--retention-keep-tags=${RETENTION_KEEP_TAGS:=}",
				"--retention-prune=${RETENTION_PRUNE:=false}",
				"--retention-dry-run=${RETENTION_DRY_RUN:=false}",
				// output information
				"--output-dir=${outputDir:=}",
			},
			VolumeMounts: []core.VolumeMount{
				{
					Name:      "${secretVolume}",
					MountPath: "/etc/repository/secret",
				},
			},
		},
	}
}
func mysqlRestoreFunction(image docker.Docker) *stashv1beta1.Function {
	return &stashv1beta1.Function{
		ObjectMeta: metav1.ObjectMeta{
			Name: MySQLRestoreFunction,
		},
		Spec: stashv1beta1.FunctionSpec{
			Image: image.ToContainerImage(),
			Args: []string{
				"restore-mysql",
				// setup information
				"--provider=${REPOSITORY_PROVIDER:=}",
				"--bucket=${REPOSITORY_BUCKET:=}",
				"--endpoint=${REPOSITORY_ENDPOINT:=}",
				"--path=${REPOSITORY_PREFIX:=}",
				"--secret-dir=/etc/repository/secret",
				"--scratch-dir=/tmp",
				"--enable-cache=${ENABLE_CACHE:=true}",
				"--max-connections=${MAX_CONNECTIONS:=0}",
				"--hostname=${HOSTNAME:=}",
				"--source-hostname=${SOURCE_HOSTNAME:=}",
				"--mysql-args=${myArgs:=}",
				// target information
				"--appbinding=${TARGET_NAME:=}",
				"--namespace=${NAMESPACE:=default}",
				"--snapshot=${RESTORE_SNAPSHOTS:=}",
				// output information
				"--output-dir=${outputDir:=}",
			},
			VolumeMounts: []core.VolumeMount{
				{
					Name:      "${secretVolume}",
					MountPath: "/etc/repository/secret",
				},
			},
		},
	}
}
func mysqlBackupTask() *stashv1beta1.Task {
	return &stashv1beta1.Task{
		ObjectMeta: metav1.ObjectMeta{
			Name: MySQLBackupTask,
		},
		Spec: stashv1beta1.TaskSpec{
			Steps: []stashv1beta1.FunctionRef{
				{
					Name: MySQLBackupFunction,
					Params: []stashv1beta1.Param{
						{
							Name:  "outputDir",
							Value: "/tmp/output",
						},
						{
							Name:  "secretVolume",
							Value: "secret-volume",
						},
					},
				},
				{
					Name: "update-status",
					Params: []stashv1beta1.Param{
						{
							Name:  "outputDir",
							Value: "/tmp/output",
						},
					},
				},
			},
			Volumes: []core.Volume{
				{
					Name: "secret-volume",
					VolumeSource: core.VolumeSource{
						Secret: &core.SecretVolumeSource{
							SecretName: "${REPOSITORY_SECRET_NAME}",
						},
					},
				},
			},
		},
	}
}
func mysqlRestoreTask() *stashv1beta1.Task {
	return &stashv1beta1.Task{
		ObjectMeta: metav1.ObjectMeta{
			Name: MySQLRestoreTask,
		},
		Spec: stashv1beta1.TaskSpec{
			Steps: []stashv1beta1.FunctionRef{
				{
					Name: MySQLRestoreFunction,
					Params: []stashv1beta1.Param{
						{
							Name:  "outputDir",
							Value: "/tmp/output",
						},
						{
							Name:  "secretVolume",
							Value: "secret-volume",
						},
					},
				},
				{
					Name: "update-status",
					Params: []stashv1beta1.Param{
						{
							Name:  "outputDir",
							Value: "/tmp/output",
						},
					},
				},
			},
			Volumes: []core.Volume{
				{
					Name: "secret-volume",
					VolumeSource: core.VolumeSource{
						Secret: &core.SecretVolumeSource{
							SecretName: "${REPOSITORY_SECRET_NAME}",
						},
					},
				},
			},
		},
	}
}
