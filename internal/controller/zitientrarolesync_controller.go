package controller

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	controllerconfig "sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

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

// +kubebuilder:rbac:groups=ziti.sixfeetup.com,resources=zitientrarolesyncs,verbs=get;list;watch
// +kubebuilder:rbac:groups=ziti.sixfeetup.com,resources=zitientrarolesyncs/status,verbs=get;update;patch

func (r *ZitiEntraRoleSyncReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var resource v1alpha1.ZitiEntraRoleSync
	if err := r.Get(ctx, req.NamespacedName, &resource); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	if !resource.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}
	interval, _ := validateRoleSyncSpec(&resource)
	result := r.runSync(ctx, &resource)
	// Use the original reconcile context, not the expired sync deadline.
	if err := r.persistSyncResult(ctx, &resource, result); err != nil {
		return ctrl.Result{}, err
	}
	return retryResult(interval, result.Reason, result.Err, result.RetryAfter)
}

func (r *ZitiEntraRoleSyncReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.ZitiEntraRoleSync{}).
		WithEventFilter(predicate.GenerationChangedPredicate{}).
		WithOptions(controllerconfig.Options{
			MaxConcurrentReconciles: 1,
			RateLimiter:             workqueue.NewTypedItemExponentialFailureRateLimiter[reconcile.Request](10*time.Second, 10*time.Minute),
		}).Complete(r)
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
