package rolesync

import (
	"errors"
	"fmt"
	"reflect"
	"testing"

	"example.com/miniziti-operator/internal/entra"
	openziti "example.com/miniziti-operator/internal/openziti/client"
)

func entraRole(value string, enabled bool) entra.AppRole {
	return entra.AppRole{ID: value, Value: value, AllowedMemberTypes: []string{"User"}, IsEnabled: enabled}
}

type planFixture struct {
	Input
	ServicePrincipal entra.ServicePrincipal
	Assignments      []entra.AppRoleAssignment
}

func (f planFixture) plan() (PlanResult, error) {
	scope, err := ResolveRoleScope(f.ServicePrincipal, f.Assignments)
	if err != nil {
		return PlanResult{}, err
	}
	f.Scope = scope
	return BuildPlan(f.Input)
}

func baseInput() planFixture {
	return planFixture{
		ServicePrincipal: entra.ServicePrincipal{ID: "sp", AppRoles: []entra.AppRole{entraRole("beta", true)}},
		Assignments:      []entra.AppRoleAssignment{{PrincipalID: "g", PrincipalType: "Group", AppRoleID: "beta"}},
		Input: Input{
			MembersByGroup: map[string][]entra.User{"g": {{ID: "user-1", DisplayName: "Alice", Mail: "alice@example.com", UserPrincipalName: "alice@tenant.example"}}},
			Identities:     []openziti.Identity{{ID: "identity-1", Name: "Alice", ExternalID: "Alice@example.com", RoleAttributes: []string{"custom"}}},
			Self:           claim("ns", "self", 1), UserProperty: "mail",
		},
	}
}

func TestBuildPlanRoleAndGrantScope(t *testing.T) {
	input := baseInput()
	input.ServicePrincipal.AppRoles = append(input.ServicePrincipal.AppRoles, entraRole("alpha", false), entra.AppRole{ID: "app", Value: "app", IsEnabled: true, AllowedMemberTypes: []string{"Application"}})
	input.Assignments = append(input.Assignments, entra.AppRoleAssignment{PrincipalID: "g", PrincipalType: "Group", AppRoleID: "alpha"})
	p, err := input.plan()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(p.CurrentValues, []string{"alpha", "beta"}) || !reflect.DeepEqual(p.Patches[0].GrantedAttributes, []string{"beta"}) {
		t.Fatalf("%+v", p)
	}
	for _, assignment := range []entra.AppRoleAssignment{
		{PrincipalID: "user-1", PrincipalType: "User", AppRoleID: "beta"},
		{PrincipalID: "g", PrincipalType: "Group", AppRoleID: "00000000-0000-0000-0000-000000000000"},
		{PrincipalID: "g", PrincipalType: "Group", AppRoleID: "app"},
	} {
		input.Assignments = []entra.AppRoleAssignment{assignment}
		input.Identities = nil
		p, err = input.plan()
		if err != nil {
			t.Fatal(err)
		}
		if len(p.Patches) != 0 || p.MembersWithoutIdentity != 0 {
			t.Fatalf("%+v", p)
		}
	}
	input = baseInput()
	input.ServicePrincipal.AppRoles[0].AllowedMemberTypes = []string{"Application"}
	_, err = input.plan()
	if !errors.Is(err, ErrNoUserRoles) {
		t.Fatalf("%v", err)
	}
	input = baseInput()
	input.ServicePrincipal.AppRoles = append(input.ServicePrincipal.AppRoles, entraRole("alpha", true))
	input.Assignments = append(input.Assignments, entra.AppRoleAssignment{PrincipalID: "other", PrincipalType: "Group", AppRoleID: "alpha"})
	input.MembersByGroup["other"] = input.MembersByGroup["g"]
	p, err = input.plan()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(p.Patches[0].GrantedAttributes, []string{"alpha", "beta"}) || p.MatchedIdentities != 1 {
		t.Fatalf("%+v", p)
	}
}

func TestMergeAttributes(t *testing.T) {
	for _, tc := range []struct{ current, managed, granted, want []string }{
		{[]string{"custom", "alpha"}, []string{"alpha", "beta"}, []string{"beta"}, []string{"custom", "beta"}},
		{[]string{"second", "alpha", "first"}, []string{"alpha", "beta", "gamma"}, []string{"gamma", "beta"}, []string{"second", "first", "beta", "gamma"}},
	} {
		if got := MergeAttributes(tc.current, tc.managed, tc.granted); !reflect.DeepEqual(got, tc.want) {
			t.Fatalf("got=%v want=%v", got, tc.want)
		}
	}
}

func partialOffboarding() planFixture {
	input := baseInput()
	input.Identities[0].RoleAttributes = []string{"custom", "beta"}
	input.Identities = append(input.Identities, openziti.Identity{ID: "keeping", ExternalID: "other@example.com", RoleAttributes: []string{"beta"}})
	input.MembersByGroup["g"][0].Mail = "other@example.com"
	return input
}
func TestBuildPlanOffboardingAndNoop(t *testing.T) {
	p, err := partialOffboarding().plan()
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Patches) != 1 || !reflect.DeepEqual(p.Patches[0].RoleAttributes, []string{"custom"}) {
		t.Fatalf("%+v", p)
	}
	input := baseInput()
	input.Identities[0].RoleAttributes = []string{"beta", "custom"}
	p, err = input.plan()
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Patches) != 0 {
		t.Fatal("set-equal attributes must not patch")
	}
	input = baseInput()
	input.Identities[0].ExternalID = ""
	p, err = input.plan()
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Patches) != 0 {
		t.Fatal("missing external ID must not patch")
	}
}

func TestBuildPlanWouldRemoveAll(t *testing.T) {
	input := baseInput()
	input.Identities[0].RoleAttributes = []string{"custom", "beta"}
	input.Assignments = nil
	p, err := input.plan()
	if err != nil {
		t.Fatal(err)
	}
	if !p.WouldRemoveAll {
		t.Fatal("must guard removal")
	}
	input.IdentityOwners = []IdentityOwner{{IdentityID: "identity-1"}}
	p, err = input.plan()
	if err != nil {
		t.Fatal(err)
	}
	if p.WouldRemoveAll {
		t.Fatal("excluded holders do not trigger guard")
	}
	p, err = partialOffboarding().plan()
	if err != nil {
		t.Fatal(err)
	}
	if p.WouldRemoveAll {
		t.Fatal("partial offboarding must proceed")
	}
	input = baseInput()
	input.Assignments = nil
	p, err = input.plan()
	if err != nil {
		t.Fatal(err)
	}
	if p.WouldRemoveAll {
		t.Fatal("no prior holder")
	}
}

func TestBuildPlanReportingDoesNotTruncateObservations(t *testing.T) {
	input := baseInput()
	input.Self.ManagedAttributes = []string{"alpha", "beta"}
	for i := 0; i < 60; i++ {
		id := fmt.Sprintf("skip-%d", i)
		input.Identities = append(input.Identities, openziti.Identity{ID: id, Name: id, ExternalID: id, RoleAttributes: []string{"alpha"}})
		input.IdentityOwners = append(input.IdentityOwners, IdentityOwner{IdentityID: id})
	}
	p, err := input.plan()
	if err != nil {
		t.Fatal(err)
	}
	if len(p.UnmatchedMembers) != 50 || len(p.RetirementHolders) != 60 {
		t.Fatalf("reports=%d holders=%d", len(p.UnmatchedMembers), len(p.RetirementHolders))
	}
}
