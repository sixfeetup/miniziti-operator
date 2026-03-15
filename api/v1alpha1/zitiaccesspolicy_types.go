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

const (
	AccessPolicyTypeDial = "Dial"
	AccessPolicyTypeBind = "Bind"
)

// SelectorSpec captures the supported identity and service selector fields.
// +kubebuilder:validation:XValidation:rule="(has(self.matchNames) && size(self.matchNames) > 0) || (has(self.matchRoleAttributes) && size(self.matchRoleAttributes) > 0)",message="selector must define at least one match field"
type SelectorSpec struct {
	// +optional
	// +listType=set
	MatchNames []string `json:"matchNames,omitempty"`
	// +optional
	// +listType=set
	MatchRoleAttributes []string `json:"matchRoleAttributes,omitempty"`
}

// ZitiAccessPolicySpec defines the desired state of ZitiAccessPolicy.
type ZitiAccessPolicySpec struct {
	// +kubebuilder:validation:Enum=Dial;Bind
	Type string `json:"type"`
	// IdentitySelector selects identities by explicit OpenZiti name and/or role attribute.
	IdentitySelector SelectorSpec `json:"identitySelector"`
	// ServiceSelector selects services by explicit OpenZiti name and/or role attribute.
	ServiceSelector SelectorSpec `json:"serviceSelector"`
}

// ZitiAccessPolicyStatus defines the observed state of ZitiAccessPolicy.
type ZitiAccessPolicyStatus struct {
	CommonStatus `json:",inline"`
	// +optional
	ResolvedIdentityCount int32 `json:"resolvedIdentityCount,omitempty"`
	// +optional
	ResolvedServiceCount int32 `json:"resolvedServiceCount,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=zitiaccesspolicies,scope=Namespaced,shortName=zap

// ZitiAccessPolicy is the Schema for the zitiaccesspolicies API.
type ZitiAccessPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ZitiAccessPolicySpec   `json:"spec,omitempty"`
	Status ZitiAccessPolicyStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ZitiAccessPolicyList contains a list of ZitiAccessPolicy.
type ZitiAccessPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ZitiAccessPolicy `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ZitiAccessPolicy{}, &ZitiAccessPolicyList{})
}
