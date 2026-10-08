package controller

import (
	"context"
	"fmt"
	"slices"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/log"

	v1alpha1 "example.com/miniziti-operator/api/v1alpha1"
	"example.com/miniziti-operator/internal/rolesync"
)

func (r *ZitiEntraRoleSyncReconciler) writeRolePlan(ctx context.Context, resource *v1alpha1.ZitiEntraRoleSync, plan rolesync.PlanResult) []rolesync.IdentityOutcome {
	outcomes := []rolesync.IdentityOutcome{}
	for _, patch := range plan.Patches {
		if err := ctx.Err(); err != nil {
			return append(outcomes, rolesync.IdentityOutcome{IdentityID: patch.IdentityID, Kind: rolesync.Failed, Err: err})
		}
		outcome := r.applyRolePatch(ctx, resource, plan, patch)
		outcomes = append(outcomes, outcome)
		if outcome.StopsWrites() {
			return outcomes
		}
	}
	if err := ctx.Err(); err != nil {
		outcomes = append(outcomes, rolesync.IdentityOutcome{Kind: rolesync.Failed, Err: err})
	}
	return outcomes
}

func (r *ZitiEntraRoleSyncReconciler) applyRolePatch(ctx context.Context, resource *v1alpha1.ZitiEntraRoleSync, plan rolesync.PlanResult, patch rolesync.IdentityPatch) rolesync.IdentityOutcome {
	outcome := rolesync.IdentityOutcome{IdentityID: patch.IdentityID, Kind: rolesync.Failed}
	fresh, err := r.ZitiClient.GetIdentity(ctx, patch.IdentityID)
	if err != nil {
		outcome.Err = err
		return outcome
	}
	if fresh != nil {
		outcome.RoleAttributes = append([]string(nil), fresh.RoleAttributes...)
	}
	self, others, owners, err := r.readOwnership(ctx, resource)
	if err != nil {
		outcome.Err = err
		return outcome
	}
	ownership := rolesync.ResolveOwnership(rolesync.OwnershipInput{Self: self, Candidates: plan.ManagedValues, Others: others})
	if len(ownership.Conflicts) > 0 {
		outcome.Kind = rolesync.Conflicted
		outcome.ConflictValues = ownership.Conflicts
		return outcome
	}
	if fresh == nil {
		outcome.Kind = rolesync.Deleted
		return outcome
	}
	if rolesync.IdentityOwned(*fresh, owners) || fresh.ExternalID == "" || !strings.EqualFold(fresh.ExternalID, patch.ExternalID) {
		outcome.Kind = rolesync.Skipped
		return outcome
	}
	next := rolesync.MergeAttributes(fresh.RoleAttributes, plan.ManagedValues, patch.GrantedAttributes)
	if sameRoleSet(fresh.RoleAttributes, next) {
		outcome.Kind = rolesync.Observed
		return outcome
	}
	if err := ctx.Err(); err != nil {
		outcome.Err = err
		return outcome
	}
	if err := r.ZitiClient.PatchIdentityRoleAttributes(ctx, patch.IdentityID, next); err != nil {
		outcome.Err = err
		return outcome
	}
	outcome.Kind = rolesync.Patched
	outcome.RoleAttributes = next
	added, removed := roleChanges(fresh.RoleAttributes, next)
	EmitEvent(r.Recorder, resource, corev1.EventTypeNormal, "RoleAttributesUpdated", fmt.Sprintf("Identity %s (externalId=%q): added=%v removed=%v", patch.IdentityID, fresh.ExternalID, added, removed))
	log.FromContext(ctx).Info("synchronized identity role attributes", "identityID", patch.IdentityID, "externalID", fresh.ExternalID, "added", added, "removed", removed)
	return outcome
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
