package rolesync

import (
	"errors"
	"slices"
	"strings"

	"example.com/miniziti-operator/internal/entra"
	openziti "example.com/miniziti-operator/internal/openziti/client"
)

var ErrNoUserRoles = errors.New("entra application has no roles allowing User")

type Input struct {
	ServicePrincipal entra.ServicePrincipal
	Assignments      []entra.AppRoleAssignment
	MembersByGroup   map[string][]entra.User
	Identities       []openziti.Identity
	IdentityOwners   []IdentityOwner
	Self             SyncClaim
	OtherClaims      []SyncClaim
	UserProperty     string
}
type IdentityPatch struct {
	IdentityID, ExternalID            string
	GrantedAttributes, RoleAttributes []string
}
type RetirementHolder struct {
	IdentityID string
	Values     []string
}
type UnmatchedMember struct{ ID, DisplayName, ExternalID, Reason string }
type PlanResult struct {
	CurrentValues, ManagedValues              []string
	Ownership                                 OwnershipResult
	Patches                                   []IdentityPatch
	RetirementHolders                         []RetirementHolder
	UnmatchedMembers                          []UnmatchedMember
	MatchedIdentities, MembersWithoutIdentity int
	WouldRemoveAll                            bool
}

// BuildPlan uses only complete snapshots. Conflicts yield no authorized patches.
func BuildPlan(input Input) (PlanResult, error) {
	result := PlanResult{}
	current, groups, err := roleScope(input)
	if err != nil {
		return result, err
	}
	result.CurrentValues = current
	result.ManagedValues = sortedSet(current, input.Self.ManagedAttributes)
	result.Ownership = ResolveOwnership(OwnershipInput{Self: input.Self, Candidates: result.ManagedValues, Others: input.OtherClaims})
	retired := []string{}
	for _, value := range input.Self.ManagedAttributes {
		if !slices.Contains(current, value) {
			retired = append(retired, value)
		}
	}
	grants := memberGrants(input, groups, &result)
	index := identityIndex(input.Identities)
	for key := range grants {
		if len(index[key]) == 0 {
			result.MembersWithoutIdentity++
		}
	}
	hadManaged, willHaveManaged := false, false
	for _, identity := range input.Identities {
		result.observeRetirement(identity, retired)
		if !eligibleIdentity(identity, input.IdentityOwners, index, &result) {
			continue
		}
		granted := grants[strings.ToLower(identity.ExternalID)]
		if len(granted) > 0 {
			result.MatchedIdentities++
		}
		next := MergeAttributes(identity.RoleAttributes, result.ManagedValues, granted)
		hadManaged = hadManaged || overlaps(identity.RoleAttributes, result.ManagedValues)
		willHaveManaged = willHaveManaged || overlaps(next, result.ManagedValues)
		if !slices.Equal(sortedSet(identity.RoleAttributes), sortedSet(next)) {
			result.Patches = append(result.Patches, IdentityPatch{IdentityID: identity.ID, ExternalID: identity.ExternalID, GrantedAttributes: granted, RoleAttributes: next})
		}
	}
	result.WouldRemoveAll = hadManaged && !willHaveManaged
	if len(result.Ownership.Conflicts) > 0 {
		result.Patches = nil
	}
	return result, nil
}

func eligibleIdentity(identity openziti.Identity, owners []IdentityOwner, index map[string][]openziti.Identity, result *PlanResult) bool {
	if identity.ExternalID == "" {
		return false
	}
	reason := ""
	if identityOwned(identity, owners) {
		reason = "ManagedByZitiIdentity"
	} else if len(index[strings.ToLower(identity.ExternalID)]) > 1 {
		reason = "DuplicateIdentity"
	}
	if reason != "" {
		result.report(UnmatchedMember{ID: identity.ID, DisplayName: identity.Name, ExternalID: identity.ExternalID, Reason: reason})
		return false
	}
	return true
}

func (p *PlanResult) observeRetirement(identity openziti.Identity, retired []string) {
	values := []string{}
	for _, value := range retired {
		if slices.Contains(identity.RoleAttributes, value) {
			values = append(values, value)
		}
	}
	if len(values) > 0 {
		p.RetirementHolders = append(p.RetirementHolders, RetirementHolder{IdentityID: identity.ID, Values: sortedSet(values)})
	}
}
func (p *PlanResult) report(member UnmatchedMember) {
	if len(p.UnmatchedMembers) < 50 {
		p.UnmatchedMembers = append(p.UnmatchedMembers, member)
	}
}

func overlaps(values, managed []string) bool {
	for _, value := range values {
		if slices.Contains(managed, value) {
			return true
		}
	}
	return false
}

// MergeAttributes preserves unmanaged order and appends sorted, unique grants.
func MergeAttributes(current, managed, granted []string) []string {
	result := []string{}
	for _, value := range current {
		if !slices.Contains(managed, value) {
			result = append(result, value)
		}
	}
	return append(result, sortedSet(granted)...)
}
