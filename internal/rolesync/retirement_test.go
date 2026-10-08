package rolesync

import (
	"fmt"
	"reflect"
	"slices"
	"testing"

	openziti "example.com/miniziti-operator/internal/openziti/client"
)

const cleanupPendingReason = "CleanupPending"

func retirementPlan(t *testing.T, kind string) PlanResult {
	t.Helper()
	input := baseInput()
	input.Self.ManagedAttributes = []string{"alpha", "beta"}
	input.Identities[0].RoleAttributes = []string{"beta"}
	switch kind {
	case "duplicate":
		input.Identities = append(input.Identities, openziti.Identity{ID: "d1", ExternalID: "duplicate", RoleAttributes: []string{"alpha"}}, openziti.Identity{ID: "d2", ExternalID: "DUPLICATE", RoleAttributes: []string{"alpha"}})
	case "owner":
		input.Identities = append(input.Identities, openziti.Identity{ID: "owned", ExternalID: "owned", RoleAttributes: []string{"alpha"}})
		input.IdentityOwners = []IdentityOwner{{IdentityID: "owned"}}
	case "no-external-id":
		input.Identities = append(input.Identities, openziti.Identity{ID: "anonymous", RoleAttributes: []string{"alpha"}})
	}
	p, err := BuildPlan(input)
	if err != nil {
		t.Fatal(err)
	}
	if p.WouldRemoveAll {
		t.Fatal("invalid retirement fixture")
	}
	return p
}
func completeInput(p PlanResult) CompletionInput {
	return CompletionInput{Plan: p, PreviousValues: []string{"alpha", "beta"}, ClaimRecorded: true}
}
func TestCompleteRetainsRetirementForSkippedHolders(t *testing.T) {
	for _, kind := range []string{"duplicate", "owner", "no-external-id"} {
		t.Run(kind, func(t *testing.T) {
			p := retirementPlan(t, kind)
			input := completeInput(p)
			for _, holder := range p.RetirementHolders {
				input.Outcomes = append(input.Outcomes, IdentityOutcome{IdentityID: holder.IdentityID, Kind: Skipped, RoleAttributes: []string{"alpha"}})
			}
			got := Complete(input)
			if !reflect.DeepEqual(got.ManagedAttributes, []string{"alpha", "beta"}) || got.Ready || got.Reason != cleanupPendingReason || got.AdvanceLastSyncTime {
				t.Fatalf("%+v", got)
			}
		})
	}
}
func TestCompleteRetiresAfterConfirmedCleanup(t *testing.T) {
	for _, kind := range []OutcomeKind{Patched, Observed, Deleted} {
		input := completeInput(retirementPlan(t, "duplicate"))
		for _, holder := range input.Plan.RetirementHolders {
			input.Outcomes = append(input.Outcomes, IdentityOutcome{IdentityID: holder.IdentityID, Kind: kind, RoleAttributes: []string{"beta"}})
		}
		got := Complete(input)
		if !reflect.DeepEqual(got.ManagedAttributes, []string{"beta"}) || !got.Ready || got.Reason != "Synced" || !got.AdvanceLastSyncTime {
			t.Fatalf("%+v", got)
		}
	}
}
func TestCompleteDoesNotUseReportingCap(t *testing.T) {
	p := retirementPlan(t, "")
	for i := 0; i < 60; i++ {
		p.RetirementHolders = append(p.RetirementHolders, RetirementHolder{IdentityID: fmt.Sprint(i), Values: []string{"alpha"}})
	}
	input := completeInput(p)
	for _, h := range p.RetirementHolders[:59] {
		input.Outcomes = append(input.Outcomes, IdentityOutcome{IdentityID: h.IdentityID, Kind: Deleted})
	}
	got := Complete(input)
	if got.Reason != cleanupPendingReason || !slices.Contains(got.ManagedAttributes, "alpha") {
		t.Fatalf("%+v", got)
	}
}
func TestCompleteTracksNewFreshRetiredValues(t *testing.T) {
	input := completeInput(retirementPlan(t, ""))
	input.Outcomes = []IdentityOutcome{{IdentityID: "new", Kind: Skipped, RoleAttributes: []string{"alpha"}}}
	got := Complete(input)
	if got.Reason != cleanupPendingReason || !slices.Contains(got.ManagedAttributes, "alpha") {
		t.Fatalf("%+v", got)
	}
}
func TestCompleteRetainsHistoryOnFailureOrLateConflict(t *testing.T) {
	for _, tc := range []struct {
		stop     string
		outcomes []IdentityOutcome
		reason   string
	}{
		{"", []IdentityOutcome{{IdentityID: "i", Kind: Failed}}, "ZitiError"},
		{"ZitiError", nil, "ZitiError"},
		{"Conflict", nil, "Conflict"},
		{"Conflict", []IdentityOutcome{{IdentityID: "i", Kind: Failed}}, "ZitiError"},
	} {
		input := completeInput(retirementPlan(t, ""))
		input.StopReason = tc.stop
		input.Outcomes = tc.outcomes
		got := Complete(input)
		if got.Reason != tc.reason || !reflect.DeepEqual(got.ManagedAttributes, []string{"alpha", "beta"}) || got.AdvanceLastSyncTime {
			t.Fatalf("%+v", got)
		}
	}
}
func TestCompletePreservesClaimsBeforeRecord(t *testing.T) {
	for _, stop := range []string{"Conflict", "WouldRemoveAll"} {
		input := completeInput(retirementPlan(t, ""))
		input.PreviousValues = []string{"alpha"}
		input.ClaimRecorded = false
		input.StopReason = stop
		got := Complete(input)
		if !reflect.DeepEqual(got.ManagedAttributes, []string{"alpha"}) || got.AdvanceLastSyncTime || got.Reason != stop {
			t.Fatalf("%+v", got)
		}
	}
}
func TestCompleteDoesNotBlockForCurrentValuesOnly(t *testing.T) {
	input := completeInput(retirementPlan(t, ""))
	input.Outcomes = []IdentityOutcome{{IdentityID: "current", Kind: Skipped, RoleAttributes: []string{"beta"}}}
	got := Complete(input)
	if got.Reason != "Synced" || !got.Ready {
		t.Fatalf("%+v", got)
	}
}
