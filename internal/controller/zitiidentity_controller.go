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
	"example.com/miniziti-operator/internal/credentials"
	openziti "example.com/miniziti-operator/internal/openziti/client"
	identityservice "example.com/miniziti-operator/internal/openziti/identity"
)

const zitiIdentityFinalizer = "ziti.sixfeetup.com/finalizer"

// ZitiIdentityReconciler reconciles a ZitiIdentity object.
type ZitiIdentityReconciler struct {
	client.Client
	Scheme          *runtime.Scheme
	Recorder        record.EventRecorder
	IdentityService *identityservice.Service
}

// +kubebuilder:rbac:groups=ziti.sixfeetup.com,resources=zitiidentities,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ziti.sixfeetup.com,resources=zitiidentities/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=ziti.sixfeetup.com,resources=zitiidentities/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

func (r *ZitiIdentityReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var identity zitiv1alpha1.ZitiIdentity
	if err := r.Get(ctx, req.NamespacedName, &identity); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if identity.DeletionTimestamp.IsZero() {
		if EnsureFinalizer(&identity, zitiIdentityFinalizer) {
			if err := r.Update(ctx, &identity); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{Requeue: true}, nil
		}
	} else {
		return r.reconcileDelete(ctx, &identity)
	}

	if err := validateIdentitySpec(&identity); err != nil {
		return r.markFailed(ctx, &identity, err, "SpecValidationFailed", false)
	}

	desired := identityservice.FromResource(&identity)
	backendIdentity, err := r.reconcileIdentity(ctx, &identity, desired)
	if err != nil {
		return r.markFailed(ctx, &identity, err, "IdentitySyncFailed", true)
	}

	identity.Status.ID = backendIdentity.ID
	identity.Status.JWTSecretName = ""

	if desired.CreateJWTSecret {
		jwt, err := r.IdentityService.EnrollmentJWT(ctx, backendIdentity.ID)
		if err != nil {
			return r.markFailed(ctx, &identity, err, "EnrollmentJWTFailed", true)
		}
		secretName, err := credentials.ReconcileEnrollmentSecret(ctx, r.Client, r.Scheme, &identity, jwt)
		if err != nil {
			logger.Error(err, "failed to reconcile enrollment secret")
			return r.markFailed(ctx, &identity, err, "EnrollmentSecretFailed", false)
		}
		identity.Status.JWTSecretName = secretName
	}

	if err := r.markReady(ctx, &identity); err != nil {
		return ctrl.Result{}, err
	}

	logger.Info("reconciled ziti identity", "identityID", identity.Status.ID, "jwtSecretName", identity.Status.JWTSecretName)
	EmitEvent(r.Recorder, &identity, corev1.EventTypeNormal, "IdentityReconciled", "ZitiIdentity reconciled successfully")
	return ctrl.Result{}, nil
}

func (r *ZitiIdentityReconciler) reconcileIdentity(
	ctx context.Context,
	identity *zitiv1alpha1.ZitiIdentity,
	desired identityservice.DesiredIdentity,
) (*openziti.Identity, error) {
	if identity.Status.ID != "" {
		return r.IdentityService.Update(ctx, identity.Status.ID, desired)
	}

	existing, err := r.IdentityService.FindByName(ctx, desired.Name)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return r.IdentityService.Update(ctx, existing.ID, desired)
	}

	return r.IdentityService.Create(ctx, desired)
}

func (r *ZitiIdentityReconciler) reconcileDelete(ctx context.Context, identity *zitiv1alpha1.ZitiIdentity) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	if !slices.Contains(identity.GetFinalizers(), zitiIdentityFinalizer) {
		return ctrl.Result{}, nil
	}

	if err := credentials.DeleteEnrollmentSecret(ctx, r.Client, identity, identity.Status.JWTSecretName); err != nil {
		return ctrl.Result{}, err
	}
	if identity.Status.ID != "" {
		if err := r.IdentityService.Delete(ctx, identity.Status.ID); err != nil {
			return ctrl.Result{}, err
		}
	}
	RemoveFinalizer(identity, zitiIdentityFinalizer)
	if err := r.Update(ctx, identity); err != nil {
		return ctrl.Result{}, err
	}
	logger.Info("deleted ziti identity backend state", "identityID", identity.Status.ID, "jwtSecretName", identity.Status.JWTSecretName)
	EmitEvent(r.Recorder, identity, corev1.EventTypeNormal, "IdentityDeleted", "ZitiIdentity backend state removed")
	return ctrl.Result{}, nil
}

