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
	"strings"

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
	zitiservice "example.com/miniziti-operator/internal/openziti/service"
)

const zitiServiceFinalizer = "ziti.sixfeetup.com/service-finalizer"

// ZitiServiceReconciler reconciles a ZitiService object.
type ZitiServiceReconciler struct {
	client.Client
	Scheme         *runtime.Scheme
	Recorder       record.EventRecorder
	ServiceManager *zitiservice.Service
}

// +kubebuilder:rbac:groups=ziti.sixfeetup.com,resources=zitiservices,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=ziti.sixfeetup.com,resources=zitiservices/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=ziti.sixfeetup.com,resources=zitiservices/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

func (r *ZitiServiceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	var service zitiv1alpha1.ZitiService
	if err := r.Get(ctx, req.NamespacedName, &service); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if service.DeletionTimestamp.IsZero() {
		if EnsureFinalizer(&service, zitiServiceFinalizer) {
			if err := r.Update(ctx, &service); err != nil {
				return ctrl.Result{}, err
			}
			return ctrl.Result{Requeue: true}, nil
		}
	} else {
		return r.reconcileDelete(ctx, &service)
	}

	if err := validateServiceSpec(&service); err != nil {
		return r.markFailed(ctx, &service, err, "SpecValidationFailed", false)
	}

	desired := zitiservice.FromResource(&service)
	backendService, err := r.reconcileService(ctx, &service, desired)
	if err != nil {
		return r.markFailed(ctx, &service, err, "ServiceSyncFailed", true)
	}

	interceptConfig, err := r.reconcileConfig(ctx, service.Status.ConfigIDs.Intercept, desired.Intercept)
	if err != nil {
		return r.markFailed(ctx, &service, err, "InterceptConfigFailed", true)
	}
	hostConfig, err := r.reconcileConfig(ctx, service.Status.ConfigIDs.Host, desired.Host)
	if err != nil {
		return r.markFailed(ctx, &service, err, "HostConfigFailed", true)
	}

	desired.ConfigIDs = []string{interceptConfig.ID, hostConfig.ID}
	if !slices.Equal(backendService.ConfigIDs, desired.ConfigIDs) {
		backendService, err = r.ServiceManager.Update(ctx, backendService.ID, desired)
		if err != nil {
			return r.markFailed(ctx, &service, err, "ServiceSyncFailed", true)
		}
	}

	service.Status.ID = backendService.ID
	service.Status.ConfigIDs.Intercept = interceptConfig.ID
	service.Status.ConfigIDs.Host = hostConfig.ID

	if service.Spec.Router != nil {
		routerPolicies, err := r.desiredRouterPolicies(ctx, &service, backendService.ID)
		if err != nil {
			return r.markFailed(ctx, &service, err, "RouterResolutionFailed", true)
		}
		bindPolicy, err := r.reconcileAccessPolicy(ctx, service.Status.BindPolicyID, routerPolicies.BindPolicy)
		if err != nil {
			return r.markFailed(ctx, &service, err, "BindPolicyFailed", true)
		}
		serviceEdgeRouterPolicy, err := r.reconcileServiceEdgeRouterPolicy(ctx, service.Status.ServiceEdgeRouterPolicyID, routerPolicies.ServiceEdgeRouterPolicy)
		if err != nil {
			return r.markFailed(ctx, &service, err, "ServiceEdgeRouterPolicyFailed", true)
		}
		service.Status.BindPolicyID = bindPolicy.ID
		service.Status.ServiceEdgeRouterPolicyID = serviceEdgeRouterPolicy.ID
	} else if err := r.deleteRouterPolicies(ctx, &service); err != nil {
		return r.markFailed(ctx, &service, err, "RouterPolicyCleanupFailed", true)
	}

	if err := r.markReady(ctx, &service); err != nil {
		return ctrl.Result{}, err
	}

	logger.Info("reconciled ziti service", "serviceID", service.Status.ID, "interceptConfigID", service.Status.ConfigIDs.Intercept, "hostConfigID", service.Status.ConfigIDs.Host)
	EmitEvent(r.Recorder, &service, corev1.EventTypeNormal, "ServiceReconciled", "ZitiService reconciled successfully")
	return ctrl.Result{}, nil
}

func (r *ZitiServiceReconciler) reconcileService(
	ctx context.Context,
	resource *zitiv1alpha1.ZitiService,
	desired zitiservice.DesiredService,
) (*openziti.Service, error) {
	if resource.Status.ID != "" {
		return r.ServiceManager.Update(ctx, resource.Status.ID, desired)
	}
	existing, err := r.ServiceManager.FindByName(ctx, desired.Name)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return r.ServiceManager.Update(ctx, existing.ID, desired)
	}
	return r.ServiceManager.Create(ctx, desired)
}

