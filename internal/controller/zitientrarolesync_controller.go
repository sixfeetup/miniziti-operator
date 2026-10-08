package controller

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1alpha1 "example.com/miniziti-operator/api/v1alpha1"
	"example.com/miniziti-operator/internal/entra"
	openziti "example.com/miniziti-operator/internal/openziti/client"
	"example.com/miniziti-operator/internal/rolesync"
)

const (
	syncReasonInvalidSpec    = "InvalidSpec"
	syncReasonGraphError     = "GraphError"
	syncReasonConflict       = "Conflict"
	syncReasonWouldRemoveAll = "WouldRemoveAll"
	syncReasonZitiError      = "ZitiError"
	syncReasonCleanupPending = "CleanupPending"
	syncReasonSynced         = "Synced"
)

type ZitiEntraRoleSyncReconciler struct {
	client.Client
	APIReader        client.Reader
	Scheme           *runtime.Scheme
	Recorder         record.EventRecorder
	ZitiClient       openziti.Client
	DirectoryFactory entra.Factory
}
type syncRunResult struct {
	Plan              *rolesync.PlanResult
	Completion        *rolesync.Completion
	IdentitiesUpdated int
	WritePhaseStarted bool
	Reason            string
	Err               error
	RetryAfter        time.Duration
}

func (r *ZitiEntraRoleSyncReconciler) runSync(ctx context.Context, resource *v1alpha1.ZitiEntraRoleSync) syncRunResult {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	if _, err := validateRoleSyncSpec(resource); err != nil {
		return syncRunResult{Reason: syncReasonInvalidSpec, Err: err}
	}
	directory, err := r.readDirectory(ctx, resource)
	if err != nil {
		return syncRunResult{Reason: syncReasonInvalidSpec, Err: err}
	}
	input, err := r.readGraph(ctx, directory, resource)
	if err != nil {
		return graphRunFailure(err)
	}
	input.Identities, err = r.ZitiClient.ListIdentities(ctx)
	if err != nil {
		return syncRunResult{Reason: syncReasonZitiError, Err: err}
	}
	self, others, owners, err := r.readOwnership(ctx, client.ObjectKeyFromObject(resource))
	if err != nil {
		return syncRunResult{Reason: syncReasonZitiError, Err: err}
	}
	input.Self, input.OtherClaims, input.IdentityOwners = self, others, owners
	input.UserProperty = resource.Spec.UserProperty
	plan, err := rolesync.BuildPlan(input)
	if err != nil {
		return graphRunFailure(err)
	}
	result := syncRunResult{Plan: &plan}
	completionInput := rolesync.CompletionInput{Plan: plan, PreviousValues: self.ManagedAttributes}
	if len(plan.Ownership.Conflicts) > 0 {
		completionInput.StopReason = syncReasonConflict
	} else if plan.WouldRemoveAll {
		completionInput.StopReason = syncReasonWouldRemoveAll
	}
	if completionInput.StopReason != "" {
		completion := rolesync.Complete(completionInput)
		result.Completion = &completion
		result.Reason = completion.Reason
		return result
	}
	if err := r.recordRoleClaims(ctx, resource, self, plan.ManagedValues); err != nil {
		result.Reason = syncReasonZitiError
		result.Err = err
		return result
	}
	completionInput.ClaimRecorded = true
	result.WritePhaseStarted = true
	r.writeRolePlan(ctx, resource, &result, &completionInput)
	completion := rolesync.Complete(completionInput)
	result.Completion = &completion
	result.Reason = completion.Reason
	return result
}
