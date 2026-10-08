package rolesync

import (
	"context"
	"errors"
	"slices"
)

const reasonSynced = "Synced"

type OutcomeKind string

const (
	Observed   OutcomeKind = "Observed"
	Patched    OutcomeKind = "Patched"
	Deleted    OutcomeKind = "Deleted"
	Skipped    OutcomeKind = "Skipped"
	Failed     OutcomeKind = "Failed"
	Conflicted OutcomeKind = "Conflicted"
)

type IdentityOutcome struct {
	IdentityID     string
	Kind           OutcomeKind
	RoleAttributes []string
	Err            error
	ConflictValues []string
}

// StopsWrites distinguishes a run-wide stop from an identity-local failure.
func (o IdentityOutcome) StopsWrites() bool {
	return o.Kind == Conflicted || errors.Is(o.Err, context.Canceled) || errors.Is(o.Err, context.DeadlineExceeded)
}

type CompletionInput struct {
	Plan           PlanResult
	PreviousValues []string
	Outcomes       []IdentityOutcome
	BlockReason    string
	ClaimRecorded  bool
}
type Completion struct {
	ManagedAttributes   []string
	Ready               bool
	Reason              string
	AdvanceLastSyncTime bool
	IdentitiesUpdated   int
	Err                 error
	ConflictValues      []string
}

// Complete forgets retired claims only when every observed holder is cleared.
func Complete(input CompletionInput) Completion {
	result := Completion{ManagedAttributes: sortedSet(input.PreviousValues), Reason: input.BlockReason, ConflictValues: sortedSet(input.Plan.Ownership.Conflicts)}
	if !input.ClaimRecorded {
		return result
	}
	result.ManagedAttributes = sortedSet(input.Plan.ManagedValues)
	failed := false
	for _, outcome := range input.Outcomes {
		switch outcome.Kind {
		case Patched:
			result.IdentitiesUpdated++
		case Failed:
			failed = true
			if result.Err == nil {
				result.Err = outcome.Err
			}
		case Conflicted:
			result.Reason = "Conflict"
			result.ConflictValues = sortedSet(result.ConflictValues, outcome.ConflictValues)
		}
	}
	if failed {
		result.Reason = "ZitiError"
	}
	if result.Reason != "" {
		return result
	}
	if retirementPending(input) {
		result.Reason = "CleanupPending"
		return result
	}
	result.ManagedAttributes = sortedSet(input.Plan.CurrentValues)
	result.Ready = true
	result.Reason = reasonSynced
	result.AdvanceLastSyncTime = true
	return result
}

func retirementPending(input CompletionInput) bool {
	retired := []string{}
	for _, value := range input.PreviousValues {
		if !slices.Contains(input.Plan.CurrentValues, value) {
			retired = append(retired, value)
		}
	}
	// Seed every snapshot holder, even those excluded from matching or reporting.
	holders := map[string][]string{}
	for _, holder := range input.Plan.RetirementHolders {
		holders[holder.IdentityID] = sortedSet(holders[holder.IdentityID], holder.Values)
	}
	for _, outcome := range input.Outcomes {
		switch outcome.Kind {
		case Deleted:
			delete(holders, outcome.IdentityID)
		case Observed, Patched, Skipped:
			values := []string{}
			for _, value := range retired {
				if slices.Contains(outcome.RoleAttributes, value) {
					values = append(values, value)
				}
			}
			if len(values) == 0 {
				delete(holders, outcome.IdentityID)
			} else {
				holders[outcome.IdentityID] = sortedSet(values)
			}
		}
	}
	return len(holders) > 0
}
