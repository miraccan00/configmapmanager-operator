package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ConfigMapUpdate defines the update details for a ConfigMap key-value pair
type ConfigMapUpdate struct {
	Key      string `json:"key"`
	NewValue string `json:"newValue"`
}

// ConfigMapSpec defines the desired state of the ConfigMap
type ConfigMapSpec struct {
	Name    string            `json:"name"`
	Updates []ConfigMapUpdate `json:"updates"`
}

// ConfigMapManagerSpec defines the desired state of ConfigMapManager
type ConfigMapManagerSpec struct {
	ConfigMaps []ConfigMapSpec `json:"configMaps"`
}

// ConfigMapManagerStatus defines the observed state of ConfigMapManager
type ConfigMapManagerStatus struct {
	// Add status fields here if needed
}

// +kubebuilder:object:root=true

// ConfigMapManager is the Schema for the configmapmanagers API
type ConfigMapManager struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ConfigMapManagerSpec   `json:"spec,omitempty"`
	Status ConfigMapManagerStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ConfigMapManagerList contains a list of ConfigMapManager
type ConfigMapManagerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ConfigMapManager `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ConfigMapManager{}, &ConfigMapManagerList{})
}
