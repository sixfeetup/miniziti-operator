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

	"example.com/miniziti-operator/internal/credentials"
)

// FakeClient provides test-controlled behavior for reconcile and adapter tests.
type FakeClient struct {
	AuthenticateFunc           func(context.Context, credentials.ManagementConfig) error
	GetIdentityFunc            func(context.Context, string) (*Identity, error)
	FindIdentityByNameFunc     func(context.Context, string) (*Identity, error)
	CreateIdentityFunc         func(context.Context, Identity) (*Identity, error)
	UpdateIdentityFunc         func(context.Context, Identity) (*Identity, error)
	DeleteIdentityFunc         func(context.Context, string) error
	GetEnrollmentJWTFunc       func(context.Context, string) (string, error)
	GetServiceFunc             func(context.Context, string) (*Service, error)
	FindServiceByNameFunc      func(context.Context, string) (*Service, error)
	CreateServiceFunc          func(context.Context, Service) (*Service, error)
	UpdateServiceFunc          func(context.Context, Service) (*Service, error)
	DeleteServiceFunc          func(context.Context, string) error
	GetConfigFunc              func(context.Context, string) (*ServiceConfig, error)
	CreateConfigFunc           func(context.Context, ServiceConfig) (*ServiceConfig, error)
	UpdateConfigFunc           func(context.Context, ServiceConfig) (*ServiceConfig, error)
	DeleteConfigFunc           func(context.Context, string) error
	GetAccessPolicyFunc        func(context.Context, string) (*AccessPolicy, error)
	FindAccessPolicyByNameFunc func(context.Context, string) (*AccessPolicy, error)
	CreateAccessPolicyFunc     func(context.Context, AccessPolicy) (*AccessPolicy, error)
	UpdateAccessPolicyFunc     func(context.Context, AccessPolicy) (*AccessPolicy, error)
	DeleteAccessPolicyFunc     func(context.Context, string) error
}

func (f *FakeClient) Authenticate(ctx context.Context, cfg credentials.ManagementConfig) error {
	if f.AuthenticateFunc != nil {
		return f.AuthenticateFunc(ctx, cfg)
	}
	return nil
}

func (f *FakeClient) GetIdentity(ctx context.Context, id string) (*Identity, error) {
	if f.GetIdentityFunc != nil {
		return f.GetIdentityFunc(ctx, id)
	}
	return nil, nil
}

func (f *FakeClient) FindIdentityByName(ctx context.Context, name string) (*Identity, error) {
	if f.FindIdentityByNameFunc != nil {
		return f.FindIdentityByNameFunc(ctx, name)
	}
	return nil, nil
}

func (f *FakeClient) CreateIdentity(ctx context.Context, identity Identity) (*Identity, error) {
	if f.CreateIdentityFunc != nil {
		return f.CreateIdentityFunc(ctx, identity)
	}
	return &identity, nil
}

func (f *FakeClient) UpdateIdentity(ctx context.Context, identity Identity) (*Identity, error) {
	if f.UpdateIdentityFunc != nil {
		return f.UpdateIdentityFunc(ctx, identity)
	}
	return &identity, nil
}

func (f *FakeClient) DeleteIdentity(ctx context.Context, id string) error {
	if f.DeleteIdentityFunc != nil {
		return f.DeleteIdentityFunc(ctx, id)
	}
	return nil
}

func (f *FakeClient) GetEnrollmentJWT(ctx context.Context, id string) (string, error) {
	if f.GetEnrollmentJWTFunc != nil {
		return f.GetEnrollmentJWTFunc(ctx, id)
	}
	return "", nil
}

func (f *FakeClient) GetService(ctx context.Context, id string) (*Service, error) {
	if f.GetServiceFunc != nil {
		return f.GetServiceFunc(ctx, id)
	}
	return nil, nil
}

func (f *FakeClient) FindServiceByName(ctx context.Context, name string) (*Service, error) {
	if f.FindServiceByNameFunc != nil {
		return f.FindServiceByNameFunc(ctx, name)
	}
	return nil, nil
}

func (f *FakeClient) CreateService(ctx context.Context, service Service) (*Service, error) {
	if f.CreateServiceFunc != nil {
		return f.CreateServiceFunc(ctx, service)
	}
	return &service, nil
}

func (f *FakeClient) UpdateService(ctx context.Context, service Service) (*Service, error) {
	if f.UpdateServiceFunc != nil {
		return f.UpdateServiceFunc(ctx, service)
	}
	return &service, nil
}

func (f *FakeClient) DeleteService(ctx context.Context, id string) error {
	if f.DeleteServiceFunc != nil {
		return f.DeleteServiceFunc(ctx, id)
	}
	return nil
}

func (f *FakeClient) GetConfig(ctx context.Context, id string) (*ServiceConfig, error) {
	if f.GetConfigFunc != nil {
		return f.GetConfigFunc(ctx, id)
	}
	return nil, nil
}

func (f *FakeClient) CreateConfig(ctx context.Context, cfg ServiceConfig) (*ServiceConfig, error) {
	if f.CreateConfigFunc != nil {
		return f.CreateConfigFunc(ctx, cfg)
	}
	return &cfg, nil
}

func (f *FakeClient) UpdateConfig(ctx context.Context, cfg ServiceConfig) (*ServiceConfig, error) {
	if f.UpdateConfigFunc != nil {
		return f.UpdateConfigFunc(ctx, cfg)
	}
	return &cfg, nil
}

func (f *FakeClient) DeleteConfig(ctx context.Context, id string) error {
	if f.DeleteConfigFunc != nil {
		return f.DeleteConfigFunc(ctx, id)
	}
	return nil
}

func (f *FakeClient) GetAccessPolicy(ctx context.Context, id string) (*AccessPolicy, error) {
	if f.GetAccessPolicyFunc != nil {
		return f.GetAccessPolicyFunc(ctx, id)
	}
	return nil, nil
}

func (f *FakeClient) FindAccessPolicyByName(ctx context.Context, name string) (*AccessPolicy, error) {
	if f.FindAccessPolicyByNameFunc != nil {
		return f.FindAccessPolicyByNameFunc(ctx, name)
	}
	return nil, nil
}

func (f *FakeClient) CreateAccessPolicy(ctx context.Context, policy AccessPolicy) (*AccessPolicy, error) {
	if f.CreateAccessPolicyFunc != nil {
		return f.CreateAccessPolicyFunc(ctx, policy)
	}
	return &policy, nil
}

func (f *FakeClient) UpdateAccessPolicy(ctx context.Context, policy AccessPolicy) (*AccessPolicy, error) {
	if f.UpdateAccessPolicyFunc != nil {
		return f.UpdateAccessPolicyFunc(ctx, policy)
	}
	return &policy, nil
}

func (f *FakeClient) DeleteAccessPolicy(ctx context.Context, id string) error {
	if f.DeleteAccessPolicyFunc != nil {
		return f.DeleteAccessPolicyFunc(ctx, id)
	}
	return nil
}
