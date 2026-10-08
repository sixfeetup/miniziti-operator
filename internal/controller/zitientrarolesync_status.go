package controller

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"

	v1alpha1 "example.com/miniziti-operator/api/v1alpha1"
	"example.com/miniziti-operator/internal/entra"
	"example.com/miniziti-operator/internal/rolesync"
)

func (r *ZitiEntraRoleSyncReconciler) persistSyncResult(ctx context.Context, resource *v1alpha1.ZitiEntraRoleSync, result syncRunResult) error {
	current, err := r.readRoleSync(ctx, resource)
	if err != nil {
		return err
	}
	previous := current.Status.DeepCopy()
	ready := result.Completion != nil && result.Completion.Ready
	message := roleSyncMessage(result)
	repeatedFailure := !ready && hasMatchingFailureStatus(current.Status.CommonStatus, resource.Generation, result.Reason, message)
	current.Status.ID = ""
	current.Status.ObservedGeneration = resource.Generation
	current.Status.LastError = ""
	if !ready {
		current.Status.LastError = message
	}
	if result.Completion != nil {
		current.Status.ManagedAttributes = append([]string{}, result.Completion.ManagedAttributes...)
		if result.Completion.AdvanceLastSyncTime {
			now := metav1.Now()
			current.Status.LastSyncTime = &now
		}
	}
	if result.WritePhaseStarted && result.Plan != nil {
		updateRoleSyncCounters(&current.Status, result)
	}
	setRoleSyncConditions(&current.Status, resource.Generation, result.Reason, message, ready)
	if !apiequality.Semantic.DeepEqual(previous, &current.Status) {
		if err := r.Status().Update(ctx, current); err != nil {
			return err
		}
	}
	resource.Status = *current.Status.DeepCopy()
	resource.ResourceVersion = current.ResourceVersion
	if !ready && !repeatedFailure {
		EmitEvent(r.Recorder, resource, corev1.EventTypeWarning, result.Reason, message)
	}
	return nil
}

func updateRoleSyncCounters(status *v1alpha1.ZitiEntraRoleSyncStatus, result syncRunResult) {
	status.MatchedIdentities = result.Plan.MatchedIdentities
	status.IdentitiesUpdated = result.IdentitiesUpdated
	status.MembersWithoutIdentity = result.Plan.MembersWithoutIdentity
	status.UnmatchedMembers = nil
	for _, member := range result.Plan.UnmatchedMembers {
		if len(status.UnmatchedMembers) == 50 {
			break
		}
		status.UnmatchedMembers = append(status.UnmatchedMembers, v1alpha1.EntraUnmatchedMember{ID: member.ID, DisplayName: member.DisplayName, ExternalID: member.ExternalID, Reason: member.Reason})
	}
}

func setRoleSyncConditions(status *v1alpha1.ZitiEntraRoleSyncStatus, generation int64, reason, message string, ready bool) {
	readyStatus, degradedStatus := metav1.ConditionFalse, metav1.ConditionTrue
	reconcilingReason, degradedReason := reason, reason
	if ready {
		readyStatus, degradedStatus = metav1.ConditionTrue, metav1.ConditionFalse
		reconcilingReason, degradedReason = "Reconciled", "AsExpected"
	}
	for _, condition := range []metav1.Condition{
		{Type: v1alpha1.ConditionTypeReady, Status: readyStatus, Reason: reason, Message: message},
		{Type: v1alpha1.ConditionTypeReconciling, Status: metav1.ConditionFalse, Reason: reconcilingReason, Message: message},
		{Type: v1alpha1.ConditionTypeDegraded, Status: degradedStatus, Reason: degradedReason, Message: message},
	} {
		condition.ObservedGeneration = generation
		condition.LastTransitionTime = metav1.Now()
		SetStatusCondition(&status.CommonStatus, condition)
	}
}

func roleSyncMessage(result syncRunResult) string {
	if result.Err != nil {
		return normalizeReconcileErrorMessage(result.Err.Error())
	}
	switch result.Reason {
	case syncReasonSynced:
		return "Entra role attributes are synchronized"
	case syncReasonCleanupPending:
		return "Retired attributes remain on skipped identities"
	case syncReasonWouldRemoveAll:
		return "Refusing to remove every managed attribute from eligible identities"
	case syncReasonConflict:
		if result.Plan != nil {
			return "Attributes owned by another role sync: " + strings.Join(result.Plan.Ownership.Conflicts, ", ")
		}
		return "Another role sync owns a managed attribute"
	default:
		return result.Reason
	}
}

func retryResult(interval time.Duration, reason string, err error, retryAfter time.Duration) (ctrl.Result, error) {
	switch reason {
	case syncReasonSynced, syncReasonCleanupPending, syncReasonInvalidSpec, syncReasonConflict, syncReasonWouldRemoveAll:
		return ctrl.Result{RequeueAfter: interval}, nil
	case syncReasonGraphError:
		var token *entra.TokenError
		var graph *entra.GraphError
		if errors.As(err, &token) || errors.Is(err, entra.ErrServicePrincipalNotFound) || errors.Is(err, entra.ErrLimitedUserData) || errors.Is(err, rolesync.ErrNoUserRoles) {
			return ctrl.Result{RequeueAfter: interval}, nil
		}
		if errors.As(err, &graph) {
			if graph.StatusCode == 401 || graph.StatusCode == 403 {
				return ctrl.Result{RequeueAfter: interval}, nil
			}
			if (graph.StatusCode == 429 || graph.StatusCode == 503) && retryAfter > 0 {
				return ctrl.Result{RequeueAfter: retryAfter}, nil
			}
		}
	}
	if err == nil {
		err = fmt.Errorf("role sync failed: %s", reason)
	}
	return ctrl.Result{}, err
}
