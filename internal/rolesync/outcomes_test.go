package rolesync

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"testing"
)

func TestCompleteAggregatesWriteOutcomes(t *testing.T) {
	failure := errors.New("first write failed")
	input := completeInput(retirementPlan(t, ""))
	input.Outcomes = []IdentityOutcome{
		{IdentityID: "success", Kind: Patched},
		{IdentityID: "noop", Kind: Observed},
		{IdentityID: "failure", Kind: Failed, Err: failure},
		{IdentityID: "success-2", Kind: Patched},
		{IdentityID: "conflict", Kind: Conflicted, ConflictValues: []string{"beta"}},
	}
	got := Complete(input)
	if got.IdentitiesUpdated != 2 || got.Reason != "ZitiError" || !errors.Is(got.Err, failure) || !reflect.DeepEqual(got.ConflictValues, []string{"beta"}) || !reflect.DeepEqual(got.ManagedAttributes, []string{"alpha", "beta"}) || got.AdvanceLastSyncTime {
		t.Fatalf("completion=%+v", got)
	}
	input.Outcomes = []IdentityOutcome{{Kind: Patched}, {Kind: Conflicted, ConflictValues: []string{"beta"}}}
	got = Complete(input)
	if got.IdentitiesUpdated != 1 || got.Reason != "Conflict" || got.Err != nil || !reflect.DeepEqual(got.ConflictValues, []string{"beta"}) {
		t.Fatalf("completion=%+v", got)
	}
}

func TestWriteOutcomeStopsOnlyForConflictOrCancellation(t *testing.T) {
	for _, tc := range []struct {
		outcome IdentityOutcome
		stop    bool
	}{
		{IdentityOutcome{Kind: Conflicted}, true},
		{IdentityOutcome{Kind: Failed, Err: fmt.Errorf("read: %w", context.Canceled)}, true},
		{IdentityOutcome{Kind: Failed, Err: context.DeadlineExceeded}, true},
		{IdentityOutcome{Kind: Failed, Err: errors.New("patch rejected")}, false},
		{IdentityOutcome{Kind: Skipped}, false},
		{IdentityOutcome{Kind: Patched}, false},
	} {
		if got := tc.outcome.StopsWrites(); got != tc.stop {
			t.Fatalf("outcome=%+v stop=%v want=%v", tc.outcome, got, tc.stop)
		}
	}
}
