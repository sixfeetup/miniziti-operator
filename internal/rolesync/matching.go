package rolesync

import (
	"slices"
	"strings"

	"example.com/miniziti-operator/internal/entra"
	openziti "example.com/miniziti-operator/internal/openziti/client"
)

const defaultAccessRoleID = "00000000-0000-0000-0000-000000000000"

func identityOwned(identity openziti.Identity, owners []IdentityOwner) bool {
	for _, owner := range owners {
		if owner.IdentityID != "" && identity.ID == owner.IdentityID {
			return true
		}
		if owner.IdentityName != "" && identity.Name == owner.IdentityName {
			return true
		}
	}
	return false
}

func selectedProperty(user entra.User, property string) string {
	switch property {
	case "id":
		return user.ID
	case "userPrincipalName":
		return user.UserPrincipalName
	default:
		return user.Mail
	}
}

// Role candidates include disabled roles; grants include enabled group roles only.
func roleScope(input Input) ([]string, map[string][]string, error) {
	current := []string{}
	enabled := map[string]string{}
	for _, role := range input.ServicePrincipal.AppRoles {
		if !slices.Contains(role.AllowedMemberTypes, "User") {
			continue
		}
		current = append(current, role.Value)
		if role.IsEnabled {
			enabled[role.ID] = role.Value
		}
	}
	if len(current) == 0 {
		return nil, nil, ErrNoUserRoles
	}
	groups := map[string][]string{}
	for _, assignment := range input.Assignments {
		value, ok := enabled[assignment.AppRoleID]
		if !ok || assignment.PrincipalType != "Group" || assignment.AppRoleID == defaultAccessRoleID {
			continue
		}
		groups[assignment.PrincipalID] = sortedSet(groups[assignment.PrincipalID], []string{value})
	}
	return sortedSet(current), groups, nil
}

func memberGrants(input Input, groups map[string][]string, result *PlanResult) map[string][]string {
	grants := map[string][]string{}
	missing := map[string]bool{}
	groupIDs := make([]string, 0, len(groups))
	for group := range groups {
		groupIDs = append(groupIDs, group)
	}
	slices.Sort(groupIDs)
	for _, group := range groupIDs {
		for _, user := range input.MembersByGroup[group] {
			property := selectedProperty(user, input.UserProperty)
			if property == "" {
				if !missing[user.ID] {
					result.report(UnmatchedMember{ID: user.ID, DisplayName: user.DisplayName, Reason: "MissingProperty"})
					missing[user.ID] = true
				}
				continue
			}
			key := strings.ToLower(property)
			grants[key] = sortedSet(grants[key], groups[group])
		}
	}
	return grants
}

func identityIndex(identities []openziti.Identity) map[string][]openziti.Identity {
	index := map[string][]openziti.Identity{}
	for _, identity := range identities {
		if identity.ExternalID != "" {
			key := strings.ToLower(identity.ExternalID)
			index[key] = append(index[key], identity)
		}
	}
	return index
}
