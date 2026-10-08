package rolesync

import (
	"slices"
	"time"
)

type ResourceKey struct{ Namespace, Name string }
type SyncClaim struct {
	Resource          ResourceKey
	CreationTimestamp time.Time
	ManagedAttributes []string
}
type IdentityOwner struct {
	Resource                 ResourceKey
	IdentityID, IdentityName string
}
type OwnershipInput struct {
	Self       SyncClaim
	Candidates []string
	Others     []SyncClaim
}
type OwnershipResult struct {
	Owners    map[string]ResourceKey
	Conflicts []string
}

// ResolveOwnership arbitrates recorded claims; age never displaces a recorded owner.
func ResolveOwnership(input OwnershipInput) OwnershipResult {
	result := OwnershipResult{Owners: map[string]ResourceKey{}, Conflicts: []string{}}
	claims := append([]SyncClaim{input.Self}, input.Others...)
	for _, value := range sortedSet(input.Candidates) {
		var winner *SyncClaim
		for i := range claims {
			claim := &claims[i]
			if !slices.Contains(claim.ManagedAttributes, value) {
				continue
			}
			if winner == nil || claimBefore(*claim, *winner) {
				winner = claim
			}
		}
		owner := input.Self.Resource
		if winner != nil {
			owner = winner.Resource
		}
		result.Owners[value] = owner
		if owner != input.Self.Resource {
			result.Conflicts = append(result.Conflicts, value)
		}
	}
	return result
}

func claimBefore(a, b SyncClaim) bool {
	if !a.CreationTimestamp.Equal(b.CreationTimestamp) {
		return a.CreationTimestamp.Before(b.CreationTimestamp)
	}
	if a.Resource.Namespace != b.Resource.Namespace {
		return a.Resource.Namespace < b.Resource.Namespace
	}
	return a.Resource.Name < b.Resource.Name
}

func sortedSet(values ...[]string) []string {
	set := map[string]bool{}
	for _, list := range values {
		for _, value := range list {
			set[value] = true
		}
	}
	result := make([]string, 0, len(set))
	for value := range set {
		result = append(result, value)
	}
	slices.Sort(result)
	return result
}
