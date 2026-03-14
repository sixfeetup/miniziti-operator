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

import (
	"context"

	"example.com/miniziti-operator/internal/openziti/client"
)

// Service maps ZitiIdentity resources to the OpenZiti identity API.
type Service struct {
	client client.Client
}

// NewService constructs a Service around the shared OpenZiti client.
func NewService(client client.Client) *Service {
	return &Service{client: client}
}

// FindByName locates an OpenZiti identity by its external name.
func (s *Service) FindByName(ctx context.Context, name string) (*client.Identity, error) {
	return s.client.FindIdentityByName(ctx, name)
}

// Get fetches an OpenZiti identity by external id.
func (s *Service) Get(ctx context.Context, id string) (*client.Identity, error) {
	return s.client.GetIdentity(ctx, id)
}

// Create provisions a new OpenZiti identity from the desired resource state.
func (s *Service) Create(ctx context.Context, desired DesiredIdentity) (*client.Identity, error) {
	return s.client.CreateIdentity(ctx, toClientIdentity(desired))
}

// Update syncs an existing OpenZiti identity to the desired resource state.
func (s *Service) Update(ctx context.Context, id string, desired DesiredIdentity) (*client.Identity, error) {
	identity := toClientIdentity(desired)
	identity.ID = id
	return s.client.UpdateIdentity(ctx, identity)
}

// Delete removes an OpenZiti identity by id.
func (s *Service) Delete(ctx context.Context, id string) error {
	return s.client.DeleteIdentity(ctx, id)
}

// EnrollmentJWT fetches enrollment material for an existing OpenZiti identity.
func (s *Service) EnrollmentJWT(ctx context.Context, id string) (string, error) {
	return s.client.GetEnrollmentJWT(ctx, id)
}

func toClientIdentity(desired DesiredIdentity) client.Identity {
	return client.Identity{
		Name:           desired.Name,
		Type:           desired.Type,
		RoleAttributes: append([]string(nil), desired.RoleAttributes...),
	}
}
