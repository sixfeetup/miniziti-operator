package rolesync

import (
	"strings"

	"example.com/miniziti-operator/internal/entra"
	openziti "example.com/miniziti-operator/internal/openziti/client"
)

func IdentityOwned(identity openziti.Identity, owners []IdentityOwner) bool {
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

func memberGrants(input Input, result *PlanResult) map[string][]string {
	grants := map[string][]string{}
	missing := map[string]bool{}
	for _, group := range input.Scope.GroupIDs() {
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
			grants[key] = sortedSet(grants[key], input.Scope.grantsByGroup[group])
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
