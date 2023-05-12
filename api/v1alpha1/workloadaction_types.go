/*
Copyright 2023.

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

package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SecretKeyReferenceSpec represents a reference to a Secret resource in the same namespace
type SecretKeyReferenceSpec struct {
	// Name of the Secret.
	Name string `json:"name"`

	// Key in the Secret, when not specified an implementation-specific default key is used.
	Key string `json:"key,omitempty"`
}

// SynchronizationSpec defines the spec of the synchronization section of a WorkloadAction
type SynchronizationSpec struct {
	Time string `json:"time"`
}

// RabbitConnectionCredentialsSecretReferenceSpec TODO
type RabbitConnectionCredentialsSecretReferenceSpec struct {
	SecretRef SecretKeyReferenceSpec `json:"secretRef"`
}

// RabbitConnectionCredentialsSpec represents the credentials required to connect to RabbitMQ
type RabbitConnectionCredentialsSpec struct {
	Username RabbitConnectionCredentialsSecretReferenceSpec `json:"username"`
	Password RabbitConnectionCredentialsSecretReferenceSpec `json:"password"`
}

// RabbitConnectionSpec represents the connection settings to connect with RabbitMQ admin API
type RabbitConnectionSpec struct {
	Url         string                          `json:"url"`
	Vhost       string                          `json:"vhost"`
	Queue       string                          `json:"queue"`
	Credentials RabbitConnectionCredentialsSpec `json:"credentials,omitempty"`
}

// ConditionSpec represent a key/value pair found in the JSON of the HTTP response
type ConditionSpec struct {
	// Key represents the field, in GJSON dot notation, to reach some point of the JSON HTTP response
	Key string `json:"key"`

	// Value represents a string that must be equal to the string found on the Key
	Value string `json:"value"`
}

// WorkloadActionSpec defines the desired state of WorkloadAction
type WorkloadActionSpec struct {

	// SynchronizationSpec defines the behavior of synchronization
	Synchronization SynchronizationSpec `json:"synchronization"`

	// RabbitConnection represents the connection settings to connect with RabbitMQ admin API
	RabbitConnection RabbitConnectionSpec `json:"rabbitConnection"`

	// Sources TODO
	AdditionalSources []corev1.ObjectReference `json:"sources,omitempty"`

	// Condition represent a key/value pair found in the JSON of the HTTP response
	Condition ConditionSpec `json:"condition"`

	// Action represents what to do with the workload if the condition is met
	// +kubebuilder:validation:Enum=restart;delete
	Action string `json:"action"`

	// WorkloadRef represents a workload resource: deployment, statefulset, daemonset
	WorkloadRef corev1.ObjectReference `json:"workloadRef"`
}

// WorkloadActionStatus defines the observed state of WorkloadAction
type WorkloadActionStatus struct {
	// Conditions represent the latest available observations of an object's state
	Conditions []metav1.Condition `json:"conditions"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced,categories={workloadactions}
// +kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type==\"WorkloadActionReady\")].status",description=""
// +kubebuilder:printcolumn:name="Status",type="string",JSONPath=".status.conditions[?(@.type==\"WorkloadActionReady\")].reason",description=""
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description=""

// WorkloadAction is the Schema for the workloadactions API
// Ref: https://book.kubebuilder.io/reference/markers/crd-validation.html
type WorkloadAction struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   WorkloadActionSpec   `json:"spec,omitempty"`
	Status WorkloadActionStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// WorkloadActionList contains a list of WorkloadAction
type WorkloadActionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []WorkloadAction `json:"items"`
}

func init() {
	SchemeBuilder.Register(&WorkloadAction{}, &WorkloadActionList{})
}
