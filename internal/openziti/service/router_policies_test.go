package service

import (
	"testing"

	zitiv1alpha1 "example.com/miniziti-operator/api/v1alpha1"
)

func TestRouterPoliciesUsesResolvedServiceIdentityAndEdgeRouterIDs(t *testing.T) {
	resource := &zitiv1alpha1.ZitiService{}
	resource.Spec.Name = "argocd"
	resource.Spec.Router = &zitiv1alpha1.ServiceRouterSpec{Name: "ziti-prod-router"}

	policies, ok := RouterPolicies(resource, "service-1", "router-identity-1", "edge-router-1")
	if !ok {
		t.Fatal("expected router policies")
	}

	if policies.BindPolicy.Name != "argocd-bind-policy" {
		t.Fatalf("unexpected bind policy name %q", policies.BindPolicy.Name)
	}
	if policies.BindPolicy.Type != zitiv1alpha1.AccessPolicyTypeBind {
		t.Fatalf("unexpected bind policy type %q", policies.BindPolicy.Type)
	}
	if len(policies.BindPolicy.ServiceRolesRaw) != 1 || policies.BindPolicy.ServiceRolesRaw[0] != "@service-1" {
		t.Fatalf("unexpected bind service roles %#v", policies.BindPolicy.ServiceRolesRaw)
	}
	if len(policies.BindPolicy.IdentityRolesRaw) != 1 || policies.BindPolicy.IdentityRolesRaw[0] != "@router-identity-1" {
		t.Fatalf("unexpected bind identity roles %#v", policies.BindPolicy.IdentityRolesRaw)
	}

	if policies.ServiceEdgeRouterPolicy.Name != "argocd-only" {
		t.Fatalf("unexpected service edge router policy name %q", policies.ServiceEdgeRouterPolicy.Name)
	}
	if len(policies.ServiceEdgeRouterPolicy.ServiceRoles) != 1 || policies.ServiceEdgeRouterPolicy.ServiceRoles[0] != "@service-1" {
		t.Fatalf("unexpected service edge router service roles %#v", policies.ServiceEdgeRouterPolicy.ServiceRoles)
	}
	if len(policies.ServiceEdgeRouterPolicy.EdgeRouterRoles) != 1 || policies.ServiceEdgeRouterPolicy.EdgeRouterRoles[0] != "@edge-router-1" {
		t.Fatalf("unexpected edge router roles %#v", policies.ServiceEdgeRouterPolicy.EdgeRouterRoles)
	}
}
