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

package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

const (
	// ConditionTypeReady reports whether reconciliation has produced the desired backend state.
	ConditionTypeReady = "Ready"
	// ConditionTypeReconciling reports whether reconciliation is actively working toward the desired state.
	ConditionTypeReconciling = "Reconciling"
	// ConditionTypeDegraded reports whether reconciliation hit an actionable failure.
	ConditionTypeDegraded = "Degraded"
)

// CommonStatus captures the status fields shared by all Miniziti resources.
type CommonStatus struct {
	// ID is the external OpenZiti identifier for the managed object.
	// +optional
	ID string `json:"id,omitempty"`
	// Conditions reports readiness, reconciliation progress, and degraded states.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// ObservedGeneration is the most recent metadata generation processed by the operator.
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
	// LastError stores the most recent actionable reconciliation error.
	// +optional
	LastError string `json:"lastError,omitempty"`
}

// GetConditions returns the current condition slice.
func (s *CommonStatus) GetConditions() []metav1.Condition {
	if s == nil {
		return nil
	}
	return s.Conditions
}

// SetConditions replaces the current condition slice.
func (s *CommonStatus) SetConditions(conditions []metav1.Condition) {
	if s == nil {
		return
	}
	s.Conditions = conditions
}
