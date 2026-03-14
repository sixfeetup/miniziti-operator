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
	"slices"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type statusConditionAccessor interface {
	GetConditions() []metav1.Condition
	SetConditions([]metav1.Condition)
}

type finalizerAccessor interface {
	client.Object
}

// SetStatusCondition upserts a condition on a resource-specific status wrapper.
func SetStatusCondition(status statusConditionAccessor, condition metav1.Condition) {
	conditions := status.GetConditions()
	apimeta.SetStatusCondition(&conditions, condition)
	status.SetConditions(conditions)
}

// RemoveStatusCondition removes a condition type from the given status wrapper.
func RemoveStatusCondition(status statusConditionAccessor, conditionType string) {
	conditions := status.GetConditions()
	filtered := conditions[:0]
	for _, condition := range conditions {
		if condition.Type != conditionType {
			filtered = append(filtered, condition)
		}
	}
	status.SetConditions(filtered)
}

// EnsureFinalizer adds the finalizer if it is not already present.
func EnsureFinalizer(obj finalizerAccessor, finalizer string) bool {
	if slices.Contains(obj.GetFinalizers(), finalizer) {
		return false
	}
	obj.SetFinalizers(append(obj.GetFinalizers(), finalizer))
	return true
}

// RemoveFinalizer removes the finalizer if it is present.
func RemoveFinalizer(obj finalizerAccessor, finalizer string) bool {
	finalizers := obj.GetFinalizers()
	if !slices.Contains(finalizers, finalizer) {
		return false
	}

	filtered := finalizers[:0]
	for _, existing := range finalizers {
		if existing != finalizer {
			filtered = append(filtered, existing)
		}
	}
	obj.SetFinalizers(filtered)
	return true
}

// IgnoreNotFound returns nil for Kubernetes not found errors.
func IgnoreNotFound(err error) error {
	if apierrors.IsNotFound(err) {
		return nil
	}
	return err
}

// EmitEvent records a Kubernetes event when a recorder is configured.
func EmitEvent(recorder record.EventRecorder, obj runtime.Object, eventType, reason, message string) {
	if recorder == nil {
		return
	}
	recorder.Event(obj, eventType, reason, message)
}

// RequeueWithError returns a zero result and the given error for rate-limited retries.
func RequeueWithError(err error) (ctrl.Result, error) {
	return ctrl.Result{}, err
}