func (r *ZitiIdentityReconciler) markReady(ctx context.Context, identity *zitiv1alpha1.ZitiIdentity) error {
	identity.Status.ObservedGeneration = identity.Generation
	identity.Status.LastError = ""
	SetStatusCondition(&identity.Status.CommonStatus, metav1.Condition{
		Type:               zitiv1alpha1.ConditionTypeReady,
		Status:             metav1.ConditionTrue,
		Reason:             "Reconciled",
		Message:            "ZitiIdentity reconciled successfully",
		ObservedGeneration: identity.Generation,
		LastTransitionTime: metav1.Now(),
	})
	SetStatusCondition(&identity.Status.CommonStatus, metav1.Condition{
		Type:               zitiv1alpha1.ConditionTypeReconciling,
		Status:             metav1.ConditionFalse,
		Reason:             "Idle",
		Message:            "No reconciliation in progress",
		ObservedGeneration: identity.Generation,
		LastTransitionTime: metav1.Now(),
	})
	SetStatusCondition(&identity.Status.CommonStatus, metav1.Condition{
		Type:               zitiv1alpha1.ConditionTypeDegraded,
		Status:             metav1.ConditionFalse,
		Reason:             "AsExpected",
		Message:            "No actionable reconciliation errors",
		ObservedGeneration: identity.Generation,
		LastTransitionTime: metav1.Now(),
	})
	return r.Status().Update(ctx, identity)
}

func (r *ZitiIdentityReconciler) markFailed(ctx context.Context, identity *zitiv1alpha1.ZitiIdentity, reconcileErr error, reason string, retryable bool) (ctrl.Result, error) {
	message := normalizeReconcileErrorMessage(reconcileErr.Error())
	if hasMatchingFailureStatus(identity.Status.CommonStatus, identity.Generation, reason, message) {
		if retryable {
			return RequeueWithError(reconcileErr)
		}
		return ctrl.Result{}, nil
	}
	previous := identity.Status.DeepCopy()
	identity.Status.ObservedGeneration = identity.Generation
	identity.Status.LastError = message
	SetStatusCondition(&identity.Status.CommonStatus, metav1.Condition{
		Type:               zitiv1alpha1.ConditionTypeReady,
		Status:             metav1.ConditionFalse,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: identity.Generation,
		LastTransitionTime: metav1.Now(),
	})
	SetStatusCondition(&identity.Status.CommonStatus, metav1.Condition{
		Type:               zitiv1alpha1.ConditionTypeReconciling,
		Status:             metav1.ConditionFalse,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: identity.Generation,
		LastTransitionTime: metav1.Now(),
	})
	SetStatusCondition(&identity.Status.CommonStatus, metav1.Condition{
		Type:               zitiv1alpha1.ConditionTypeDegraded,
		Status:             metav1.ConditionTrue,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: identity.Generation,
		LastTransitionTime: metav1.Now(),
	})
	if !apiequality.Semantic.DeepEqual(previous, &identity.Status) {
		if err := r.Status().Update(ctx, identity); err != nil {
			return ctrl.Result{}, err
		}
		EmitEvent(r.Recorder, identity, corev1.EventTypeWarning, reason, message)
	}
	if retryable {
		return RequeueWithError(reconcileErr)
	}
	return ctrl.Result{}, nil
}

func (r *ZitiIdentityReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&zitiv1alpha1.ZitiIdentity{}).
		Owns(&corev1.Secret{}).
		Complete(r)
}

func validateIdentitySpec(identity *zitiv1alpha1.ZitiIdentity) error {
	if identity.Spec.Name == "" {
		return fmt.Errorf("spec.name must not be empty")
	}
	seen := map[string]struct{}{}
	for _, attr := range identity.Spec.RoleAttributes {
		if _, ok := seen[attr]; ok {
			return fmt.Errorf("spec.roleAttributes must not contain duplicates")
		}
		seen[attr] = struct{}{}
	}
	if identity.Spec.Enrollment.CreateJWTSecret && identity.Spec.Enrollment.JWTSecretName == "" {
		return fmt.Errorf("spec.enrollment.jwtSecretName is required when createJwtSecret is true")
	}
	return nil
}
