package rolesync

import (
	"reflect"
	"testing"

	"example.com/miniziti-operator/internal/entra"
)

func TestResolvedScopeSelectsOnlyEnabledGroupGrants(t *testing.T) {
	principal := entra.ServicePrincipal{AppRoles: []entra.AppRole{
		entraRole("beta", true), entraRole("alpha", false),
		{ID: "app", Value: "app", IsEnabled: true, AllowedMemberTypes: []string{"Application"}},
	}}
	assignments := []entra.AppRoleAssignment{
		{PrincipalID: "enabled", PrincipalType: "Group", AppRoleID: "beta"},
		{PrincipalID: "enabled", PrincipalType: "Group", AppRoleID: "beta"},
		{PrincipalID: "disabled", PrincipalType: "Group", AppRoleID: "alpha"},
		{PrincipalID: "application", PrincipalType: "Group", AppRoleID: "app"},
		{PrincipalID: "default", PrincipalType: "Group", AppRoleID: "00000000-0000-0000-0000-000000000000"},
		{PrincipalID: "direct", PrincipalType: "User", AppRoleID: "beta"},
	}
	scope, err := ResolveRoleScope(principal, assignments)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(scope.GroupIDs(), []string{"enabled"}) {
		t.Fatalf("groups=%v", scope.GroupIDs())
	}
	fixture := baseInput()
	input := fixture.Input
	input.Scope = scope
	input.MembersByGroup["enabled"] = input.MembersByGroup["g"]
	// The plan consumes the resolved snapshot, not mutable source records.
	principal.AppRoles[0].IsEnabled = false
	assignments[0].PrincipalID = "changed"
	plan, err := BuildPlan(input)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(plan.CurrentValues, []string{"alpha", "beta"}) || len(plan.Patches) != 1 || !reflect.DeepEqual(plan.Patches[0].GrantedAttributes, []string{"beta"}) {
		t.Fatalf("plan=%+v", plan)
	}
}