func (r *ZitiServiceReconciler) reconcileConfig(
	ctx context.Context,
	existingID string,
	desired openziti.ServiceConfig,
) (*openziti.ServiceConfig, error) {
	if existingID != "" {
		desired.ID = existingID
		return r.ServiceManager.UpdateConfig(ctx, desired)
	}
	return r.ServiceManager.CreateConfig(ctx, desired)
}

func (r *ZitiServiceReconciler) desiredRouterPolicies(
	ctx context.Context,
	resource *zitiv1alpha1.ZitiService,
	serviceID string,
) (zitiservice.RouterPolicySet, error) {
	routerName := strings.TrimSpace(resource.Spec.Router.Name)
	routerIdentity, err := r.ServiceManager.FindIdentityByName(ctx, routerName)
	if err != nil {
		return zitiservice.RouterPolicySet{}, err
	}
	if routerIdentity == nil || strings.TrimSpace(routerIdentity.ID) == "" {
		return zitiservice.RouterPolicySet{}, fmt.Errorf("router identity %q not found", routerName)
	}
	if !strings.EqualFold(strings.TrimSpace(routerIdentity.Type), "Router") {
		return zitiservice.RouterPolicySet{}, fmt.Errorf("identity %q is not a Router identity", routerName)
	}

	edgeRouter, err := r.ServiceManager.FindEdgeRouterByName(ctx, routerName)
	if err != nil {
		return zitiservice.RouterPolicySet{}, err
	}
	if edgeRouter == nil || strings.TrimSpace(edgeRouter.ID) == "" {
		return zitiservice.RouterPolicySet{}, fmt.Errorf("edge router %q not found", routerName)
	}

	policies, ok := zitiservice.RouterPolicies(resource, serviceID, routerIdentity.ID, edgeRouter.ID)
	if !ok {
		return zitiservice.RouterPolicySet{}, fmt.Errorf("spec.router.name must not be empty")
	}
	return policies, nil
}

func (r *ZitiServiceReconciler) reconcileAccessPolicy(
	ctx context.Context,
	existingID string,
	desired openziti.AccessPolicy,
) (*openziti.AccessPolicy, error) {
	if existingID != "" {
		desired.ID = existingID
		updated, err := r.ServiceManager.UpdateAccessPolicy(ctx, desired)
		if err != nil || updated != nil {
			return updated, err
		}
	}
	existing, err := r.ServiceManager.FindAccessPolicyByName(ctx, desired.Name)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		desired.ID = existing.ID
		return r.ServiceManager.UpdateAccessPolicy(ctx, desired)
	}
	return r.ServiceManager.CreateAccessPolicy(ctx, desired)
}

func (r *ZitiServiceReconciler) reconcileServiceEdgeRouterPolicy(
	ctx context.Context,
	existingID string,
	desired openziti.ServiceEdgeRouterPolicy,
) (*openziti.ServiceEdgeRouterPolicy, error) {
	if existingID != "" {
		desired.ID = existingID
		updated, err := r.ServiceManager.UpdateServiceEdgeRouterPolicy(ctx, desired)
		if err != nil || updated != nil {
			return updated, err
		}
	}
	existing, err := r.ServiceManager.FindServiceEdgeRouterPolicyByName(ctx, desired.Name)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		desired.ID = existing.ID
		return r.ServiceManager.UpdateServiceEdgeRouterPolicy(ctx, desired)
	}
	return r.ServiceManager.CreateServiceEdgeRouterPolicy(ctx, desired)
}

func (r *ZitiServiceReconciler) deleteRouterPolicies(ctx context.Context, service *zitiv1alpha1.ZitiService) error {
	if service.Status.ServiceEdgeRouterPolicyID != "" {
		if err := r.ServiceManager.DeleteServiceEdgeRouterPolicy(ctx, service.Status.ServiceEdgeRouterPolicyID); err != nil {
			return err
		}
		service.Status.ServiceEdgeRouterPolicyID = ""
	}
	if service.Status.BindPolicyID != "" {
		if err := r.ServiceManager.DeleteAccessPolicy(ctx, service.Status.BindPolicyID); err != nil {
			return err
		}
		service.Status.BindPolicyID = ""
	}
	return nil
}

