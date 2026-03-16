/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"fmt"
	"slices"

	corev1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	zitiv1alpha1 "example.com/miniziti-operator/api/v1alpha1"
	openziti "example.com/miniziti-operator/internal/openziti/client"
	policyservice "example.com/miniziti-operator/internal/openziti/policy"
)

const zitiAccessPolicyFinalizer = "ziti.sixfeetup.com/access-policy-finalizer"

// ZitiAccessPolicyReconciler reconciles a ZitiAccessPolicy object.
type ZitiAccessPolicyReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	Recorder      record.EventRecorder
	PolicyService *policyservice.Service
}

// +kubebuilder:rbac:groups=ziti.sixfeetup.com,resources=zitiaccesspolicies,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ziti.sixfeetup.com,resources=zitiaccesspolicies/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=ziti.sixfeetup.com,resources=zitiaccesspolicies/finalizers,verbs=update
// +kubebuilder:rbac:groups=ziti.sixfeetup.com,resources=zitiidentities,verbs=get;list;watch
// +kubebuilder:rbac:groups=ziti.sixfeetup.com,resources=zitiservices,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

func (r *ZitiAccessPolicyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var policy zitiv1alpha1.ZitiAccessPolicy
	if err := r.Get(ctx, req.NamespacedName, &policy); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if policy.DeletionTimestamp.IsZero() {
		if EnsureFinalizer(&policy, zitiAccessPolicyFinalizer) {
			if err := r.Update(ctx, &policy); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{Requeue: true}, nil
		}
	} else {
		return r.reconcileDelete(ctx, &policy)
	}

	if err := validateAccessPolicySpec(&policy); err != nil {
		return r.markFailed(ctx, &policy, err, "SelectorValidationFailed", false)
	}

	identitySelector, serviceSelector, identityCount, serviceCount, err := r.resolveSelectors(ctx, &policy)
	if err != nil {
		return r.markFailed(ctx, &policy, err, "SelectorResolutionFailed", true)
	}
	policy.Status.ResolvedIdentityCount = int32(identityCount)
	policy.Status.ResolvedServiceCount = int32(serviceCount)

	if identityCount == 0 {
		return r.markFailed(ctx, &policy, fmt.Errorf("identity selector matched zero identities"), "SelectorResolutionFailed", false)
	}
	if serviceCount == 0 {
		return r.markFailed(ctx, &policy, fmt.Errorf("service selector matched zero services"), "SelectorResolutionFailed", false)
	}

	desired := policyservice.FromResource(&policy, identitySelector, serviceSelector)
	backendPolicy, err := r.reconcilePolicy(ctx, &policy, desired)
	if err != nil {
		return r.markFailed(ctx, &policy, err, "PolicySyncFailed", true)
	}

	policy.Status.ID = backendPolicy.ID
	if err := r.markReady(ctx, &policy); err != nil {
		return ctrl.Result{}, err
	}

	logger.Info("reconciled ziti access policy", "policyID", policy.Status.ID, "resolvedIdentityCount", policy.Status.ResolvedIdentityCount, "resolvedServiceCount", policy.Status.ResolvedServiceCount)
	EmitEvent(r.Recorder, &policy, corev1.EventTypeNormal, "PolicyReconciled", "ZitiAccessPolicy reconciled successfully")
	return ctrl.Result{}, nil
}

func (r *ZitiAccessPolicyReconciler) reconcilePolicy(
	ctx context.Context,
	resource *zitiv1alpha1.ZitiAccessPolicy,
	desired policyservice.DesiredPolicy,
) (*openziti.AccessPolicy, error) {
	if resource.Status.ID != "" {
		return r.PolicyService.Update(ctx, resource.Status.ID, desired)
	}
	existing, err := r.PolicyService.FindByName(ctx, desired.Name)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return r.PolicyService.Update(ctx, existing.ID, desired)
	}
	return r.PolicyService.Create(ctx, desired)
}

