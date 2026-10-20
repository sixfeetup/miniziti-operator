package service

import (
	"reflect"
	"slices"
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

// These cases catch widening Bind permission, dropping the host, unstable roles,
// and mutation of caller-owned resolved IDs.
func TestRouterPoliciesAdditionalAllowedEdgeRouters(t *testing.T) {
	for _, tc := range []struct {
		name       string
		additional []string
		want       []string
	}{
		{"omitted", nil, []string{"@edge-router-1"}},
		{"empty", []string{}, []string{"@edge-router-1"}},
		{"additional", []string{"edge-router-2"}, []string{"@edge-router-1", "@edge-router-2"}},
		{"reversed and repeated", []string{"edge-router-3", "edge-router-2", "edge-router-1", "edge-router-2"}, []string{"@edge-router-1", "@edge-router-2", "@edge-router-3"}},
		{"padded IDs", []string{" edge-router-2 ", "edge-router-1"}, []string{"@edge-router-1", "@edge-router-2"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resource := &zitiv1alpha1.ZitiService{Spec: zitiv1alpha1.ZitiServiceSpec{
				Name: "argocd", Router: &zitiv1alpha1.ServiceRouterSpec{Name: "ziti-prod-router"},
			}}
			before := slices.Clone(tc.additional)
			policies, ok := RouterPolicies(resource, "service-1", "router-identity-1", "edge-router-1", tc.additional...)
			if !ok {
				t.Fatal("expected router policies")
			}
			if !reflect.DeepEqual(policies.ServiceEdgeRouterPolicy.EdgeRouterRoles, tc.want) {
				t.Fatalf("router roles = %v, want %v", policies.ServiceEdgeRouterPolicy.EdgeRouterRoles, tc.want)
			}
			if !reflect.DeepEqual(tc.additional, before) {
				t.Fatalf("mutated input: %v", tc.additional)
			}
			if !reflect.DeepEqual(policies.BindPolicy.IdentityRolesRaw, []string{"@router-identity-1"}) ||
				!reflect.DeepEqual(policies.BindPolicy.IdentityRoles, []string{"@router-identity-1"}) ||
				policies.BindPolicy.Type != "Bind" {
				t.Fatalf("Bind permission changed: %+v", policies.BindPolicy)
			}
			if !reflect.DeepEqual(policies.BindPolicy.ServiceRolesRaw, []string{"@service-1"}) ||
				!reflect.DeepEqual(policies.BindPolicy.ServiceRoles, []string{"@service-1"}) ||
				!reflect.DeepEqual(policies.ServiceEdgeRouterPolicy.ServiceRoles, []string{"@service-1"}) {
				t.Fatalf("service scope changed: %+v", policies)
			}
			if policies.BindPolicy.Name != "argocd-bind-policy" || policies.ServiceEdgeRouterPolicy.Name != "argocd-only" ||
				policies.BindPolicy.Semantic != "AnyOf" || policies.ServiceEdgeRouterPolicy.Semantic != "AnyOf" {
				t.Fatalf("policy identity/semantics changed: %+v", policies)
			}
		})
	}
}

func TestRouterPoliciesWithoutRouter(t *testing.T) {
	if _, ok := RouterPolicies(&zitiv1alpha1.ZitiService{}, "service-1", "identity-1", "router-1"); ok {
		t.Fatal("unexpected router policies without spec.router")
	}
}
