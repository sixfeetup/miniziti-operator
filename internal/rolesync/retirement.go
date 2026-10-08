package rolesync

import "slices"

type OutcomeKind string

const (
	Observed OutcomeKind = "Observed"
	Patched  OutcomeKind = "Patched"
	Deleted  OutcomeKind = "Deleted"
	Skipped  OutcomeKind = "Skipped"
	Failed   OutcomeKind = "Failed"
)

type IdentityOutcome struct {
	IdentityID     string
	Kind           OutcomeKind
	RoleAttributes []string
}
type CompletionInput struct {
	Plan           PlanResult
	PreviousValues []string
	Outcomes       []IdentityOutcome
	StopReason     string
	ClaimRecorded  bool
}
type Completion struct {
	ManagedAttributes   []string
	Ready               bool
	Reason              string
	AdvanceLastSyncTime bool
}

// Complete forgets retired claims only when every observed holder is cleared.
func Complete(input CompletionInput) Completion {
	if !input.ClaimRecorded {
		return Completion{ManagedAttributes: sortedSet(input.PreviousValues), Reason: input.StopReason}
	}
	reason := input.StopReason
	for _, outcome := range input.Outcomes {
		if outcome.Kind == Failed {
			reason = "ZitiError"
		}
	}
	if reason != "" {
		return Completion{ManagedAttributes: sortedSet(input.Plan.ManagedValues), Reason: reason}
	}
	if retirementPending(input) {
		return Completion{ManagedAttributes: sortedSet(input.Plan.ManagedValues), Reason: "CleanupPending"}
	}
	return Completion{ManagedAttributes: sortedSet(input.Plan.CurrentValues), Ready: true, Reason: "Synced", AdvanceLastSyncTime: true}
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
