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
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

// JoinConfiguration contains elements describing a particular node.
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ClusterConfiguration contains cluster-wide configuration for a kubeadm cluster.
type ClusterConfiguration struct {
	metav1.TypeMeta `json:",inline"`

	// cluster agent port (25000) and dqlite port (19001) set to use ports 30000 and 2379 respectively
	// The default ports of cluster agent and dqlite are blocked by security groups and as a temporary
	// workaround we reuse the etcd and calico ports that are open in the infra providers because kubeadm uses those.

	// PortCompatibilityRemap switches the default ports used by cluster agent (25000) and dqlite (19001)
	// to 30000 and 2379. The default ports are blocked via security groups in several infra providers.
	// +kubebuilder:default:=true
	// +optional
	PortCompatibilityRemap bool `json:"portCompatibilityRemap,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type InitConfiguration struct {
	metav1.TypeMeta `json:",inline"`

	// The join token will expire after the specified seconds, defaults to 10 years
	// +optional
	// +kubebuilder:default:=315569260
	// +kubebuilder:validation:Minimum:=1
	JoinTokenTTLInSecs int64 `json:"joinTokenTTLInSecs,omitempty"`

	// The optional https proxy configuration
	// +optional
	HTTPSProxy string `json:"httpsProxy,omitempty"`

	// The optional http proxy configuration
	// +optional
	HTTPProxy string `json:"httpProxy,omitempty"`

	// The optional no proxy configuration
	// +optional
	NoProxy string `json:"noProxy,omitempty"`

	// List of addons to be enabled upon cluster creation
	// +optional
	Addons []string `json:"addons,omitempty"`

	// The optional IPinIP configuration
	// +optional
	IPinIP bool `json:"IPinIP,omitempty"`

	// The confinement (strict or classic) configuration
	// +optional
	// +kubebuilder:validation:Enum=classic;strict
	Confinement string `json:"Confinement,omitempty"`
}

// MicroK8sConfigSpec defines the desired state of MicroK8sConfig
type MicroK8sConfigSpec struct {
	// InitConfiguration along with ClusterConfiguration are the configurations necessary for the init command
	// +optional
	ClusterConfiguration *ClusterConfiguration `json:"clusterConfiguration,omitempty"`

	InitConfiguration *InitConfiguration `json:"initConfiguration,omitempty"`
}

// MicroK8sConfigStatus defines the observed state of MicroK8sConfig
type MicroK8sConfigStatus struct {
	// Ready indicates the BootstrapData field is ready to be consumed
	Ready bool `json:"ready,omitempty"`

	// DataSecretName is the name of the secret that stores the bootstrap data script.
	// +optional
	DataSecretName *string `json:"dataSecretName,omitempty"`

	// FailureReason will be set on non-retryable errors
	// +optional
	FailureReason string `json:"failureReason,omitempty"`

	// FailureMessage will be set on non-retryable errors
	// +optional
	FailureMessage string `json:"failureMessage,omitempty"`

	// ObservedGeneration is the latest generation observed by the controller.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	// +optional
	Conditions clusterv1.Conditions `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:storageversion
// MicroK8sConfig is the Schema for the microk8sconfigs API
type MicroK8sConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   MicroK8sConfigSpec   `json:"spec,omitempty"`
	Status MicroK8sConfigStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// MicroK8sConfigList contains a list of MicroK8sConfig
type MicroK8sConfigList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []MicroK8sConfig `json:"items"`
}

func init() {
	SchemeBuilder.Register(&MicroK8sConfig{}, &MicroK8sConfigList{})
}

// GetConditions returns the set of conditions for this object.
func (c *MicroK8sConfig) GetConditions() clusterv1.Conditions {
	return c.Status.Conditions
}

// SetConditions sets the conditions on this object.
func (c *MicroK8sConfig) SetConditions(conditions clusterv1.Conditions) {
	c.Status.Conditions = conditions
}