func (r *ZitiAccessPolicyReconciler) resolveSelectors(
	ctx context.Context,
	policy *zitiv1alpha1.ZitiAccessPolicy,
) (policyservice.ResolvedSelector, policyservice.ResolvedSelector, int, int, error) {
	var identities zitiv1alpha1.ZitiIdentityList
	if err := r.List(ctx, &identities, client.InNamespace(policy.Namespace)); err != nil {
		return policyservice.ResolvedSelector{}, policyservice.ResolvedSelector{}, 0, 0, err
	}
	var services zitiv1alpha1.ZitiServiceList
	if err := r.List(ctx, &services, client.InNamespace(policy.Namespace)); err != nil {
		return policyservice.ResolvedSelector{}, policyservice.ResolvedSelector{}, 0, 0, err
	}

	identitySelector := policyservice.ResolvedSelector{
		RoleAttributes: append([]string(nil), policy.Spec.IdentitySelector.MatchRoleAttributes...),
	}
	identityCount := 0
	for i := range identities.Items {
		if matchesSelector(policy.Spec.IdentitySelector, identities.Items[i].Spec.Name, identities.Items[i].Spec.RoleAttributes) {
			identityCount++
		}
		if matchesNames(policy.Spec.IdentitySelector.MatchNames, identities.Items[i].Spec.Name) && identities.Items[i].Status.ID != "" {
			identitySelector.IDs = append(identitySelector.IDs, identities.Items[i].Status.ID)
		}
	}

	serviceSelector := policyservice.ResolvedSelector{
		RoleAttributes: append([]string(nil), policy.Spec.ServiceSelector.MatchRoleAttributes...),
	}
	serviceCount := 0
	for i := range services.Items {
		if matchesSelector(policy.Spec.ServiceSelector, services.Items[i].Spec.Name, services.Items[i].Spec.RoleAttributes) {
			serviceCount++
		}
		if matchesNames(policy.Spec.ServiceSelector.MatchNames, services.Items[i].Spec.Name) && services.Items[i].Status.ID != "" {
			serviceSelector.IDs = append(serviceSelector.IDs, services.Items[i].Status.ID)
		}
	}

	return identitySelector, serviceSelector, identityCount, serviceCount, nil
}

func (r *ZitiAccessPolicyReconciler) reconcileDelete(ctx context.Context, policy *zitiv1alpha1.ZitiAccessPolicy) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	if !slices.Contains(policy.GetFinalizers(), zitiAccessPolicyFinalizer) {
		return ctrl.Result{}, nil
	}
	if policy.Status.ID != "" {
		if err := r.PolicyService.Delete(ctx, policy.Status.ID); err != nil {
			return ctrl.Result{}, err
		}
	}
	RemoveFinalizer(policy, zitiAccessPolicyFinalizer)
	if err := r.Update(ctx, policy); err != nil {
		return ctrl.Result{}, err
	}
	logger.Info("deleted ziti access policy backend state", "policyID", policy.Status.ID)
	EmitEvent(r.Recorder, policy, corev1.EventTypeNormal, "PolicyDeleted", "ZitiAccessPolicy backend state removed")
	return ctrl.Result{}, nil
}

func (r *ZitiAccessPolicyReconciler) markReady(ctx context.Context, policy *zitiv1alpha1.ZitiAccessPolicy) error {
	policy.Status.ObservedGeneration = policy.Generation
	policy.Status.LastError = ""
	SetStatusCondition(&policy.Status.CommonStatus, metav1.Condition{
		Type:               zitiv1alpha1.ConditionTypeReady,
		Status:             metav1.ConditionTrue,
		Reason:             "Reconciled",
		Message:            "ZitiAccessPolicy reconciled successfully",
		ObservedGeneration: policy.Generation,
		LastTransitionTime: metav1.Now(),
	})
	SetStatusCondition(&policy.Status.CommonStatus, metav1.Condition{
		Type:               zitiv1alpha1.ConditionTypeReconciling,
		Status:             metav1.ConditionFalse,
		Reason:             "Idle",
		Message:            "No reconciliation in progress",
		ObservedGeneration: policy.Generation,
		LastTransitionTime: metav1.Now(),
	})
	SetStatusCondition(&policy.Status.CommonStatus, metav1.Condition{
		Type:               zitiv1alpha1.ConditionTypeDegraded,
		Status:             metav1.ConditionFalse,
		Reason:             "AsExpected",
		Message:            "No actionable reconciliation errors",
		ObservedGeneration: policy.Generation,
		LastTransitionTime: metav1.Now(),
	})
	return r.Status().Update(ctx, policy)
}

