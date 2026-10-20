package controller

import (
	"context"
	"fmt"
	"strings"

	zitiv1alpha1 "example.com/miniziti-operator/api/v1alpha1"
	zitiservice "example.com/miniziti-operator/internal/openziti/service"
)

func (r *ZitiServiceReconciler) desiredRouterPolicies(
	ctx context.Context,
	resource *zitiv1alpha1.ZitiService,
	serviceID string,
) (zitiservice.RouterPolicySet, error) {
	if resource.Spec.Router == nil || strings.TrimSpace(resource.Spec.Router.Name) == "" {
		return zitiservice.RouterPolicySet{}, fmt.Errorf("spec.router.name must not be empty")
	}
	routerName := strings.TrimSpace(resource.Spec.Router.Name)
	routerIdentity, err := r.ServiceManager.FindIdentityByName(ctx, routerName)
	if err != nil {
		return zitiservice.RouterPolicySet{}, fmt.Errorf("resolve router identity %q: %w", routerName, err)
	}
	if routerIdentity == nil || strings.TrimSpace(routerIdentity.ID) == "" {
		return zitiservice.RouterPolicySet{}, fmt.Errorf("router identity %q not found", routerName)
	}
	if !strings.EqualFold(strings.TrimSpace(routerIdentity.Type), "Router") {
		return zitiservice.RouterPolicySet{}, fmt.Errorf("identity %q is not a Router identity", routerName)
	}

	hostingRouterID, err := r.resolveEdgeRouterID(ctx, routerName)
	if err != nil {
		return zitiservice.RouterPolicySet{}, err
	}

	// Resolve the entire desired set before returning either policy. Failed
	// resolution must not replace the last good policies with a partial set.
	seen := map[string]bool{routerName: true}
	var additionalIDs []string
	for _, rawName := range resource.Spec.Router.AdditionalAllowedEdgeRouterNames {
		name := strings.TrimSpace(rawName)
		if name == "" {
			return zitiservice.RouterPolicySet{}, fmt.Errorf("spec.router.additionalAllowedEdgeRouterNames must not contain blank names")
		}
		if seen[name] {
			continue
		}
		id, err := r.resolveEdgeRouterID(ctx, name)
		if err != nil {
			return zitiservice.RouterPolicySet{}, err
		}
		seen[name] = true
		additionalIDs = append(additionalIDs, id)
	}

	policies, ok := zitiservice.RouterPolicies(resource, serviceID, routerIdentity.ID, hostingRouterID, additionalIDs...)
	if !ok {
		return zitiservice.RouterPolicySet{}, fmt.Errorf("spec.router.name must not be empty")
	}
	return policies, nil
}

func (r *ZitiServiceReconciler) resolveEdgeRouterID(ctx context.Context, name string) (string, error) {
	edgeRouter, err := r.ServiceManager.FindEdgeRouterByName(ctx, name)
	if err != nil {
		return "", fmt.Errorf("resolve edge router %q: %w", name, err)
	}
	if edgeRouter == nil || strings.TrimSpace(edgeRouter.ID) == "" {
		return "", fmt.Errorf("edge router %q not found or has an empty ID", name)
	}
	return strings.TrimSpace(edgeRouter.ID), nil
}
