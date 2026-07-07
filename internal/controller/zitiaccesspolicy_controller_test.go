package controller

import (
	"context"
	"reflect"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	zitiv1alpha1 "example.com/miniziti-operator/api/v1alpha1"
	openziti "example.com/miniziti-operator/internal/openziti/client"
	identityservice "example.com/miniziti-operator/internal/openziti/identity"
	policyservice "example.com/miniziti-operator/internal/openziti/policy"
)

type selectorBackendClient struct {
	openziti.FakeClient
	identities []openziti.Identity
}

func (c *selectorBackendClient) ListIdentities(context.Context) ([]openziti.Identity, error) {
	identities := make([]openziti.Identity, 0, len(c.identities))
	for _, identity := range c.identities {
		identity.RoleAttributes = append([]string(nil), identity.RoleAttributes...)
		identities = append(identities, identity)
	}
	return identities, nil
}

func (c *selectorBackendClient) FindIdentityByName(_ context.Context, name string) (*openziti.Identity, error) {
	for _, identity := range c.identities {
		if identity.Name == name {
			identity.RoleAttributes = append([]string(nil), identity.RoleAttributes...)
			return &identity, nil
		}
	}
	return nil, nil
}

func TestResolveSelectorsCountsBackendIdentityRoleAttributes(t *testing.T) {
	reconciler := newAccessPolicySelectorReconciler(
		t,
		[]openziti.Identity{{ID: "identity-admin", Name: "existing-admin", Type: "User", RoleAttributes: []string{"admin"}}},
		readyService("authentik-cr", "authentik", "service-authentik", "authentik"),
	)
	policy := accessPolicyWithSelectors(
		"backend-admin-dial",
		zitiv1alpha1.SelectorSpec{MatchRoleAttributes: []string{"admin"}},
		zitiv1alpha1.SelectorSpec{MatchNames: []string{"authentik"}},
	)

	identitySelector, serviceSelector, identityCount, serviceCount, err := reconciler.resolveSelectors(context.Background(), policy)
	if err != nil {
		t.Fatalf("resolve selectors: %v", err)
	}

	if identityCount != 1 {
		t.Fatalf("identityCount = %d, want 1", identityCount)
	}
	if len(identitySelector.IDs) != 0 {
		t.Fatalf("identitySelector.IDs = %v, want role selector without backend identity IDs", identitySelector.IDs)
	}
	if !reflect.DeepEqual(identitySelector.RoleAttributes, []string{"admin"}) {
		t.Fatalf("identitySelector.RoleAttributes = %v, want [admin]", identitySelector.RoleAttributes)
	}
	if serviceCount != 1 {
		t.Fatalf("serviceCount = %d, want 1", serviceCount)
	}
	if !reflect.DeepEqual(serviceSelector.IDs, []string{"service-authentik"}) {
		t.Fatalf("serviceSelector.IDs = %v, want [service-authentik]", serviceSelector.IDs)
	}
}

func TestResolveSelectorsResolvesBackendIdentityNames(t *testing.T) {
	reconciler := newAccessPolicySelectorReconciler(
		t,
		[]openziti.Identity{{ID: "identity-admin", Name: "existing-admin", Type: "User", RoleAttributes: []string{"admin"}}},
		readyService("authentik-cr", "authentik", "service-authentik", "authentik"),
	)
	policy := accessPolicyWithSelectors(
		"backend-name-dial",
		zitiv1alpha1.SelectorSpec{MatchNames: []string{"existing-admin"}},
		zitiv1alpha1.SelectorSpec{MatchNames: []string{"authentik"}},
	)

	identitySelector, _, identityCount, _, err := reconciler.resolveSelectors(context.Background(), policy)
	if err != nil {
		t.Fatalf("resolve selectors: %v", err)
	}

	if identityCount != 1 {
		t.Fatalf("identityCount = %d, want 1", identityCount)
	}
	if !reflect.DeepEqual(identitySelector.IDs, []string{"identity-admin"}) {
		t.Fatalf("identitySelector.IDs = %v, want [identity-admin]", identitySelector.IDs)
	}
}

func TestResolveSelectorsKeepsZeroMatchRoleSelectorsAtZero(t *testing.T) {
	reconciler := newAccessPolicySelectorReconciler(
		t,
		[]openziti.Identity{{ID: "identity-admin", Name: "existing-admin", Type: "User", RoleAttributes: []string{"admin"}}},
		readyService("authentik-cr", "authentik", "service-authentik", "authentik"),
	)
	policy := accessPolicyWithSelectors(
		"missing-role-dial",
		zitiv1alpha1.SelectorSpec{MatchRoleAttributes: []string{"missing-role"}},
		zitiv1alpha1.SelectorSpec{MatchNames: []string{"authentik"}},
	)

	_, _, identityCount, serviceCount, err := reconciler.resolveSelectors(context.Background(), policy)
	if err != nil {
		t.Fatalf("resolve selectors: %v", err)
	}

	if identityCount != 0 {
		t.Fatalf("identityCount = %d, want 0", identityCount)
	}
	if serviceCount != 1 {
		t.Fatalf("serviceCount = %d, want 1", serviceCount)
	}
}

