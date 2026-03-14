/*
Copyright 2026.

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

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// PortRange defines an inclusive low/high service port span.
type PortRange struct {
	// +kubebuilder:validation:Minimum=1
	Low int32 `json:"low"`
	// +kubebuilder:validation:Minimum=1
	High int32 `json:"high"`
}

// InterceptConfigSpec defines the intercept.v1 config subset supported in v1alpha1.
type InterceptConfigSpec struct {
	// +kubebuilder:validation:MinItems=1
	Protocols []string `json:"protocols"`
	// +kubebuilder:validation:MinItems=1
	Addresses []string `json:"addresses"`
	// +kubebuilder:validation:MinItems=1
	PortRanges []PortRange `json:"portRanges"`
}

// HostConfigSpec defines the host.v1 config subset supported in v1alpha1.
type HostConfigSpec struct {
	// +kubebuilder:validation:MinLength=1
	Protocol string `json:"protocol"`
	// +kubebuilder:validation:MinLength=1
	Address string `json:"address"`
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	Port int32 `json:"port"`
}

// ZitiServiceConfigsSpec captures the supported service config documents.
type ZitiServiceConfigsSpec struct {
	Intercept InterceptConfigSpec `json:"intercept"`
	Host      HostConfigSpec      `json:"host"`
}

// ZitiServiceSpec defines the desired state of ZitiService.
type ZitiServiceSpec struct {
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
	// +optional
	// +listType=set
	RoleAttributes []string               `json:"roleAttributes,omitempty"`
	Configs        ZitiServiceConfigsSpec `json:"configs"`
}

// ZitiServiceConfigIDs captures the managed external config identifiers.
type ZitiServiceConfigIDs struct {
	// +optional
	Intercept string `json:"intercept,omitempty"`
	// +optional
	Host string `json:"host,omitempty"`
}

// ZitiServiceStatus defines the observed state of ZitiService.
type ZitiServiceStatus struct {
	CommonStatus `json:",inline"`
	// +optional
	ConfigIDs ZitiServiceConfigIDs `json:"configIDs,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=zitiservices,scope=Namespaced,shortName=zs

// ZitiService is the Schema for the zitiservices API.
type ZitiService struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ZitiServiceSpec   `json:"spec,omitempty"`
	Status ZitiServiceStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ZitiServiceList contains a list of ZitiService.
type ZitiServiceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ZitiService `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ZitiService{}, &ZitiServiceList{})
}
