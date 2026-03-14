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

package identity

import zitiapiv1alpha1 "example.com/miniziti-operator/api/v1alpha1"

// DesiredIdentity is the normalized operator view of a ZitiIdentity spec.
type DesiredIdentity struct {
	Name            string
	Type            string
	RoleAttributes  []string
	CreateJWTSecret bool
	JWTSecretName   string
}

// FromResource maps a CRD resource into the identity service's desired-state model.
func FromResource(resource *zitiapiv1alpha1.ZitiIdentity) DesiredIdentity {
	return DesiredIdentity{
		Name:            resource.Spec.Name,
		Type:            resource.Spec.Type,
		RoleAttributes:  append([]string(nil), resource.Spec.RoleAttributes...),
		CreateJWTSecret: resource.Spec.Enrollment.CreateJWTSecret,
		JWTSecretName:   resource.Spec.Enrollment.JWTSecretName,
	}
}
