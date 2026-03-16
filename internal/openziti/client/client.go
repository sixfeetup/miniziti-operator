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
	"bytes"
	"context"
	"crypto/x509"
	"fmt"
	"strings"
	"sync"

	"github.com/openziti/edge-api/rest_management_api_client"
	configapi "github.com/openziti/edge-api/rest_management_api_client/config"
	identityapi "github.com/openziti/edge-api/rest_management_api_client/identity"
	serviceapi "github.com/openziti/edge-api/rest_management_api_client/service"
	servicepolicyapi "github.com/openziti/edge-api/rest_management_api_client/service_policy"
	"github.com/openziti/edge-api/rest_model"
	"github.com/openziti/edge-api/rest_util"

	"example.com/miniziti-operator/internal/credentials"
)

// Identity models the subset of OpenZiti identity state needed by the operator.
type Identity struct {
	ID             string
	Name           string
	Type           string
	RoleAttributes []string
	CreateOTT      bool
}

// Service models the subset of OpenZiti service state needed by the operator.
type Service struct {
	ID             string
	Name           string
	RoleAttributes []string
	ConfigIDs      []string
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

// ConfigLoader returns the latest management credentials.
type ConfigLoader func(context.Context) (credentials.ManagementConfig, error)

type authenticateFunc func(context.Context, credentials.ManagementConfig) (*rest_management_api_client.ZitiEdgeManagement, error)

// ManagementClient implements the operator's real OpenZiti management transport with lazy session reuse.
type ManagementClient struct {
	mu           sync.Mutex
	loadConfig   ConfigLoader
	authenticate authenticateFunc

	cachedClient *rest_management_api_client.ZitiEdgeManagement
	cachedConfig credentials.ManagementConfig
}

// New constructs a lazy, Secret-backed OpenZiti management client.
func New(loadConfig ConfigLoader) *ManagementClient {
	return &ManagementClient{
		loadConfig:   loadConfig,
		authenticate: authenticateWithUPDB,
	}
}

func authenticateWithUPDB(
	_ context.Context,
	cfg credentials.ManagementConfig,
) (*rest_management_api_client.ZitiEdgeManagement, error) {
	rootCAs, err := rootCAsFromConfig(cfg)
	if err != nil {
		return nil, err
	}
	client, err := rest_util.NewEdgeManagementClientWithUpdb(cfg.Username, cfg.Password, cfg.ControllerURL, rootCAs)
	if err != nil {
		return nil, fmt.Errorf("authenticate to %s: %w", cfg.ControllerURL, err)
	}
	return client, nil
}

func rootCAsFromConfig(cfg credentials.ManagementConfig) (*x509.CertPool, error) {
	if len(cfg.CABundlePEM) == 0 {
		return nil, nil
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(cfg.CABundlePEM) {
		return nil, fmt.Errorf("invalid %q in management credentials", credentials.CABundleKey)
	}
	return pool, nil
}

func (c *ManagementClient) Authenticate(ctx context.Context, cfg credentials.ManagementConfig) error {
	_, err := useAuthenticatedClient(ctx, c, func(context.Context) (credentials.ManagementConfig, error) {
		return cfg, nil
	}, func(*rest_management_api_client.ZitiEdgeManagement) (struct{}, error) {
		return struct{}{}, nil
	})
	return err
}

func (c *ManagementClient) GetIdentity(ctx context.Context, id string) (*Identity, error) {
	return useAuthenticatedClient(ctx, c, c.loadCurrentConfig, func(api *rest_management_api_client.ZitiEdgeManagement) (*Identity, error) {
		resp, err := api.Identity.DetailIdentity(identityapi.NewDetailIdentityParamsWithContext(ctx).WithID(id), nil)
		if err != nil {
			if isStatusCode(err, 404) {
				return nil, nil
			}
			return nil, wrapAPICallError("get identity", err)
		}
		return identityFromEnvelope(resp.Payload), nil
	})
}

func (c *ManagementClient) FindIdentityByName(ctx context.Context, name string) (*Identity, error) {
	return useAuthenticatedClient(ctx, c, c.loadCurrentConfig, func(api *rest_management_api_client.ZitiEdgeManagement) (*Identity, error) {
		resp, err := api.Identity.ListIdentities(identityapi.NewListIdentitiesParamsWithContext(ctx), nil)
		if err != nil {
			return nil, wrapAPICallError("list identities", err)
		}
		for _, item := range resp.Payload.Data {
			identity := identityFromEnvelope(&rest_model.DetailIdentityEnvelope{Data: item})
			if identity != nil && identity.Name == name {
				return identity, nil
			}
		}
		return nil, nil
	})
}

func (c *ManagementClient) CreateIdentity(ctx context.Context, identity Identity) (*Identity, error) {
	return useAuthenticatedClient(ctx, c, c.loadCurrentConfig, func(api *rest_management_api_client.ZitiEdgeManagement) (*Identity, error) {
		resp, err := api.Identity.CreateIdentity(identityapi.NewCreateIdentityParamsWithContext(ctx).WithIdentity(toIdentityCreate(identity)), nil)
		if err != nil {
			return nil, wrapAPICallError("create identity", err)
		}
		return c.getIdentityByIDWithAPI(ctx, api, resp.Payload)
	})
}

func (c *ManagementClient) UpdateIdentity(ctx context.Context, identity Identity) (*Identity, error) {
	updated, err := useAuthenticatedClient(ctx, c, c.loadCurrentConfig, func(api *rest_management_api_client.ZitiEdgeManagement) (*Identity, error) {
		_, err := api.Identity.UpdateIdentity(
			identityapi.NewUpdateIdentityParamsWithContext(ctx).
				WithID(identity.ID).
				WithIdentity(toIdentityUpdate(identity)),
			nil,
		)
		if err != nil {
			if isStatusCode(err, 404) {
				return nil, err
			}
			return nil, wrapAPICallError("update identity", err)
		}
		return c.getIdentityByIDWithAPI(ctx, api, &rest_model.CreateEnvelope{Data: &rest_model.CreateLocation{ID: identity.ID}})
	})
	if err != nil && isStatusCode(err, 404) {
		identity.ID = ""
		return c.CreateIdentity(ctx, identity)
	}
	return updated, err
}

func (c *ManagementClient) DeleteIdentity(ctx context.Context, id string) error {
	_, err := useAuthenticatedClient(ctx, c, c.loadCurrentConfig, func(api *rest_management_api_client.ZitiEdgeManagement) (struct{}, error) {
		_, err := api.Identity.DeleteIdentity(identityapi.NewDeleteIdentityParamsWithContext(ctx).WithID(id), nil)
		if err != nil {
			if isStatusCode(err, 404) {
				return struct{}{}, nil
			}
			return struct{}{}, wrapAPICallError("delete identity", err)
		}
		return struct{}{}, nil
	})
	return err
}

func (c *ManagementClient) GetEnrollmentJWT(ctx context.Context, id string) (string, error) {
	return useAuthenticatedClient(ctx, c, c.loadCurrentConfig, func(api *rest_management_api_client.ZitiEdgeManagement) (string, error) {
		resp, err := api.Identity.DetailIdentity(identityapi.NewDetailIdentityParamsWithContext(ctx).WithID(id), nil)
		if err != nil {
			return "", wrapAPICallError("get identity enrollment jwt", err)
		}
		if resp.Payload == nil || resp.Payload.Data == nil || resp.Payload.Data.Enrollment == nil {
			return "", fmt.Errorf("identity %s has no enrollment data", id)
		}
		if resp.Payload.Data.Enrollment.Ott != nil && strings.TrimSpace(resp.Payload.Data.Enrollment.Ott.JWT) != "" {
			return strings.TrimSpace(resp.Payload.Data.Enrollment.Ott.JWT), nil
		}
		if resp.Payload.Data.Enrollment.Ottca != nil && strings.TrimSpace(resp.Payload.Data.Enrollment.Ottca.JWT) != "" {
			return strings.TrimSpace(resp.Payload.Data.Enrollment.Ottca.JWT), nil
		}
		return "", fmt.Errorf("identity %s has no enrollment jwt", id)
	})
}

func (c *ManagementClient) GetService(ctx context.Context, id string) (*Service, error) {
	return useAuthenticatedClient(ctx, c, c.loadCurrentConfig, func(api *rest_management_api_client.ZitiEdgeManagement) (*Service, error) {
		resp, err := api.Service.DetailService(serviceapi.NewDetailServiceParamsWithContext(ctx).WithID(id), nil)
		if err != nil {
			if isStatusCode(err, 404) {
				return nil, nil
			}
			return nil, wrapAPICallError("get service", err)
		}
		return serviceFromEnvelope(resp.Payload), nil
	})
}

func (c *ManagementClient) FindServiceByName(ctx context.Context, name string) (*Service, error) {
	return useAuthenticatedClient(ctx, c, c.loadCurrentConfig, func(api *rest_management_api_client.ZitiEdgeManagement) (*Service, error) {
		resp, err := api.Service.ListServices(serviceapi.NewListServicesParamsWithContext(ctx), nil)
		if err != nil {
			return nil, wrapAPICallError("list services", err)
		}
		for _, item := range resp.Payload.Data {
			service := serviceFromEnvelope(&rest_model.DetailServiceEnvelope{Data: item})
			if service != nil && service.Name == name {
				return service, nil
			}
		}
		return nil, nil
	})
}

func (c *ManagementClient) CreateService(ctx context.Context, service Service) (*Service, error) {
	return useAuthenticatedClient(ctx, c, c.loadCurrentConfig, func(api *rest_management_api_client.ZitiEdgeManagement) (*Service, error) {
		resp, err := api.Service.CreateService(serviceapi.NewCreateServiceParamsWithContext(ctx).WithService(toServiceCreate(service)), nil)
		if err != nil {
			return nil, wrapAPICallError("create service", err)
		}
		return c.getServiceByIDWithAPI(ctx, api, resp.Payload)
	})
}

func (c *ManagementClient) UpdateService(ctx context.Context, service Service) (*Service, error) {
	updated, err := useAuthenticatedClient(ctx, c, c.loadCurrentConfig, func(api *rest_management_api_client.ZitiEdgeManagement) (*Service, error) {
		_, err := api.Service.UpdateService(
			serviceapi.NewUpdateServiceParamsWithContext(ctx).
				WithID(service.ID).
				WithService(toServiceUpdate(service)),
			nil,
		)
		if err != nil {
			if isStatusCode(err, 404) {
				return nil, err
			}
			return nil, wrapAPICallError("update service", err)
		}
		return c.getServiceByIDWithAPI(ctx, api, &rest_model.CreateEnvelope{Data: &rest_model.CreateLocation{ID: service.ID}})
	})
	if err != nil && isStatusCode(err, 404) {
		service.ID = ""
		return c.CreateService(ctx, service)
	}
	return updated, err
}

func (c *ManagementClient) DeleteService(ctx context.Context, id string) error {
	_, err := useAuthenticatedClient(ctx, c, c.loadCurrentConfig, func(api *rest_management_api_client.ZitiEdgeManagement) (struct{}, error) {
		_, err := api.Service.DeleteService(serviceapi.NewDeleteServiceParamsWithContext(ctx).WithID(id), nil)
		if err != nil {
			if isStatusCode(err, 404) {
				return struct{}{}, nil
			}
			return struct{}{}, wrapAPICallError("delete service", err)
		}
		return struct{}{}, nil
	})
	return err
}

func (c *ManagementClient) GetConfig(ctx context.Context, id string) (*ServiceConfig, error) {
	return useAuthenticatedClient(ctx, c, c.loadCurrentConfig, func(api *rest_management_api_client.ZitiEdgeManagement) (*ServiceConfig, error) {
		resp, err := api.Config.DetailConfig(configapi.NewDetailConfigParamsWithContext(ctx).WithID(id), nil)
		if err != nil {
			if isStatusCode(err, 404) {
				return nil, nil
			}
			return nil, wrapAPICallError("get config", err)
		}
		return configFromEnvelope(resp.Payload), nil
	})
}

func (c *ManagementClient) CreateConfig(ctx context.Context, cfg ServiceConfig) (*ServiceConfig, error) {
	return useAuthenticatedClient(ctx, c, c.loadCurrentConfig, func(api *rest_management_api_client.ZitiEdgeManagement) (*ServiceConfig, error) {
		configTypeID, err := c.resolveConfigTypeIDWithAPI(ctx, api, cfg.Type)
		if err != nil {
			return nil, err
		}
		resp, err := api.Config.CreateConfig(
			configapi.NewCreateConfigParamsWithContext(ctx).WithConfig(toConfigCreate(cfg, configTypeID)),
			nil,
		)
		if err != nil {
			return nil, wrapAPICallError("create config", err)
		}
		return c.getConfigByIDWithAPI(ctx, api, resp.Payload)
	})
}

func (c *ManagementClient) UpdateConfig(ctx context.Context, cfg ServiceConfig) (*ServiceConfig, error) {
	updated, err := useAuthenticatedClient(ctx, c, c.loadCurrentConfig, func(api *rest_management_api_client.ZitiEdgeManagement) (*ServiceConfig, error) {
		_, err := api.Config.UpdateConfig(
			configapi.NewUpdateConfigParamsWithContext(ctx).
				WithID(cfg.ID).
				WithConfig(toConfigUpdate(cfg)),
			nil,
		)
		if err != nil {
			if isStatusCode(err, 404) {
				return nil, err
			}
			return nil, wrapAPICallError("update config", err)
		}
		return c.getConfigByIDWithAPI(ctx, api, &rest_model.CreateEnvelope{Data: &rest_model.CreateLocation{ID: cfg.ID}})
	})
	if err != nil && isStatusCode(err, 404) {
		cfg.ID = ""
		return c.CreateConfig(ctx, cfg)
	}
	return updated, err
}

func (c *ManagementClient) DeleteConfig(ctx context.Context, id string) error {
	_, err := useAuthenticatedClient(ctx, c, c.loadCurrentConfig, func(api *rest_management_api_client.ZitiEdgeManagement) (struct{}, error) {
		_, err := api.Config.DeleteConfig(configapi.NewDeleteConfigParamsWithContext(ctx).WithID(id), nil)
		if err != nil {
			if isStatusCode(err, 404) {
				return struct{}{}, nil
			}
			return struct{}{}, wrapAPICallError("delete config", err)
		}
		return struct{}{}, nil
	})
	return err
}

func (c *ManagementClient) GetAccessPolicy(ctx context.Context, id string) (*AccessPolicy, error) {
	return useAuthenticatedClient(ctx, c, c.loadCurrentConfig, func(api *rest_management_api_client.ZitiEdgeManagement) (*AccessPolicy, error) {
		resp, err := api.ServicePolicy.DetailServicePolicy(
			servicepolicyapi.NewDetailServicePolicyParamsWithContext(ctx).WithID(id),
			nil,
		)
		if err != nil {
			if isStatusCode(err, 404) {
				return nil, nil
			}
			return nil, wrapAPICallError("get access policy", err)
		}
		return accessPolicyFromEnvelope(resp.Payload), nil
	})
}

func (c *ManagementClient) FindAccessPolicyByName(ctx context.Context, name string) (*AccessPolicy, error) {
	return useAuthenticatedClient(ctx, c, c.loadCurrentConfig, func(api *rest_management_api_client.ZitiEdgeManagement) (*AccessPolicy, error) {
		resp, err := api.ServicePolicy.ListServicePolicies(servicepolicyapi.NewListServicePoliciesParamsWithContext(ctx), nil)
		if err != nil {
			return nil, wrapAPICallError("list access policies", err)
		}
		for _, item := range resp.Payload.Data {
			policy := accessPolicyFromEnvelope(&rest_model.DetailServicePolicyEnvelop{Data: item})
			if policy != nil && policy.Name == name {
				return policy, nil
			}
		}
		return nil, nil
	})
}

func (c *ManagementClient) CreateAccessPolicy(ctx context.Context, policy AccessPolicy) (*AccessPolicy, error) {
	return useAuthenticatedClient(ctx, c, c.loadCurrentConfig, func(api *rest_management_api_client.ZitiEdgeManagement) (*AccessPolicy, error) {
		resp, err := api.ServicePolicy.CreateServicePolicy(
			servicepolicyapi.NewCreateServicePolicyParamsWithContext(ctx).WithPolicy(toAccessPolicyCreate(policy)),
			nil,
		)
		if err != nil {
			return nil, wrapAPICallError("create access policy", err)
		}
		return c.getAccessPolicyByIDWithAPI(ctx, api, resp.Payload)
	})
}

func (c *ManagementClient) UpdateAccessPolicy(ctx context.Context, policy AccessPolicy) (*AccessPolicy, error) {
	updated, err := useAuthenticatedClient(ctx, c, c.loadCurrentConfig, func(api *rest_management_api_client.ZitiEdgeManagement) (*AccessPolicy, error) {
		_, err := api.ServicePolicy.UpdateServicePolicy(
			servicepolicyapi.NewUpdateServicePolicyParamsWithContext(ctx).
				WithID(policy.ID).
				WithPolicy(toAccessPolicyUpdate(policy)),
			nil,
		)
		if err != nil {
			if isStatusCode(err, 404) {
				return nil, err
			}
			return nil, wrapAPICallError("update access policy", err)
		}
		return c.getAccessPolicyByIDWithAPI(ctx, api, &rest_model.CreateEnvelope{Data: &rest_model.CreateLocation{ID: policy.ID}})
	})
	if err != nil && isStatusCode(err, 404) {
		policy.ID = ""
		return c.CreateAccessPolicy(ctx, policy)
	}
	return updated, err
}

func (c *ManagementClient) DeleteAccessPolicy(ctx context.Context, id string) error {
	_, err := useAuthenticatedClient(ctx, c, c.loadCurrentConfig, func(api *rest_management_api_client.ZitiEdgeManagement) (struct{}, error) {
		_, err := api.ServicePolicy.DeleteServicePolicy(
			servicepolicyapi.NewDeleteServicePolicyParamsWithContext(ctx).WithID(id),
			nil,
		)
		if err != nil {
			if isStatusCode(err, 404) {
				return struct{}{}, nil
			}
			return struct{}{}, wrapAPICallError("delete access policy", err)
		}
		return struct{}{}, nil
	})
	return err
}

func (c *ManagementClient) loadCurrentConfig(ctx context.Context) (credentials.ManagementConfig, error) {
	if c.loadConfig == nil {
		c.mu.Lock()
		defer c.mu.Unlock()
		if c.cachedConfig.ControllerURL == "" {
			return credentials.ManagementConfig{}, fmt.Errorf("openziti credentials loader is not configured")
		}
		return c.cachedConfig, nil
	}
	return c.loadConfig(ctx)
}

func (c *ManagementClient) getOrAuthenticate(
	ctx context.Context,
	load ConfigLoader,
	force bool,
) (*rest_management_api_client.ZitiEdgeManagement, error) {
	cfg, err := load(ctx)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	if !force && c.cachedClient != nil && sameManagementConfig(c.cachedConfig, cfg) {
		client := c.cachedClient
		c.mu.Unlock()
		return client, nil
	}
	c.mu.Unlock()

	api, err := c.authenticate(ctx, cfg)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	c.cachedClient = api
	c.cachedConfig = cfg
	c.mu.Unlock()

	return api, nil
}

func (c *ManagementClient) invalidate() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cachedClient = nil
}

func useAuthenticatedClient[T any](
	ctx context.Context,
	c *ManagementClient,
	load ConfigLoader,
	fn func(*rest_management_api_client.ZitiEdgeManagement) (T, error),
) (T, error) {
	var zero T

	api, err := c.getOrAuthenticate(ctx, load, false)
	if err != nil {
		return zero, err
	}

	result, err := fn(api)
	if err == nil || !shouldReauthenticate(err) {
		return result, err
	}

	c.invalidate()
	api, authErr := c.getOrAuthenticate(ctx, load, true)
	if authErr != nil {
		return zero, authErr
	}
	return fn(api)
}

func shouldReauthenticate(err error) bool {
	return isStatusCode(err, 401) || isStatusCode(err, 403)
}

func sameManagementConfig(left, right credentials.ManagementConfig) bool {
	return left.ControllerURL == right.ControllerURL &&
		left.Username == right.Username &&
		left.Password == right.Password &&
		bytes.Equal(left.CABundlePEM, right.CABundlePEM)
}

func isStatusCode(err error, code int) bool {
	type coder interface {
		Code() int
	}
	if err == nil {
		return false
	}
	response, ok := err.(coder)
	return ok && response.Code() == code
}

func wrapAPICallError(action string, err error) error {
	return fmt.Errorf("%s: %w", action, err)
}

func identityFromEnvelope(envelope *rest_model.DetailIdentityEnvelope) *Identity {
	if envelope == nil {
		return nil
	}
	detail := envelope.Data
	if detail == nil || detail.ID == nil || detail.Name == nil || detail.Type == nil || strings.TrimSpace(detail.Type.Name) == "" {
		return nil
	}
	roleAttributes := []string(nil)
	if detail.RoleAttributes != nil {
		roleAttributes = append(roleAttributes, []string(*detail.RoleAttributes)...)
	}
	return &Identity{
		ID:             *detail.ID,
		Name:           *detail.Name,
		Type:           detail.Type.Name,
		RoleAttributes: roleAttributes,
	}
}

func serviceFromEnvelope(envelope *rest_model.DetailServiceEnvelope) *Service {
	if envelope == nil {
		return nil
	}
	detail := envelope.Data
	if detail == nil || detail.ID == nil || detail.Name == nil {
		return nil
	}
	roleAttributes := []string(nil)
	if detail.RoleAttributes != nil {
		roleAttributes = append(roleAttributes, []string(*detail.RoleAttributes)...)
	}
	return &Service{
		ID:             *detail.ID,
		Name:           *detail.Name,
		RoleAttributes: roleAttributes,
		ConfigIDs:      append([]string(nil), detail.Configs...),
	}
}

func configFromEnvelope(envelope *rest_model.DetailConfigEnvelope) *ServiceConfig {
	if envelope == nil {
		return nil
	}
	detail := envelope.Data
	if detail == nil || detail.ID == nil || detail.Name == nil || detail.ConfigType == nil || strings.TrimSpace(detail.ConfigType.Name) == "" {
		return nil
	}
	payload := map[string]any{}
	if typed, ok := detail.Data.(map[string]any); ok {
		for key, value := range typed {
			payload[key] = value
		}
	}
	return &ServiceConfig{
		ID:      *detail.ID,
		Name:    *detail.Name,
		Type:    detail.ConfigType.Name,
		Payload: payload,
	}
}

func accessPolicyFromEnvelope(envelope *rest_model.DetailServicePolicyEnvelop) *AccessPolicy {
	if envelope == nil {
		return nil
	}
	detail := envelope.Data
	if detail == nil || detail.ID == nil || detail.Name == nil || detail.Type == nil || detail.Semantic == nil {
		return nil
	}
	return &AccessPolicy{
		ID:               *detail.ID,
		Name:             *detail.Name,
		Type:             string(*detail.Type),
		IdentityRoles:    append([]string(nil), []string(detail.IdentityRoles)...),
		ServiceRoles:     append([]string(nil), []string(detail.ServiceRoles)...),
		IdentityRolesRaw: append([]string(nil), []string(detail.IdentityRoles)...),
		ServiceRolesRaw:  append([]string(nil), []string(detail.ServiceRoles)...),
		Semantic:         string(*detail.Semantic),
	}
}

func (c *ManagementClient) getIdentityByIDWithAPI(
	ctx context.Context,
	api *rest_management_api_client.ZitiEdgeManagement,
	createEnvelope *rest_model.CreateEnvelope,
) (*Identity, error) {
	id, err := extractCreatedID(createEnvelope)
	if err != nil {
		return nil, err
	}
	resp, err := api.Identity.DetailIdentity(identityapi.NewDetailIdentityParamsWithContext(ctx).WithID(id), nil)
	if err != nil {
		return nil, wrapAPICallError("get identity", err)
	}
	return identityFromEnvelope(resp.Payload), nil
}

func (c *ManagementClient) getServiceByIDWithAPI(
	ctx context.Context,
	api *rest_management_api_client.ZitiEdgeManagement,
	createEnvelope *rest_model.CreateEnvelope,
) (*Service, error) {
	id, err := extractCreatedID(createEnvelope)
	if err != nil {
		return nil, err
	}
	resp, err := api.Service.DetailService(serviceapi.NewDetailServiceParamsWithContext(ctx).WithID(id), nil)
	if err != nil {
		return nil, wrapAPICallError("get service", err)
	}
	return serviceFromEnvelope(resp.Payload), nil
}

func (c *ManagementClient) getConfigByIDWithAPI(
	ctx context.Context,
	api *rest_management_api_client.ZitiEdgeManagement,
	createEnvelope *rest_model.CreateEnvelope,
) (*ServiceConfig, error) {
	id, err := extractCreatedID(createEnvelope)
	if err != nil {
		return nil, err
	}
	resp, err := api.Config.DetailConfig(configapi.NewDetailConfigParamsWithContext(ctx).WithID(id), nil)
	if err != nil {
		return nil, wrapAPICallError("get config", err)
	}
	return configFromEnvelope(resp.Payload), nil
}

func (c *ManagementClient) getAccessPolicyByIDWithAPI(
	ctx context.Context,
	api *rest_management_api_client.ZitiEdgeManagement,
	createEnvelope *rest_model.CreateEnvelope,
) (*AccessPolicy, error) {
	id, err := extractCreatedID(createEnvelope)
	if err != nil {
		return nil, err
	}
	resp, err := api.ServicePolicy.DetailServicePolicy(
		servicepolicyapi.NewDetailServicePolicyParamsWithContext(ctx).WithID(id),
		nil,
	)
	if err != nil {
		return nil, wrapAPICallError("get access policy", err)
	}
	return accessPolicyFromEnvelope(resp.Payload), nil
}

func extractCreatedID(createEnvelope *rest_model.CreateEnvelope) (string, error) {
	if createEnvelope == nil || createEnvelope.Data == nil || strings.TrimSpace(createEnvelope.Data.ID) == "" {
		return "", fmt.Errorf("create response did not include an object id")
	}
	return strings.TrimSpace(createEnvelope.Data.ID), nil
}

func (c *ManagementClient) resolveConfigTypeIDWithAPI(
	ctx context.Context,
	api *rest_management_api_client.ZitiEdgeManagement,
	configTypeName string,
) (string, error) {
	name := strings.TrimSpace(configTypeName)
	if name == "" {
		return "", fmt.Errorf("config type name must not be empty")
	}

	var offset int64
	limit := int64(100)

	for {
		resp, err := api.Config.ListConfigTypes(
			configapi.NewListConfigTypesParamsWithContext(ctx).WithLimit(&limit).WithOffset(&offset),
			nil,
		)
		if err != nil {
			return "", wrapAPICallError("list config types", err)
		}
		if resp.Payload == nil {
			break
		}

		count := 0
		for _, item := range resp.Payload.Data {
			if item == nil || item.ID == nil || item.Name == nil {
				continue
			}
			count++
			if strings.TrimSpace(*item.Name) == name {
				return strings.TrimSpace(*item.ID), nil
			}
		}
		if count == 0 || count < int(limit) {
			break
		}
		offset += int64(count)
	}

	return "", fmt.Errorf("config type %q not found", name)
}

func toIdentityCreate(identity Identity) *rest_model.IdentityCreate {
	roleAttributes := rest_model.Attributes(append([]string(nil), identity.RoleAttributes...))
	identityType := rest_model.IdentityType(identity.Type)
	isAdmin := false
	created := &rest_model.IdentityCreate{
		Name:           &identity.Name,
		Type:           &identityType,
		IsAdmin:        &isAdmin,
		RoleAttributes: &roleAttributes,
	}
	if identity.CreateOTT {
		created.Enrollment = &rest_model.IdentityCreateEnrollment{Ott: true}
	}
	return created
}

func toIdentityUpdate(identity Identity) *rest_model.IdentityUpdate {
	roleAttributes := rest_model.Attributes(append([]string(nil), identity.RoleAttributes...))
	identityType := rest_model.IdentityType(identity.Type)
	isAdmin := false
	return &rest_model.IdentityUpdate{
		Name:           &identity.Name,
		Type:           &identityType,
		IsAdmin:        &isAdmin,
		RoleAttributes: &roleAttributes,
	}
}

func toServiceCreate(service Service) *rest_model.ServiceCreate {
	encryptionRequired := true
	return &rest_model.ServiceCreate{
		Name:               &service.Name,
		EncryptionRequired: &encryptionRequired,
		RoleAttributes:     append([]string(nil), service.RoleAttributes...),
		Configs:            append([]string(nil), service.ConfigIDs...),
	}
}

func toServiceUpdate(service Service) *rest_model.ServiceUpdate {
	return &rest_model.ServiceUpdate{
		Name:           &service.Name,
		RoleAttributes: append([]string(nil), service.RoleAttributes...),
		Configs:        append([]string(nil), service.ConfigIDs...),
	}
}

func toConfigCreate(cfg ServiceConfig, configTypeID string) *rest_model.ConfigCreate {
	return &rest_model.ConfigCreate{
		Name:         &cfg.Name,
		ConfigTypeID: &configTypeID,
		Data:         clonePayload(cfg.Payload),
	}
}

func toConfigUpdate(cfg ServiceConfig) *rest_model.ConfigUpdate {
	return &rest_model.ConfigUpdate{
		Name: &cfg.Name,
		Data: clonePayload(cfg.Payload),
	}
}

func toAccessPolicyCreate(policy AccessPolicy) *rest_model.ServicePolicyCreate {
	policyType := rest_model.DialBind(policy.Type)
	semantic := rest_model.Semantic(policy.Semantic)
	return &rest_model.ServicePolicyCreate{
		Name:              &policy.Name,
		Type:              &policyType,
		Semantic:          &semantic,
		IdentityRoles:     rest_model.Roles(append([]string(nil), policy.IdentityRolesRaw...)),
		ServiceRoles:      rest_model.Roles(append([]string(nil), policy.ServiceRolesRaw...)),
		PostureCheckRoles: rest_model.Roles{},
	}
}

func toAccessPolicyUpdate(policy AccessPolicy) *rest_model.ServicePolicyUpdate {
	policyType := rest_model.DialBind(policy.Type)
	semantic := rest_model.Semantic(policy.Semantic)
	return &rest_model.ServicePolicyUpdate{
		Name:              &policy.Name,
		Type:              &policyType,
		Semantic:          &semantic,
		IdentityRoles:     rest_model.Roles(append([]string(nil), policy.IdentityRolesRaw...)),
		ServiceRoles:      rest_model.Roles(append([]string(nil), policy.ServiceRolesRaw...)),
		PostureCheckRoles: rest_model.Roles{},
	}
}

func clonePayload(payload map[string]any) map[string]any {
	if payload == nil {
		return nil
	}
	cloned := make(map[string]any, len(payload))
	for key, value := range payload {
		switch typed := value.(type) {
		case []string:
			cloned[key] = append([]string(nil), typed...)
		case []map[string]int32:
			items := make([]map[string]int32, 0, len(typed))
			for _, item := range typed {
				copyItem := make(map[string]int32, len(item))
				for itemKey, itemValue := range item {
					copyItem[itemKey] = itemValue
				}
				items = append(items, copyItem)
			}
			cloned[key] = items
		default:
			cloned[key] = value
		}
	}
	return cloned
}
