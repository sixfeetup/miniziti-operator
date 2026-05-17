package controller

import (
	"context"
	"strings"
	"testing"

	zitiv1alpha1 "example.com/miniziti-operator/api/v1alpha1"
	openziti "example.com/miniziti-operator/internal/openziti/client"
	zitiservice "example.com/miniziti-operator/internal/openziti/service"
)

func TestDesiredRouterPoliciesRejectsNonRouterIdentity(t *testing.T) {
	reconciler := &ZitiServiceReconciler{
		ServiceManager: zitiservice.NewService(&openziti.FakeClient{
			FindIdentityByNameFunc: func(context.Context, string) (*openziti.Identity, error) {
				return &openziti.Identity{ID: "user-1", Name: "alice", Type: "User"}, nil
			},
			FindEdgeRouterByNameFunc: func(context.Context, string) (*openziti.EdgeRouter, error) {
				return &openziti.EdgeRouter{ID: "edge-router-1", Name: "alice"}, nil
			},
		}),
	}
	resource := &zitiv1alpha1.ZitiService{}
	resource.Spec.Name = "argocd"
	resource.Spec.Router = &zitiv1alpha1.ServiceRouterSpec{Name: "alice"}

	_, err := reconciler.desiredRouterPolicies(context.Background(), resource, "service-1")
	if err == nil {
		t.Fatal("expected non-router identity to be rejected")
	}
	if !strings.Contains(err.Error(), "not a Router identity") {
		t.Fatalf("unexpected error: %v", err)
	}
}
