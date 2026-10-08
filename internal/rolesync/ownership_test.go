package rolesync

import (
	"reflect"
	"testing"
	"time"
)

func claim(namespace, name string, age int, values ...string) SyncClaim {
	return SyncClaim{Resource: ResourceKey{Namespace: namespace, Name: name}, CreationTimestamp: time.Unix(int64(age), 0), ManagedAttributes: values}
}

func TestResolveOwnershipSingleRecordedClaim(t *testing.T) {
	self, other := claim("a", "old", 1), claim("b", "young", 2, "alpha")
	got := ResolveOwnership(OwnershipInput{Self: self, Candidates: []string{"alpha"}, Others: []SyncClaim{other}})
	if got.Owners["alpha"] != other.Resource || !reflect.DeepEqual(got.Conflicts, []string{"alpha"}) {
		t.Fatalf("%+v", got)
	}
}

func TestResolveOwnershipOverlappingRecordedClaims(t *testing.T) {
	older, younger := claim("a", "old", 1, "alpha"), claim("b", "young", 2, "alpha")
	for _, self := range []SyncClaim{older, younger} {
		other := younger
		if self.Resource == younger.Resource {
			other = older
		}
		got := ResolveOwnership(OwnershipInput{Self: self, Candidates: []string{"alpha"}, Others: []SyncClaim{other}})
		if got.Owners["alpha"] != older.Resource {
			t.Fatalf("%+v", got)
		}
		if self.Resource == older.Resource && len(got.Conflicts) != 0 {
			t.Fatal(got.Conflicts)
		}
		if self.Resource == younger.Resource && !reflect.DeepEqual(got.Conflicts, []string{"alpha"}) {
			t.Fatal(got.Conflicts)
		}
	}
}

func TestResolveOwnershipTieBreaks(t *testing.T) {
	for _, pair := range [][2]SyncClaim{{claim("a", "z", 1, "alpha"), claim("b", "a", 1, "alpha")}, {claim("a", "a", 1, "alpha"), claim("a", "z", 1, "alpha")}} {
		self := claim("z", "self", 2)
		got := ResolveOwnership(OwnershipInput{Self: self, Candidates: []string{"alpha"}, Others: []SyncClaim{pair[0], pair[1]}})
		reversed := ResolveOwnership(OwnershipInput{Self: self, Candidates: []string{"alpha"}, Others: []SyncClaim{pair[1], pair[0]}})
		if got.Owners["alpha"] != pair[0].Resource || !reflect.DeepEqual(got.Owners, reversed.Owners) {
			t.Fatalf("%+v vs %+v", got, reversed)
		}
	}
}

func TestResolveOwnershipMixedClaims(t *testing.T) {
	input := baseInput()
	input.Self.ManagedAttributes = []string{"beta"}
	input.ServicePrincipal.AppRoles = append(input.ServicePrincipal.AppRoles, entraRole("alpha", true))
	input.OtherClaims = []SyncClaim{claim("b", "other", 2, "alpha")}
	p, err := BuildPlan(input)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(p.Ownership.Conflicts, []string{"alpha"}) || len(p.Patches) != 0 || p.Ownership.Owners["beta"] != input.Self.Resource {
		t.Fatalf("%+v", p)
	}
}
