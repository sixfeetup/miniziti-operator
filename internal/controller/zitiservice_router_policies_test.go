package controller

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	zitiv1alpha1 "example.com/miniziti-operator/api/v1alpha1"
	openziti "example.com/miniziti-operator/internal/openziti/client"
	zitiservice "example.com/miniziti-operator/internal/openziti/service"
)

// A client edge router need not have an identity or any Bind permission.
func TestDesiredRouterPoliciesResolvesAdditionalAllowedEdgeRouters(t *testing.T) {
	for _, names := range [][]string{
		nil, {}, {"ziti-client-router"},
		{" ziti-client-router ", "ziti-prod-router", "ziti-client-router", "same-router"},
	} {
		t.Run(strings.Join(names, ","), func(t *testing.T) {
			var identities, routers []string
			backend := routerResolutionBackend()
			identityLookup, routerLookup := backend.FindIdentityByNameFunc, backend.FindEdgeRouterByNameFunc
			backend.FindIdentityByNameFunc = func(ctx context.Context, name string) (*openziti.Identity, error) {
				identities = append(identities, name)
				return identityLookup(ctx, name)
			}
			backend.FindEdgeRouterByNameFunc = func(ctx context.Context, name string) (*openziti.EdgeRouter, error) {
				routers = append(routers, name)
				return routerLookup(ctx, name)
			}
			r := &ZitiServiceReconciler{ServiceManager: zitiservice.NewService(backend)}
			resource := routerResolutionResource(names)
			resource.Spec.Router.Name = " ziti-prod-router "
			policies, err := r.desiredRouterPolicies(context.Background(), resource, "service-1")
			if err != nil {
				t.Fatal(err)
			}
			wantRoles, wantRouters := []string{"@edge-router-1"}, []string{"ziti-prod-router"}
			if len(names) > 0 {
				wantRoles = append(wantRoles, "@edge-router-2")
				wantRouters = append(wantRouters, "ziti-client-router")
				if slices.Contains(names, "same-router") {
					wantRouters = append(wantRouters, "same-router")
				}
			}
			if !reflect.DeepEqual(policies.ServiceEdgeRouterPolicy.EdgeRouterRoles, wantRoles) ||
				!reflect.DeepEqual(policies.BindPolicy.IdentityRolesRaw, []string{"@router-identity-1"}) {
				t.Fatalf("unexpected policies: %+v", policies)
			}
			if !reflect.DeepEqual(identities, []string{"ziti-prod-router"}) || !reflect.DeepEqual(routers, wantRouters) {
				t.Fatalf("lookups: identities=%v routers=%v", identities, routers)
			}
		})
	}
}

// These failures must return no partial policy set, even after a valid lookup.
func TestDesiredRouterPoliciesRejectsUnresolvedRouters(t *testing.T) {
	for _, tc := range []struct {
		name      string
		identity  bool
		result    string
		lookupErr error
	}{
		{name: "unknown"},
		{name: "empty-id", result: ""},
		{name: "blank-id", result: "  "},
		{name: "lookup-error", lookupErr: errors.New("backend unavailable")},
		{name: "ziti-prod-router", result: ""},
		{name: "ziti-prod-router", lookupErr: errors.New("router backend unavailable")},
		{name: "ziti-prod-router", identity: true, result: ""},
		{name: "ziti-prod-router", identity: true, lookupErr: errors.New("identity backend unavailable")},
		{name: " "},
	} {
		t.Run(tc.name, func(t *testing.T) {
			backend := routerResolutionBackend()
			if tc.identity {
				backend.FindIdentityByNameFunc = func(context.Context, string) (*openziti.Identity, error) {
					return &openziti.Identity{ID: tc.result, Type: "Router"}, tc.lookupErr
				}
			} else {
				lookup := backend.FindEdgeRouterByNameFunc
				backend.FindEdgeRouterByNameFunc = func(ctx context.Context, name string) (*openziti.EdgeRouter, error) {
					if name == tc.name {
						if tc.name == "unknown" {
							return nil, nil
						}
						return &openziti.EdgeRouter{ID: tc.result, Name: name}, tc.lookupErr
					}
					return lookup(ctx, name)
				}
			}
			r := &ZitiServiceReconciler{ServiceManager: zitiservice.NewService(backend)}
			policies, err := r.desiredRouterPolicies(context.Background(), routerResolutionResource([]string{"ziti-client-router", tc.name}), "service-1")
			if err == nil {
				t.Fatalf("expected failure, got %+v", policies)
			}
			if strings.TrimSpace(tc.name) != "" && !strings.Contains(err.Error(), tc.name) {
				t.Fatalf("error does not identify failing name: %v", err)
			}
			if tc.lookupErr != nil && !errors.Is(err, tc.lookupErr) {
				t.Fatalf("lost lookup error: %v", err)
			}
			if !reflect.DeepEqual(policies, zitiservice.RouterPolicySet{}) {
				t.Fatalf("partial policies: %+v", policies)
			}
		})
	}
}

