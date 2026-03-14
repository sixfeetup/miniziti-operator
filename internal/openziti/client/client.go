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

package client

import (
	"context"
	"fmt"

	"example.com/miniziti-operator/internal/credentials"
)

// Identity models the subset of OpenZiti identity state needed by the operator.
type Identity struct {
	ID             string
	Name           string
	Type           string
	RoleAttributes []string
}

// Service models the subset of OpenZiti service state needed by the operator.
type Service struct {
	ID             string
	Name           string
	RoleAttributes []string
}

// ServiceConfig models an OpenZiti service config artifact.
type ServiceConfig struct {
	ID      string
	Name    string
	Type    string
	Payload map[string]any
}

// AccessPolicy models the subset of OpenZiti service-policy state needed by the operator.
type AccessPolicy struct {
	ID               string
	Name             string
	Type             string
	IdentityRoles    []string
	ServiceRoles     []string
	IdentityRolesRaw []string
	ServiceRolesRaw  []string
	Semantic         string
}

// Client describes the management operations the operator needs from OpenZiti.
type Client interface {
	Authenticate(context.Context, credentials.ManagementConfig) error

	GetIdentity(context.Context, string) (*Identity, error)
	FindIdentityByName(context.Context, string) (*Identity, error)
	CreateIdentity(context.Context, Identity) (*Identity, error)
	UpdateIdentity(context.Context, Identity) (*Identity, error)
	DeleteIdentity(context.Context, string) error
	GetEnrollmentJWT(context.Context, string) (string, error)

	GetService(context.Context, string) (*Service, error)
	FindServiceByName(context.Context, string) (*Service, error)
	CreateService(context.Context, Service) (*Service, error)
	UpdateService(context.Context, Service) (*Service, error)
	DeleteService(context.Context, string) error

	GetConfig(context.Context, string) (*ServiceConfig, error)
	CreateConfig(context.Context, ServiceConfig) (*ServiceConfig, error)
	UpdateConfig(context.Context, ServiceConfig) (*ServiceConfig, error)
	DeleteConfig(context.Context, string) error

	GetAccessPolicy(context.Context, string) (*AccessPolicy, error)
	FindAccessPolicyByName(context.Context, string) (*AccessPolicy, error)
	CreateAccessPolicy(context.Context, AccessPolicy) (*AccessPolicy, error)
	UpdateAccessPolicy(context.Context, AccessPolicy) (*AccessPolicy, error)
	DeleteAccessPolicy(context.Context, string) error
}

// ErrNotImplemented is returned by the transport wrapper until concrete API calls are wired in.
var ErrNotImplemented = fmt.Errorf("openziti client transport not implemented")

// ManagementClient is the placeholder transport-backed implementation used by production wiring.
type ManagementClient struct{}

// New returns a transport-backed client placeholder that will later wrap the generated edge API client.
func New() *ManagementClient {
	return &ManagementClient{}
}

func (c *ManagementClient) Authenticate(context.Context, credentials.ManagementConfig) error {
	return ErrNotImplemented
}

func (c *ManagementClient) GetIdentity(context.Context, string) (*Identity, error) {
	return nil, ErrNotImplemented
}

func (c *ManagementClient) FindIdentityByName(context.Context, string) (*Identity, error) {
	return nil, ErrNotImplemented
}

func (c *ManagementClient) CreateIdentity(context.Context, Identity) (*Identity, error) {
	return nil, ErrNotImplemented
}

func (c *ManagementClient) UpdateIdentity(context.Context, Identity) (*Identity, error) {
	return nil, ErrNotImplemented
}

func (c *ManagementClient) DeleteIdentity(context.Context, string) error {
	return ErrNotImplemented
}

func (c *ManagementClient) GetEnrollmentJWT(context.Context, string) (string, error) {
	return "", ErrNotImplemented
}

func (c *ManagementClient) GetService(context.Context, string) (*Service, error) {
	return nil, ErrNotImplemented
}

func (c *ManagementClient) FindServiceByName(context.Context, string) (*Service, error) {
	return nil, ErrNotImplemented
}

func (c *ManagementClient) CreateService(context.Context, Service) (*Service, error) {
	return nil, ErrNotImplemented
}

func (c *ManagementClient) UpdateService(context.Context, Service) (*Service, error) {
	return nil, ErrNotImplemented
}

func (c *ManagementClient) DeleteService(context.Context, string) error {
	return ErrNotImplemented
}

func (c *ManagementClient) GetConfig(context.Context, string) (*ServiceConfig, error) {
	return nil, ErrNotImplemented
}

func (c *ManagementClient) CreateConfig(context.Context, ServiceConfig) (*ServiceConfig, error) {
	return nil, ErrNotImplemented
}

func (c *ManagementClient) UpdateConfig(context.Context, ServiceConfig) (*ServiceConfig, error) {
	return nil, ErrNotImplemented
}

func (c *ManagementClient) DeleteConfig(context.Context, string) error {
	return ErrNotImplemented
}

func (c *ManagementClient) GetAccessPolicy(context.Context, string) (*AccessPolicy, error) {
	return nil, ErrNotImplemented
}

func (c *ManagementClient) FindAccessPolicyByName(context.Context, string) (*AccessPolicy, error) {
	return nil, ErrNotImplemented
}

func (c *ManagementClient) CreateAccessPolicy(context.Context, AccessPolicy) (*AccessPolicy, error) {
	return nil, ErrNotImplemented
}

func (c *ManagementClient) UpdateAccessPolicy(context.Context, AccessPolicy) (*AccessPolicy, error) {
	return nil, ErrNotImplemented
}

func (c *ManagementClient) DeleteAccessPolicy(context.Context, string) error {
	return ErrNotImplemented
}
