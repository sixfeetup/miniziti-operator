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

	desired := zitiservice.FromResource(&service)
	backendService, err := r.reconcileService(ctx, &service, desired)
	if err != nil {
		logger.Error(err, "service reconciliation failed")
		return r.markFailed(ctx, &service, err, "ServiceSyncFailed")
	}

	interceptConfig, err := r.reconcileConfig(ctx, service.Status.ConfigIDs.Intercept, desired.Intercept)
	if err != nil {
		logger.Error(err, "intercept config reconciliation failed")
		return r.markFailed(ctx, &service, err, "InterceptConfigFailed")
	}
	hostConfig, err := r.reconcileConfig(ctx, service.Status.ConfigIDs.Host, desired.Host)
	if err != nil {
		logger.Error(err, "host config reconciliation failed")
		return r.markFailed(ctx, &service, err, "HostConfigFailed")
	}

	service.Status.ID = backendService.ID
	service.Status.ConfigIDs.Intercept = interceptConfig.ID
	service.Status.ConfigIDs.Host = hostConfig.ID

	if err := r.markReady(ctx, &service); err != nil {
		return ctrl.Result{}, err
	}

	EmitEvent(r.Recorder, &service, corev1.EventTypeNormal, "Reconciled", "ZitiService reconciled successfully")
	return ctrl.Result{}, nil
}

func (r *ZitiServiceReconciler) reconcileService(
	ctx context.Context,
	resource *zitiv1alpha1.ZitiService,
	desired zitiservice.DesiredService,
) (*openziti.Service, error) {
	if err := validateServiceSpec(resource); err != nil {
		return nil, err
	}
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

func (r *ZitiServiceReconciler) reconcileDelete(ctx context.Context, service *zitiv1alpha1.ZitiService) (ctrl.Result, error) {
	if !slices.Contains(service.GetFinalizers(), zitiServiceFinalizer) {
		return ctrl.Result{}, nil
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
	EmitEvent(r.Recorder, service, corev1.EventTypeNormal, "Deleted", "ZitiService backend state removed")
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

func (r *ZitiServiceReconciler) markFailed(ctx context.Context, service *zitiv1alpha1.ZitiService, reconcileErr error, reason string) (ctrl.Result, error) {
	service.Status.ObservedGeneration = service.Generation
	service.Status.LastError = reconcileErr.Error()
	SetStatusCondition(&service.Status.CommonStatus, metav1.Condition{
		Type:               zitiv1alpha1.ConditionTypeReady,
		Status:             metav1.ConditionFalse,
		Reason:             reason,
		Message:            reconcileErr.Error(),
		ObservedGeneration: service.Generation,
		LastTransitionTime: metav1.Now(),
	})
	SetStatusCondition(&service.Status.CommonStatus, metav1.Condition{
		Type:               zitiv1alpha1.ConditionTypeReconciling,
		Status:             metav1.ConditionFalse,
		Reason:             reason,
		Message:            reconcileErr.Error(),
		ObservedGeneration: service.Generation,
		LastTransitionTime: metav1.Now(),
	})
	SetStatusCondition(&service.Status.CommonStatus, metav1.Condition{
		Type:               zitiv1alpha1.ConditionTypeDegraded,
		Status:             metav1.ConditionTrue,
		Reason:             reason,
		Message:            reconcileErr.Error(),
		ObservedGeneration: service.Generation,
		LastTransitionTime: metav1.Now(),
	})
	if err := r.Status().Update(ctx, service); err != nil {
		return ctrl.Result{}, err
	}
	EmitEvent(r.Recorder, service, corev1.EventTypeWarning, reason, reconcileErr.Error())
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
	for _, portRange := range service.Spec.Configs.Intercept.PortRanges {
		if portRange.Low > portRange.High {
			return fmt.Errorf("spec.configs.intercept.portRanges low must be <= high")
		}
	}
	return nil
}
