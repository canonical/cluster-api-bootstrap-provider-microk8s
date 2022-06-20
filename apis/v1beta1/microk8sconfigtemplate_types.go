/*
Copyright 2022.

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

package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.
type MicroK8sConfigTemplateResource struct {
	Spec MicroK8sConfigSpec `json:"spec,omitempty"`
}

// MicroK8sConfigTemplateSpec defines the desired state of MicroK8sConfigTemplate
type MicroK8sConfigTemplateSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	Template MicroK8sConfigTemplateResource `json:"template"`
}

// MicroK8sConfigTemplateStatus defines the observed state of MicroK8sConfigTemplate
type MicroK8sConfigTemplateStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:storageversion
// MicroK8sConfigTemplate is the Schema for the microk8sconfigtemplates API
type MicroK8sConfigTemplate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MicroK8sConfigTemplateSpec   `json:"spec,omitempty"`
	Status MicroK8sConfigTemplateStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// MicroK8sConfigTemplateList contains a list of MicroK8sConfigTemplate
type MicroK8sConfigTemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MicroK8sConfigTemplate `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MicroK8sConfigTemplate{}, &MicroK8sConfigTemplateList{})
}
