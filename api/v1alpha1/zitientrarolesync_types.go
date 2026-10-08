package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ZitiEntraRoleSyncSpec selects the Entra application and matching property.
type ZitiEntraRoleSyncSpec struct {
	// TenantID is the public-cloud Entra tenant GUID.
	// +kubebuilder:validation:Pattern=`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`
	TenantID string `json:"tenantId"`
	// AppID is the application whose group roles are synced.
	// +kubebuilder:validation:Pattern=`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`
	AppID string `json:"appId"`
	// CredentialsSecretRef names a local Secret with clientId and clientSecret.
	// +kubebuilder:validation:XValidation:rule="has(self.name) && size(self.name) > 0",message="a Secret name is required"
	CredentialsSecretRef corev1.LocalObjectReference `json:"credentialsSecretRef"`
	// UserProperty is compared case-insensitively with the identity externalId.
	// +kubebuilder:validation:Enum=mail;userPrincipalName;id
	// +kubebuilder:default=mail
	// +optional
	UserProperty string `json:"userProperty,omitempty"`
	// Interval is the delay between syncs.
	// +kubebuilder:default="10m"
	// +kubebuilder:validation:XValidation:rule="duration(self) >= duration('1m')",message="interval must be at least 1m"
	// +optional
	Interval string `json:"interval,omitempty"`
}

// EntraUnmatchedMember reports a member or identity that cannot be synced.
// +kubebuilder:validation:XValidation:rule="(has(self.id) && size(self.id) > 0) || (has(self.displayName) && size(self.displayName) > 0) || (has(self.externalId) && size(self.externalId) > 0)",message="an identifier is required"
type EntraUnmatchedMember struct {
	// +optional
	ID string `json:"id,omitempty"`
	// +optional
	DisplayName string `json:"displayName,omitempty"`
	// +optional
	ExternalID string `json:"externalId,omitempty"`
	// +kubebuilder:validation:MinLength=1
	Reason string `json:"reason"`
}

// ZitiEntraRoleSyncStatus records claims until retirement cleanup is confirmed.
type ZitiEntraRoleSyncStatus struct {
	CommonStatus `json:",inline"`
	// +optional
	LastSyncTime *metav1.Time `json:"lastSyncTime,omitempty"`
	// +optional
	// +listType=set
	ManagedAttributes []string `json:"managedAttributes,omitempty"`
	// +optional
	MatchedIdentities int `json:"matchedIdentities,omitempty"`
	// +optional
	IdentitiesUpdated int `json:"identitiesUpdated,omitempty"`
	// +optional
	MembersWithoutIdentity int `json:"membersWithoutIdentity,omitempty"`
	// +optional
	// +kubebuilder:validation:MaxItems=50
	UnmatchedMembers []EntraUnmatchedMember `json:"unmatchedMembers,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=zitientrarolesyncs,scope=Namespaced,shortName=zers
// +kubebuilder:printcolumn:name="Ready",type=string,JSONPath=`.status.conditions[?(@.type=="Ready")].status`
// +kubebuilder:printcolumn:name="Last Sync",type=date,JSONPath=`.status.lastSyncTime`

// ZitiEntraRoleSync keeps existing identity attributes in sync with group roles.
type ZitiEntraRoleSync struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              ZitiEntraRoleSyncSpec   `json:"spec"`
	Status            ZitiEntraRoleSyncStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// ZitiEntraRoleSyncList contains sync resources.
type ZitiEntraRoleSyncList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ZitiEntraRoleSync `json:"items"`
}

func init() { SchemeBuilder.Register(&ZitiEntraRoleSync{}, &ZitiEntraRoleSyncList{}) }