func (r *ZitiAccessPolicyReconciler) markFailed(ctx context.Context, policy *zitiv1alpha1.ZitiAccessPolicy, reconcileErr error, reason string, retryable bool) (ctrl.Result, error) {
	message := normalizeReconcileErrorMessage(reconcileErr.Error())
	if hasMatchingFailureStatus(policy.Status.CommonStatus, policy.Generation, reason, message) {
		if retryable {
			return RequeueWithError(reconcileErr)
		}
		return ctrl.Result{}, nil
	}
	previous := policy.Status.DeepCopy()
	policy.Status.ObservedGeneration = policy.Generation
	policy.Status.LastError = message
	SetStatusCondition(&policy.Status.CommonStatus, metav1.Condition{
		Type:               zitiv1alpha1.ConditionTypeReady,
		Status:             metav1.ConditionFalse,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: policy.Generation,
		LastTransitionTime: metav1.Now(),
	})
	SetStatusCondition(&policy.Status.CommonStatus, metav1.Condition{
		Type:               zitiv1alpha1.ConditionTypeReconciling,
		Status:             metav1.ConditionFalse,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: policy.Generation,
		LastTransitionTime: metav1.Now(),
	})
	SetStatusCondition(&policy.Status.CommonStatus, metav1.Condition{
		Type:               zitiv1alpha1.ConditionTypeDegraded,
		Status:             metav1.ConditionTrue,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: policy.Generation,
		LastTransitionTime: metav1.Now(),
	})
	if !apiequality.Semantic.DeepEqual(previous, &policy.Status) {
		if err := r.Status().Update(ctx, policy); err != nil {
			return ctrl.Result{}, err
		}
		EmitEvent(r.Recorder, policy, corev1.EventTypeWarning, reason, message)
	}
	if retryable {
		return RequeueWithError(reconcileErr)
	}
	return ctrl.Result{}, nil
}

func (r *ZitiAccessPolicyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&zitiv1alpha1.ZitiAccessPolicy{}).
		Complete(r)
}

func validateAccessPolicySpec(policy *zitiv1alpha1.ZitiAccessPolicy) error {
	switch policy.Spec.Type {
	case zitiv1alpha1.AccessPolicyTypeDial, zitiv1alpha1.AccessPolicyTypeBind:
	default:
		return fmt.Errorf("spec.type must be Dial or Bind")
	}
	if !selectorDefined(policy.Spec.IdentitySelector) {
		return fmt.Errorf("spec.identitySelector must define at least one match field")
	}
	if !selectorDefined(policy.Spec.ServiceSelector) {
		return fmt.Errorf("spec.serviceSelector must define at least one match field")
	}
	return nil
}

func selectorDefined(selector zitiv1alpha1.SelectorSpec) bool {
	return len(selector.MatchNames) > 0 || len(selector.MatchRoleAttributes) > 0
}

func matchesNames(matchNames []string, name string) bool {
	for _, candidate := range matchNames {
		if candidate == name {
			return true
		}
	}
	return false
}

func matchesSelector(selector zitiv1alpha1.SelectorSpec, name string, roleAttributes []string) bool {
	if matchesNames(selector.MatchNames, name) {
		return true
	}
	for _, candidate := range selector.MatchRoleAttributes {
		if slices.Contains(roleAttributes, candidate) {
			return true
		}
	}
	return false
}
