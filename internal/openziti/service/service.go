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

package service

import (
	"context"

	openziti "example.com/miniziti-operator/internal/openziti/client"
)

// Service coordinates ZitiService resources with OpenZiti service/config APIs.
type Service struct {
	client openziti.Client
}

// NewService constructs a Service around the shared OpenZiti client.
func NewService(client openziti.Client) *Service {
	return &Service{client: client}
}

func (s *Service) FindByName(ctx context.Context, name string) (*openziti.Service, error) {
	return s.client.FindServiceByName(ctx, name)
}

func (s *Service) Get(ctx context.Context, id string) (*openziti.Service, error) {
	return s.client.GetService(ctx, id)
}

func (s *Service) Create(ctx context.Context, desired DesiredService) (*openziti.Service, error) {
	return s.client.CreateService(ctx, openziti.Service{
		Name:           desired.Name,
		RoleAttributes: append([]string(nil), desired.RoleAttributes...),
	})
}

func (s *Service) Update(ctx context.Context, id string, desired DesiredService) (*openziti.Service, error) {
	service := openziti.Service{
		ID:             id,
		Name:           desired.Name,
		RoleAttributes: append([]string(nil), desired.RoleAttributes...),
	}
	return s.client.UpdateService(ctx, service)
}

func (s *Service) Delete(ctx context.Context, id string) error {
	return s.client.DeleteService(ctx, id)
}

func (s *Service) CreateConfig(ctx context.Context, cfg openziti.ServiceConfig) (*openziti.ServiceConfig, error) {
	return s.client.CreateConfig(ctx, cfg)
}

func (s *Service) UpdateConfig(ctx context.Context, cfg openziti.ServiceConfig) (*openziti.ServiceConfig, error) {
	return s.client.UpdateConfig(ctx, cfg)
}

func (s *Service) DeleteConfig(ctx context.Context, id string) error {
	return s.client.DeleteConfig(ctx, id)
}
