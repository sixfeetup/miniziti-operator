package rolesync

import (
	"strings"
	"testing"

	openziti "example.com/miniziti-operator/internal/openziti/client"
)

func TestBuildPlanExcludesZitiIdentityClaims(t *testing.T) {
	for _, owner := range []IdentityOwner{
		{Resource: ResourceKey{Namespace: "elsewhere", Name: "deleting"}, IdentityID: "identity-1"},
		{Resource: ResourceKey{Namespace: "elsewhere", Name: "adopting"}, IdentityName: "Alice"},
	} {
		input := baseInput()
		input.IdentityOwners = []IdentityOwner{owner}
		p, err := BuildPlan(input)
		if err != nil {
			t.Fatal(err)
		}
		if len(p.Patches) != 0 || len(p.UnmatchedMembers) != 1 || p.UnmatchedMembers[0].Reason != "ManagedByZitiIdentity" || p.MembersWithoutIdentity != 0 {
			t.Fatalf("%+v", p)
		}
	}
	input := baseInput()
	input.IdentityOwners = []IdentityOwner{{IdentityName: "alice"}}
	p, err := BuildPlan(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Patches) != 1 {
		t.Fatal("name exclusion must be exact")
	}
}

func TestBuildPlanIndexesDuplicatesBeforeExclusion(t *testing.T) {
	for _, excluded := range []bool{false, true} {
		input := baseInput()
		input.Identities = append(input.Identities, openziti.Identity{ID: "identity-2", Name: "Other", ExternalID: strings.ToUpper(input.Identities[0].ExternalID)})
		if excluded {
			input.IdentityOwners = []IdentityOwner{{IdentityID: "identity-1"}}
		}
		p, err := BuildPlan(input)
		if err != nil {
			t.Fatal(err)
		}
		if len(p.Patches) != 0 || len(p.UnmatchedMembers) != 2 || p.MembersWithoutIdentity != 0 {
			t.Fatalf("%+v", p)
		}
		if !excluded {
			for _, u := range p.UnmatchedMembers {
				if u.Reason != "DuplicateIdentity" {
					t.Fatal(u)
				}
			}
		}
	}
}

func TestBuildPlanUserProperties(t *testing.T) {
	for _, property := range []string{"mail", "userPrincipalName", "id"} {
		input := baseInput()
		input.UserProperty = property
		user := input.MembersByGroup["g"][0]
		switch property {
		case "id":
			input.Identities[0].ExternalID = strings.ToUpper(user.ID)
		case "userPrincipalName":
			input.Identities[0].ExternalID = strings.ToUpper(user.UserPrincipalName)
		}
		p, err := BuildPlan(input)
		if err != nil {
			t.Fatal(err)
		}
		if p.MatchedIdentities != 1 {
			t.Fatalf("%s: %+v", property, p)
		}
	}
	input := baseInput()
	input.MembersByGroup["g"][0].Mail = ""
	p, err := BuildPlan(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.UnmatchedMembers) != 1 || p.UnmatchedMembers[0].Reason != "MissingProperty" {
		t.Fatalf("%+v", p)
	}
	input = baseInput()
	input.Identities = nil
	p, err = BuildPlan(input)
	if err != nil {
		t.Fatal(err)
	}
	if p.MembersWithoutIdentity != 1 {
		t.Fatalf("%+v", p)
	}
}
