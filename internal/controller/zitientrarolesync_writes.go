package controller

import (
	"context"
	"fmt"
	"slices"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"

	v1alpha1 "example.com/miniziti-operator/api/v1alpha1"
	openziti "example.com/miniziti-operator/internal/openziti/client"
	"example.com/miniziti-operator/internal/rolesync"
)

func (r *ZitiEntraRoleSyncReconciler) writeRolePlan(ctx context.Context, resource *v1alpha1.ZitiEntraRoleSync, result *syncRunResult, completion *rolesync.CompletionInput) {
	for _, patch := range result.Plan.Patches {
		if err := ctx.Err(); err != nil {
			completion.StopReason = syncReasonZitiError
			result.Err = err
			break
		}
		outcome, conflict, err := r.applyRolePatch(ctx, resource, result.Plan, patch)
		completion.Outcomes = append(completion.Outcomes, outcome)
		if err != nil && result.Err == nil {
			result.Err = err
		}
		if outcome.Kind == rolesync.Patched {
			result.IdentitiesUpdated++
		}
		if conflict {
			completion.StopReason = syncReasonConflict
			break
		}
	}
	if err := ctx.Err(); err != nil {
		completion.StopReason = syncReasonZitiError
		if result.Err == nil {
			result.Err = err
		}
	}
}

func (r *ZitiEntraRoleSyncReconciler) applyRolePatch(ctx context.Context, resource *v1alpha1.ZitiEntraRoleSync, plan *rolesync.PlanResult, patch rolesync.IdentityPatch) (rolesync.IdentityOutcome, bool, error) {
	outcome := rolesync.IdentityOutcome{IdentityID: patch.IdentityID, Kind: rolesync.Failed}
	fresh, err := r.ZitiClient.GetIdentity(ctx, patch.IdentityID)
	if err != nil {
		return outcome, false, err
	}
	if fresh != nil {
		outcome.RoleAttributes = append([]string(nil), fresh.RoleAttributes...)
	}
	self, others, owners, err := r.readOwnership(ctx, resource)
	if err != nil {
		return outcome, false, err
	}
	ownership := rolesync.ResolveOwnership(rolesync.OwnershipInput{Self: self, Candidates: plan.ManagedValues, Others: others})
	if len(ownership.Conflicts) > 0 {
		plan.Ownership = ownership
		outcome.Kind = rolesync.Skipped
		return outcome, true, nil
	}
	if fresh == nil {
		outcome.Kind = rolesync.Deleted
		return outcome, false, nil
	}
	if claimedRoleIdentity(*fresh, owners) || fresh.ExternalID == "" || !strings.EqualFold(fresh.ExternalID, patch.ExternalID) {
		outcome.Kind = rolesync.Skipped
		return outcome, false, nil
	}
	next := rolesync.MergeAttributes(fresh.RoleAttributes, plan.ManagedValues, patch.GrantedAttributes)
	if sameRoleSet(fresh.RoleAttributes, next) {
		outcome.Kind = rolesync.Observed
		return outcome, false, nil
	}
	if err := ctx.Err(); err != nil {
		return outcome, false, err
	}
	if err := r.ZitiClient.PatchIdentityRoleAttributes(ctx, patch.IdentityID, next); err != nil {
		return outcome, false, err
	}
	outcome.Kind = rolesync.Patched
	outcome.RoleAttributes = next
	added, removed := roleChanges(fresh.RoleAttributes, next)
	EmitEvent(r.Recorder, resource, corev1.EventTypeNormal, "RoleAttributesUpdated", fmt.Sprintf("Identity %s (externalId=%q): added=%v removed=%v", patch.IdentityID, fresh.ExternalID, added, removed))
	log.FromContext(ctx).Info("synchronized identity role attributes", "identityID", patch.IdentityID, "externalID", fresh.ExternalID, "added", added, "removed", removed)
	return outcome, false, nil
}

func claimedRoleIdentity(identity openziti.Identity, owners []rolesync.IdentityOwner) bool {
	for _, owner := range owners {
		if owner.IdentityID != "" && identity.ID == owner.IdentityID {
			return true
		}
		if owner.IdentityName != "" && identity.Name == owner.IdentityName {
			return true
		}
	}
	return false
}

func roleChanges(before, after []string) ([]string, []string) {
	added, removed := []string{}, []string{}
	for _, value := range after {
		if !slices.Contains(before, value) {
			added = append(added, value)
		}
	}
	for _, value := range before {
		if !slices.Contains(after, value) {
			removed = append(removed, value)
		}
	}
	slices.Sort(added)
	slices.Sort(removed)
	return slices.Compact(added), slices.Compact(removed)
}
