/*
Copyright The KubeDB Authors.

Licensed under the Apache License, ModificationRequest 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

const (
	ResourceCodeMemcachedModificationRequest     = "mcmodreq"
	ResourceKindMemcachedModificationRequest     = "MemcachedModificationRequest"
	ResourceSingularMemcachedModificationRequest = "memcachedmodificationrequest"
	ResourcePluralMemcachedModificationRequest   = "memcachedmodificationrequests"
)

// MemcachedModificationRequest defines a Memcached database version.

// +genclient
// +genclient:nonNamespaced
// +genclient:skipVerbs=updateStatus
// +k8s:openapi-gen=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// +kubebuilder:object:root=true
// +kubebuilder:resource:path=memcachedmodificationrequests,singular=memcachedmodificationrequest,shortName=mcmodreq,categories={datastore,kubedb,appscode}
type MemcachedModificationRequest struct {
	metav1.TypeMeta   `json:",inline,omitempty"`
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	Spec              MemcachedModificationRequestSpec   `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
	Status            MemcachedModificationRequestStatus `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
}

// MemcachedModificationRequestSpec is the spec for elasticsearch version
type MemcachedModificationRequestSpec struct {
}

// MemcachedModificationRequestStatus is the status for elasticsearch version
type MemcachedModificationRequestStatus struct {
	// Conditions applied to the request, such as approval or denial.
	// +optional
	Conditions []MemcachedModificationRequestCondition `json:"conditions,omitempty" protobuf:"bytes,1,rep,name=conditions"`
}

type MemcachedModificationRequestCondition struct {
	// request approval state, currently Approved or Denied.
	Type RequestConditionType `json:"type" protobuf:"bytes,1,opt,name=type,casttype=RequestConditionType"`

	// brief reason for the request state
	// +optional
	Reason string `json:"reason,omitempty" protobuf:"bytes,2,opt,name=reason"`

	// human readable message with details about the request state
	// +optional
	Message string `json:"message,omitempty" protobuf:"bytes,3,opt,name=message"`

	// timestamp for the last update to this condition
	// +optional
	LastUpdateTime metav1.Time `json:"lastUpdateTime,omitempty" protobuf:"bytes,4,opt,name=lastUpdateTime"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// MemcachedModificationRequestList is a list of MemcachedModificationRequests
type MemcachedModificationRequestList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`
	// Items is a list of MemcachedModificationRequest CRD objects
	Items []MemcachedModificationRequest `json:"items,omitempty" protobuf:"bytes,2,rep,name=items"`
}