func TestResolveSelectorsPreservesCRBackedIdentitySelectors(t *testing.T) {
	reconciler := newAccessPolicySelectorReconciler(
		t,
		nil,
		readyIdentity("alice-cr", "alice@example.com", "identity-alice", "devops"),
		readyService("argocd-cr", "argocd", "service-argocd", "gitops"),
	)
	policy := accessPolicyWithSelectors(
		"cr-backed-dial",
		zitiv1alpha1.SelectorSpec{MatchNames: []string{"alice@example.com"}, MatchRoleAttributes: []string{"devops"}},
		zitiv1alpha1.SelectorSpec{MatchNames: []string{"argocd"}},
	)

	identitySelector, serviceSelector, identityCount, serviceCount, err := reconciler.resolveSelectors(context.Background(), policy)
	if err != nil {
		t.Fatalf("resolve selectors: %v", err)
	}

	if identityCount != 1 {
		t.Fatalf("identityCount = %d, want 1", identityCount)
	}
	if !reflect.DeepEqual(identitySelector.IDs, []string{"identity-alice"}) {
		t.Fatalf("identitySelector.IDs = %v, want [identity-alice]", identitySelector.IDs)
	}
	if !reflect.DeepEqual(identitySelector.RoleAttributes, []string{"devops"}) {
		t.Fatalf("identitySelector.RoleAttributes = %v, want [devops]", identitySelector.RoleAttributes)
	}
	if serviceCount != 1 {
		t.Fatalf("serviceCount = %d, want 1", serviceCount)
	}
	if !reflect.DeepEqual(serviceSelector.IDs, []string{"service-argocd"}) {
		t.Fatalf("serviceSelector.IDs = %v, want [service-argocd]", serviceSelector.IDs)
	}
}

func newAccessPolicySelectorReconciler(t *testing.T, backendIdentities []openziti.Identity, objects ...runtime.Object) *ZitiAccessPolicyReconciler {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := zitiv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatalf("add scheme: %v", err)
	}
	backend := &selectorBackendClient{identities: backendIdentities}
	return &ZitiAccessPolicyReconciler{
		Client:          fake.NewClientBuilder().WithScheme(scheme).WithRuntimeObjects(objects...).Build(),
		Scheme:          scheme,
		IdentityService: identityservice.NewService(backend),
		PolicyService:   policyservice.NewService(backend),
	}
}

func accessPolicyWithSelectors(name string, identitySelector, serviceSelector zitiv1alpha1.SelectorSpec) *zitiv1alpha1.ZitiAccessPolicy {
	return &zitiv1alpha1.ZitiAccessPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
		Spec: zitiv1alpha1.ZitiAccessPolicySpec{
			Type:             zitiv1alpha1.AccessPolicyTypeDial,
			IdentitySelector: identitySelector,
			ServiceSelector:  serviceSelector,
		},
	}
}

func readyIdentity(k8sName, zitiName, backendID string, roleAttributes ...string) *zitiv1alpha1.ZitiIdentity {
	return &zitiv1alpha1.ZitiIdentity{
		ObjectMeta: metav1.ObjectMeta{Name: k8sName, Namespace: "default"},
		Spec: zitiv1alpha1.ZitiIdentitySpec{
			Name:           zitiName,
			Type:           zitiv1alpha1.IdentityTypeUser,
			RoleAttributes: append([]string(nil), roleAttributes...),
		},
		Status: zitiv1alpha1.ZitiIdentityStatus{CommonStatus: zitiv1alpha1.CommonStatus{ID: backendID}},
	}
}

func readyService(k8sName, zitiName, backendID string, roleAttributes ...string) *zitiv1alpha1.ZitiService {
	return &zitiv1alpha1.ZitiService{
		ObjectMeta: metav1.ObjectMeta{Name: k8sName, Namespace: "default"},
		Spec: zitiv1alpha1.ZitiServiceSpec{
			Name:           zitiName,
			RoleAttributes: append([]string(nil), roleAttributes...),
		},
		Status: zitiv1alpha1.ZitiServiceStatus{CommonStatus: zitiv1alpha1.CommonStatus{ID: backendID}},
	}
}
