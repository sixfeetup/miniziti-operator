package policy

import (
	"testing"

	zitiv1alpha1 "example.com/miniziti-operator/api/v1alpha1"
)

func TestFromResourceResolvesMatchNamesToIDs(t *testing.T) {
	resource := &zitiv1alpha1.ZitiAccessPolicy{}
	resource.Name = "argocd-devops-dial"
	resource.Spec.Type = zitiv1alpha1.AccessPolicyTypeDial

	desired := FromResource(
		resource,
		ResolvedSelector{IDs: []string{"identity-1"}, RoleAttributes: []string{"devops"}},
		ResolvedSelector{IDs: []string{"service-1"}},
	)

	if len(desired.IdentityRolesRaw) != 2 {
		t.Fatalf("expected 2 identity roles, got %#v", desired.IdentityRolesRaw)
	}
	if desired.IdentityRolesRaw[0] != "@identity-1" || desired.IdentityRolesRaw[1] != "#devops" {
		t.Fatalf("unexpected identity roles %#v", desired.IdentityRolesRaw)
	}
	if len(desired.ServiceRolesRaw) != 1 || desired.ServiceRolesRaw[0] != "@service-1" {
		t.Fatalf("unexpected service roles %#v", desired.ServiceRolesRaw)
	}
}
