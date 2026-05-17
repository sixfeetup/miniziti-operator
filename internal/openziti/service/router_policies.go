package service

import (
	"strings"

	zitiv1alpha1 "example.com/miniziti-operator/api/v1alpha1"
	openziti "example.com/miniziti-operator/internal/openziti/client"
)

const semanticAnyOf = "AnyOf"

// RouterPolicySet captures the router-side policies that make a hosted service usable.
type RouterPolicySet struct {
	BindPolicy              openziti.AccessPolicy
	ServiceEdgeRouterPolicy openziti.ServiceEdgeRouterPolicy
}

// RouterPolicies maps a ZitiService router declaration and resolved backend IDs into OpenZiti policies.
func RouterPolicies(resource *zitiv1alpha1.ZitiService, serviceID, routerIdentityID, routerEdgeRouterID string) (RouterPolicySet, bool) {
	if resource.Spec.Router == nil || strings.TrimSpace(resource.Spec.Router.Name) == "" {
		return RouterPolicySet{}, false
	}

	serviceRole := "@" + strings.TrimSpace(serviceID)
	routerIdentityRole := "@" + strings.TrimSpace(routerIdentityID)
	routerEdgeRouterRole := "@" + strings.TrimSpace(routerEdgeRouterID)

	return RouterPolicySet{
		BindPolicy: openziti.AccessPolicy{
			Name:             resource.Spec.Name + "-bind-policy",
			Type:             zitiv1alpha1.AccessPolicyTypeBind,
			IdentityRoles:    []string{routerIdentityRole},
			IdentityRolesRaw: []string{routerIdentityRole},
			ServiceRoles:     []string{serviceRole},
			ServiceRolesRaw:  []string{serviceRole},
			Semantic:         semanticAnyOf,
		},
		ServiceEdgeRouterPolicy: openziti.ServiceEdgeRouterPolicy{
			Name:            resource.Spec.Name + "-only",
			EdgeRouterRoles: []string{routerEdgeRouterRole},
			ServiceRoles:    []string{serviceRole},
			Semantic:        semanticAnyOf,
		},
	}, true
}
