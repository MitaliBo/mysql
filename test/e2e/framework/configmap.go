package framework

import (
	"strings"

	core "k8s.io/api/core/v1"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var initSQL = `
USE 'mysql'';

DROP TABLE IF EXISTS
    'kubedb_table'';

CREATE TABLE 'kubedb_table''(
    'id' BIGINT(20) NOT NULL,
    'name' VARCHAR(255) DEFAULT NULL
);

--
-- Dumping data for table 'kubedb_table'
--

INSERT INTO 'kubedb_table'('id', 'name')
VALUES(1, 'name1'),(2, 'name2'),(3, 'name3');

--
-- Indexes for table 'kubedb_table'
--

ALTER TABLE
    'kubedb_table' ADD PRIMARY KEY('id');

--
-- AUTO_INCREMENT for table 'kubedb_table'
--

ALTER TABLE
    'kubedb_table' MODIFY 'id' BIGINT(20) NOT NULL AUTO_INCREMENT,
    AUTO_INCREMENT = 4;
`

func (f *Framework) InitScriptConfigMap() *core.ConfigMap {
	return &core.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-init-script",
			Namespace: f.namespace,
		},
		Data: map[string]string{
			"init.sql": initSQL,
		},
	}
}

func (f *Invocation) GetCustomConfig(configs []string) *core.ConfigMap {
	configs = append([]string{"[mysqld]"}, configs...)
	return &core.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      f.app,
			Namespace: f.namespace,
		},
		Data: map[string]string{
			"my-custom.cnf": strings.Join(configs, "\n"),
		},
	}
}

func (f *Invocation) CreateConfigMap(obj *core.ConfigMap) error {
	_, err := f.kubeClient.CoreV1().ConfigMaps(obj.Namespace).Create(obj)
	return err
}

func (f *Framework) DeleteConfigMap(meta metav1.ObjectMeta) error {
	err := f.kubeClient.CoreV1().ConfigMaps(meta.Namespace).Delete(meta.Name, deleteInForeground())
	if !kerr.IsNotFound(err) {
		return err
	}
	return nil
}
