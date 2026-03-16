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

package policy

import (
	"context"
	"strings"

	zitiv1alpha1 "example.com/miniziti-operator/api/v1alpha1"
	openziti "example.com/miniziti-operator/internal/openziti/client"
)

const semanticAnyOf = "AnyOf"

// DesiredPolicy captures the normalized operator view of a ZitiAccessPolicy resource.
type DesiredPolicy struct {
	Name             string
	Type             string
	IdentityRoles    []string
	ServiceRoles     []string
	IdentityRolesRaw []string
	ServiceRolesRaw  []string
	Semantic         string
}

type ResolvedSelector struct {
	IDs            []string
	RoleAttributes []string
}

// Service coordinates ZitiAccessPolicy resources with the OpenZiti policy API.
type Service struct {
	client openziti.Client
}

// NewService constructs a Service around the shared OpenZiti client.
func NewService(client openziti.Client) *Service {
	return &Service{client: client}
}

// FromResource maps a ZitiAccessPolicy resource into the backend policy payload.
func FromResource(resource *zitiv1alpha1.ZitiAccessPolicy, identitySelector, serviceSelector ResolvedSelector) DesiredPolicy {
	return DesiredPolicy{
		Name:             resource.Name,
		Type:             resource.Spec.Type,
		IdentityRoles:    selectorRoles(identitySelector),
		ServiceRoles:     selectorRoles(serviceSelector),
		IdentityRolesRaw: selectorRoles(identitySelector),
		ServiceRolesRaw:  selectorRoles(serviceSelector),
		Semantic:         semanticAnyOf,
	}
}

func selectorRoles(selector ResolvedSelector) []string {
	roles := make([]string, 0, len(selector.IDs)+len(selector.RoleAttributes))
	seen := make(map[string]struct{}, len(selector.IDs)+len(selector.RoleAttributes))

	for _, id := range selector.IDs {
		expression := "@" + strings.TrimSpace(id)
		if expression == "@" {
			continue
		}
		if _, ok := seen[expression]; ok {
			continue
		}
		seen[expression] = struct{}{}
		roles = append(roles, expression)
	}

	for _, attribute := range selector.RoleAttributes {
		expression := "#" + strings.TrimSpace(attribute)
		if expression == "#" {
			continue
		}
		if _, ok := seen[expression]; ok {
			continue
		}
		seen[expression] = struct{}{}
		roles = append(roles, expression)
	}

	return roles
}

func (s *Service) FindByName(ctx context.Context, name string) (*openziti.AccessPolicy, error) {
	return s.client.FindAccessPolicyByName(ctx, name)
}

func (s *Service) Get(ctx context.Context, id string) (*openziti.AccessPolicy, error) {
	return s.client.GetAccessPolicy(ctx, id)
}

func (s *Service) Create(ctx context.Context, desired DesiredPolicy) (*openziti.AccessPolicy, error) {
	return s.client.CreateAccessPolicy(ctx, openziti.AccessPolicy{
		Name:             desired.Name,
		Type:             desired.Type,
		IdentityRoles:    append([]string(nil), desired.IdentityRoles...),
		ServiceRoles:     append([]string(nil), desired.ServiceRoles...),
		IdentityRolesRaw: append([]string(nil), desired.IdentityRolesRaw...),
		ServiceRolesRaw:  append([]string(nil), desired.ServiceRolesRaw...),
		Semantic:         desired.Semantic,
	})
}

func (s *Service) Update(ctx context.Context, id string, desired DesiredPolicy) (*openziti.AccessPolicy, error) {
	return s.client.UpdateAccessPolicy(ctx, openziti.AccessPolicy{
		ID:               id,
		Name:             desired.Name,
		Type:             desired.Type,
		IdentityRoles:    append([]string(nil), desired.IdentityRoles...),
		ServiceRoles:     append([]string(nil), desired.ServiceRoles...),
		IdentityRolesRaw: append([]string(nil), desired.IdentityRolesRaw...),
		ServiceRolesRaw:  append([]string(nil), desired.ServiceRolesRaw...),
		Semantic:         desired.Semantic,
	})
}

func (s *Service) Delete(ctx context.Context, id string) error {
	return s.client.DeleteAccessPolicy(ctx, id)
}