func (r *ZitiServiceReconciler) reconcileDelete(ctx context.Context, service *zitiv1alpha1.ZitiService) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	if !slices.Contains(service.GetFinalizers(), zitiServiceFinalizer) {
		return ctrl.Result{}, nil
	}
	if err := r.deleteRouterPolicies(ctx, service); err != nil {
		return ctrl.Result{}, err
	}
	if service.Status.ConfigIDs.Intercept != "" {
		if err := r.ServiceManager.DeleteConfig(ctx, service.Status.ConfigIDs.Intercept); err != nil {
			return ctrl.Result{}, err
		}
	}
	if service.Status.ConfigIDs.Host != "" {
		if err := r.ServiceManager.DeleteConfig(ctx, service.Status.ConfigIDs.Host); err != nil {
			return ctrl.Result{}, err
		}
	}
	if service.Status.ID != "" {
		if err := r.ServiceManager.Delete(ctx, service.Status.ID); err != nil {
			return ctrl.Result{}, err
		}
	}
	RemoveFinalizer(service, zitiServiceFinalizer)
	if err := r.Update(ctx, service); err != nil {
		return ctrl.Result{}, err
	}
	logger.Info("deleted ziti service backend state", "serviceID", service.Status.ID, "interceptConfigID", service.Status.ConfigIDs.Intercept, "hostConfigID", service.Status.ConfigIDs.Host)
	EmitEvent(r.Recorder, service, corev1.EventTypeNormal, "ServiceDeleted", "ZitiService backend state removed")
	return ctrl.Result{}, nil
}

func (r *ZitiServiceReconciler) markReady(ctx context.Context, service *zitiv1alpha1.ZitiService) error {
	service.Status.ObservedGeneration = service.Generation
	service.Status.LastError = ""
	SetStatusCondition(&service.Status.CommonStatus, metav1.Condition{
		Type:               zitiv1alpha1.ConditionTypeReady,
		Status:             metav1.ConditionTrue,
		Reason:             "Reconciled",
		Message:            "ZitiService reconciled successfully",
		ObservedGeneration: service.Generation,
		LastTransitionTime: metav1.Now(),
	})
	SetStatusCondition(&service.Status.CommonStatus, metav1.Condition{
		Type:               zitiv1alpha1.ConditionTypeReconciling,
		Status:             metav1.ConditionFalse,
		Reason:             "Idle",
		Message:            "No reconciliation in progress",
		ObservedGeneration: service.Generation,
		LastTransitionTime: metav1.Now(),
	})
	SetStatusCondition(&service.Status.CommonStatus, metav1.Condition{
		Type:               zitiv1alpha1.ConditionTypeDegraded,
		Status:             metav1.ConditionFalse,
		Reason:             "AsExpected",
		Message:            "No actionable reconciliation errors",
		ObservedGeneration: service.Generation,
		LastTransitionTime: metav1.Now(),
	})
	return r.Status().Update(ctx, service)
}

func (r *ZitiServiceReconciler) markFailed(ctx context.Context, service *zitiv1alpha1.ZitiService, reconcileErr error, reason string, retryable bool) (ctrl.Result, error) {
	message := normalizeReconcileErrorMessage(reconcileErr.Error())
	if hasMatchingFailureStatus(service.Status.CommonStatus, service.Generation, reason, message) {
		if retryable {
			return RequeueWithError(reconcileErr)
		}
		return ctrl.Result{}, nil
	}
	previous := service.Status.DeepCopy()
	service.Status.ObservedGeneration = service.Generation
	service.Status.LastError = message
	SetStatusCondition(&service.Status.CommonStatus, metav1.Condition{
		Type:               zitiv1alpha1.ConditionTypeReady,
		Status:             metav1.ConditionFalse,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: service.Generation,
		LastTransitionTime: metav1.Now(),
	})
	SetStatusCondition(&service.Status.CommonStatus, metav1.Condition{
		Type:               zitiv1alpha1.ConditionTypeReconciling,
		Status:             metav1.ConditionFalse,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: service.Generation,
		LastTransitionTime: metav1.Now(),
	})
	SetStatusCondition(&service.Status.CommonStatus, metav1.Condition{
		Type:               zitiv1alpha1.ConditionTypeDegraded,
		Status:             metav1.ConditionTrue,
		Reason:             reason,
		Message:            message,
		ObservedGeneration: service.Generation,
		LastTransitionTime: metav1.Now(),
	})
	if !apiequality.Semantic.DeepEqual(previous, &service.Status) {
		if err := r.Status().Update(ctx, service); err != nil {
			return ctrl.Result{}, err
		}
		EmitEvent(r.Recorder, service, corev1.EventTypeWarning, reason, message)
	}
	if retryable {
		return RequeueWithError(reconcileErr)
	}
	return ctrl.Result{}, nil
}

func (r *ZitiServiceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&zitiv1alpha1.ZitiService{}).
		Complete(r)
}

func validateServiceSpec(service *zitiv1alpha1.ZitiService) error {
	if service.Spec.Name == "" {
		return fmt.Errorf("spec.name must not be empty")
	}
	if service.Spec.Router != nil && strings.TrimSpace(service.Spec.Router.Name) == "" {
		return fmt.Errorf("spec.router.name must not be empty")
	}
	for _, portRange := range service.Spec.Configs.Intercept.PortRanges {
		if portRange.Low > portRange.High {
			return fmt.Errorf("spec.configs.intercept.portRanges low must be <= high")
		}
	}
	return nil
}