func routerResolutionBackend() *openziti.FakeClient {
	return &openziti.FakeClient{
		FindIdentityByNameFunc: func(_ context.Context, name string) (*openziti.Identity, error) {
			if name == "ziti-prod-router" {
				return &openziti.Identity{ID: "router-identity-1", Name: name, Type: "Router"}, nil
			}
			return nil, nil
		},
		FindEdgeRouterByNameFunc: func(_ context.Context, name string) (*openziti.EdgeRouter, error) {
			switch name {
			case "ziti-prod-router":
				return &openziti.EdgeRouter{ID: "edge-router-1", Name: name}, nil
			case "ziti-client-router", "same-router":
				return &openziti.EdgeRouter{ID: "edge-router-2", Name: name}, nil
			default:
				return nil, nil
			}
		},
	}
}

func routerResolutionResource(names []string) *zitiv1alpha1.ZitiService {
	return &zitiv1alpha1.ZitiService{
		ObjectMeta: metav1.ObjectMeta{Name: "argocd", Namespace: "default", Finalizers: []string{zitiServiceFinalizer}},
		Spec: zitiv1alpha1.ZitiServiceSpec{Name: "argocd", Router: &zitiv1alpha1.ServiceRouterSpec{
			Name: "ziti-prod-router", AdditionalAllowedEdgeRouterNames: names,
		}},
	}
}

// Writing Bind before all lookups succeed must fail this reconcile-level test.
func TestRouterPoliciesResolutionFailureDoesNotWritePolicies(t *testing.T) {
	ctx := context.Background()
	resource := routerResolutionResource([]string{"ziti-client-router", "missing-router"})
	resource.Status.ID = "service-1"
	resource.Status.ConfigIDs = zitiv1alpha1.ZitiServiceConfigIDs{Intercept: "intercept-1", Host: "host-1"}
	scheme := runtime.NewScheme()
	if err := zitiv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	k8s := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(resource).WithObjects(resource).Build()
	backend := routerResolutionBackend()
	backend.UpdateServiceFunc = func(_ context.Context, service openziti.Service) (*openziti.Service, error) { return &service, nil }
	backend.UpdateConfigFunc = func(_ context.Context, config openziti.ServiceConfig) (*openziti.ServiceConfig, error) {
		return &config, nil
	}
	var writes int
	backend.CreateAccessPolicyFunc = func(_ context.Context, p openziti.AccessPolicy) (*openziti.AccessPolicy, error) {
		writes++
		p.ID = "bind-1"
		return &p, nil
	}
	backend.UpdateAccessPolicyFunc = backend.CreateAccessPolicyFunc
	backend.CreateServiceEdgeRouterPolicyFunc = func(_ context.Context, p openziti.ServiceEdgeRouterPolicy) (*openziti.ServiceEdgeRouterPolicy, error) {
		writes++
		p.ID = "router-policy-1"
		return &p, nil
	}
	backend.UpdateServiceEdgeRouterPolicyFunc = backend.CreateServiceEdgeRouterPolicyFunc
	r := &ZitiServiceReconciler{Client: k8s, ServiceManager: zitiservice.NewService(backend)}
	_, _ = r.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(resource)})
	if writes != 0 {
		t.Fatalf("wrote %d router policies after failed resolution", writes)
	}
	var stored zitiv1alpha1.ZitiService
	if err := k8s.Get(ctx, client.ObjectKeyFromObject(resource), &stored); err != nil {
		t.Fatal(err)
	}
	ready := meta.FindStatusCondition(stored.Status.Conditions, zitiv1alpha1.ConditionTypeReady)
	if ready == nil || ready.Status != metav1.ConditionFalse || ready.Reason != "RouterResolutionFailed" {
		t.Fatalf("missing resolution failure status: %+v", stored.Status)
	}
}
