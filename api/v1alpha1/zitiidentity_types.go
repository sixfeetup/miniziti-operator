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

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	IdentityTypeUser    = "User"
	IdentityTypeDevice  = "Device"
	IdentityTypeService = "Service"
)

// EnrollmentSpec captures the optional enrollment JWT output configuration.
type EnrollmentSpec struct {
	// CreateJWTSecret requests that the operator write enrollment material to a Secret.
	// +optional
	CreateJWTSecret bool `json:"createJwtSecret,omitempty"`
	// JWTSecretName is required when CreateJWTSecret is true.
	// +optional
	JWTSecretName string `json:"jwtSecretName,omitempty"`
}

// ZitiIdentitySpec defines the desired state of ZitiIdentity.
type ZitiIdentitySpec struct {
	// Name is the exact OpenZiti identity name.
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
	// Type is the OpenZiti identity type.
	// +kubebuilder:validation:Enum=User;Device;Service
	Type string `json:"type"`
	// RoleAttributes are propagated to OpenZiti for selector-based policy matching.
	// +optional
	// +listType=set
	RoleAttributes []string `json:"roleAttributes,omitempty"`
	// Enrollment controls optional JWT Secret output.
	// +optional
	Enrollment EnrollmentSpec `json:"enrollment,omitempty"`
}

// ZitiIdentityStatus defines the observed state of ZitiIdentity.
type ZitiIdentityStatus struct {
	CommonStatus `json:",inline"`
	// JWTSecretName records the generated Secret name when enrollment material is requested.
	// +optional
	JWTSecretName string `json:"jwtSecretName,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=zitiidentities,scope=Namespaced,shortName=zi

// ZitiIdentity is the Schema for the zitiidentities API.
type ZitiIdentity struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ZitiIdentitySpec   `json:"spec,omitempty"`
	Status ZitiIdentityStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ZitiIdentityList contains a list of ZitiIdentity.
type ZitiIdentityList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ZitiIdentity `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ZitiIdentity{}, &ZitiIdentityList{})
}
