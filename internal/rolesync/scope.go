package rolesync

import (
	"slices"

	"example.com/miniziti-operator/internal/entra"
)

const defaultAccessRoleID = "00000000-0000-0000-0000-000000000000"

// RoleScope is the resolved role snapshot shared by collection and planning.
// Its private fields prevent callers from changing eligibility after collection.
type RoleScope struct {
	currentValues []string
	grantsByGroup map[string][]string
}

func (s RoleScope) GroupIDs() []string {
	groups := make([]string, 0, len(s.grantsByGroup))
	for id := range s.grantsByGroup {
		groups = append(groups, id)
	}
	slices.Sort(groups)
	return groups
}

// ResolveRoleScope manages disabled User roles, but grants only enabled group roles.
func ResolveRoleScope(principal entra.ServicePrincipal, assignments []entra.AppRoleAssignment) (RoleScope, error) {
	current := []string{}
	enabled := map[string]string{}
	for _, role := range principal.AppRoles {
		if !slices.Contains(role.AllowedMemberTypes, "User") {
			continue
		}
		current = append(current, role.Value)
		if role.IsEnabled {
			enabled[role.ID] = role.Value
		}
	}
	if len(current) == 0 {
		return RoleScope{}, ErrNoUserRoles
	}
	groups := map[string][]string{}
	for _, assignment := range assignments {
		value, ok := enabled[assignment.AppRoleID]
		if !ok || assignment.PrincipalType != "Group" || assignment.AppRoleID == defaultAccessRoleID {
			continue
		}
		groups[assignment.PrincipalID] = sortedSet(groups[assignment.PrincipalID], []string{value})
	}
	return RoleScope{currentValues: sortedSet(current), grantsByGroup: groups}, nil
}
