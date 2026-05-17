package client

import (
	"context"
	"strings"

	"github.com/openziti/edge-api/rest_management_api_client"
	serpapi "github.com/openziti/edge-api/rest_management_api_client/service_edge_router_policy"
	"github.com/openziti/edge-api/rest_model"
)

func (c *ManagementClient) GetServiceEdgeRouterPolicy(ctx context.Context, id string) (*ServiceEdgeRouterPolicy, error) {
	return useAuthenticatedClient(ctx, c, c.loadCurrentConfig, func(api *rest_management_api_client.ZitiEdgeManagement) (*ServiceEdgeRouterPolicy, error) {
		resp, err := api.ServiceEdgeRouterPolicy.DetailServiceEdgeRouterPolicy(
			serpapi.NewDetailServiceEdgeRouterPolicyParamsWithContext(ctx).WithID(id),
			nil,
		)
		if err != nil {
			if isStatusCode(err, 404) {
				return nil, nil
			}
			return nil, wrapAPICallError("get service edge router policy", err)
		}
		return serviceEdgeRouterPolicyFromEnvelope(resp.Payload), nil
	})
}

func (c *ManagementClient) FindServiceEdgeRouterPolicyByName(ctx context.Context, name string) (*ServiceEdgeRouterPolicy, error) {
	return useAuthenticatedClient(ctx, c, c.loadCurrentConfig, func(api *rest_management_api_client.ZitiEdgeManagement) (*ServiceEdgeRouterPolicy, error) {
		var offset int64
		limit := int64(100)

		for {
			resp, err := api.ServiceEdgeRouterPolicy.ListServiceEdgeRouterPolicies(
				serpapi.NewListServiceEdgeRouterPoliciesParamsWithContext(ctx).WithLimit(&limit).WithOffset(&offset),
				nil,
			)
			if err != nil {
				return nil, wrapAPICallError("list service edge router policies", err)
			}
			if resp.Payload == nil {
				return nil, nil
			}

			count := 0
			for _, item := range resp.Payload.Data {
				count++
				policy := serviceEdgeRouterPolicyFromDetail(item)
				if policy != nil && policy.Name == name {
					return policy, nil
				}
			}
			if count == 0 || count < int(limit) {
				return nil, nil
			}
			offset += int64(count)
		}
	})
}

func (c *ManagementClient) CreateServiceEdgeRouterPolicy(ctx context.Context, policy ServiceEdgeRouterPolicy) (*ServiceEdgeRouterPolicy, error) {
	return useAuthenticatedClient(ctx, c, c.loadCurrentConfig, func(api *rest_management_api_client.ZitiEdgeManagement) (*ServiceEdgeRouterPolicy, error) {
		resp, err := api.ServiceEdgeRouterPolicy.CreateServiceEdgeRouterPolicy(
			serpapi.NewCreateServiceEdgeRouterPolicyParamsWithContext(ctx).WithPolicy(toServiceEdgeRouterPolicyCreate(policy)),
			nil,
		)
		if err != nil {
			return nil, wrapAPICallError("create service edge router policy", err)
		}
		return c.getServiceEdgeRouterPolicyByIDWithAPI(ctx, api, resp.Payload)
	})
}

func (c *ManagementClient) UpdateServiceEdgeRouterPolicy(ctx context.Context, policy ServiceEdgeRouterPolicy) (*ServiceEdgeRouterPolicy, error) {
	updated, err := useAuthenticatedClient(ctx, c, c.loadCurrentConfig, func(api *rest_management_api_client.ZitiEdgeManagement) (*ServiceEdgeRouterPolicy, error) {
		_, err := api.ServiceEdgeRouterPolicy.UpdateServiceEdgeRouterPolicy(
			serpapi.NewUpdateServiceEdgeRouterPolicyParamsWithContext(ctx).
				WithID(policy.ID).
				WithPolicy(toServiceEdgeRouterPolicyUpdate(policy)),
			nil,
		)
		if err != nil {
			if isStatusCode(err, 404) {
				return nil, err
			}
			return nil, wrapAPICallError("update service edge router policy", err)
		}
		return c.getServiceEdgeRouterPolicyByIDWithAPI(ctx, api, &rest_model.CreateEnvelope{Data: &rest_model.CreateLocation{ID: policy.ID}})
	})
	if err != nil && isStatusCode(err, 404) {
		policy.ID = ""
		return c.CreateServiceEdgeRouterPolicy(ctx, policy)
	}
	return updated, err
}

func (c *ManagementClient) DeleteServiceEdgeRouterPolicy(ctx context.Context, id string) error {
	_, err := useAuthenticatedClient(ctx, c, c.loadCurrentConfig, func(api *rest_management_api_client.ZitiEdgeManagement) (struct{}, error) {
		_, err := api.ServiceEdgeRouterPolicy.DeleteServiceEdgeRouterPolicy(
			serpapi.NewDeleteServiceEdgeRouterPolicyParamsWithContext(ctx).WithID(id),
			nil,
		)
		if err != nil {
			if isStatusCode(err, 404) {
				return struct{}{}, nil
			}
			return struct{}{}, wrapAPICallError("delete service edge router policy", err)
		}
		return struct{}{}, nil
	})
	return err
}

func (c *ManagementClient) getServiceEdgeRouterPolicyByIDWithAPI(
	ctx context.Context,
	api *rest_management_api_client.ZitiEdgeManagement,
	createEnvelope *rest_model.CreateEnvelope,
) (*ServiceEdgeRouterPolicy, error) {
	id, err := extractCreatedID(createEnvelope)
	if err != nil {
		return nil, err
	}
	resp, err := api.ServiceEdgeRouterPolicy.DetailServiceEdgeRouterPolicy(
		serpapi.NewDetailServiceEdgeRouterPolicyParamsWithContext(ctx).WithID(id),
		nil,
	)
	if err != nil {
		return nil, wrapAPICallError("get service edge router policy", err)
	}
	return serviceEdgeRouterPolicyFromEnvelope(resp.Payload), nil
}

func serviceEdgeRouterPolicyFromEnvelope(envelope *rest_model.DetailServiceEdgePolicyEnvelope) *ServiceEdgeRouterPolicy {
	if envelope == nil {
		return nil
	}
	return serviceEdgeRouterPolicyFromDetail(envelope.Data)
}

func serviceEdgeRouterPolicyFromDetail(detail *rest_model.ServiceEdgeRouterPolicyDetail) *ServiceEdgeRouterPolicy {
	if detail == nil || detail.ID == nil || detail.Name == nil || detail.Semantic == nil {
		return nil
	}
	return &ServiceEdgeRouterPolicy{
		ID:              strings.TrimSpace(*detail.ID),
		Name:            strings.TrimSpace(*detail.Name),
		EdgeRouterRoles: append([]string(nil), []string(detail.EdgeRouterRoles)...),
		ServiceRoles:    append([]string(nil), []string(detail.ServiceRoles)...),
		Semantic:        string(*detail.Semantic),
	}
}

func toServiceEdgeRouterPolicyCreate(policy ServiceEdgeRouterPolicy) *rest_model.ServiceEdgeRouterPolicyCreate {
	semantic := rest_model.Semantic(policy.Semantic)
	return &rest_model.ServiceEdgeRouterPolicyCreate{
		Name:            &policy.Name,
		Semantic:        &semantic,
		EdgeRouterRoles: rest_model.Roles(append([]string(nil), policy.EdgeRouterRoles...)),
		ServiceRoles:    rest_model.Roles(append([]string(nil), policy.ServiceRoles...)),
	}
}

func toServiceEdgeRouterPolicyUpdate(policy ServiceEdgeRouterPolicy) *rest_model.ServiceEdgeRouterPolicyUpdate {
	semantic := rest_model.Semantic(policy.Semantic)
	return &rest_model.ServiceEdgeRouterPolicyUpdate{
		Name:            &policy.Name,
		Semantic:        &semantic,
		EdgeRouterRoles: rest_model.Roles(append([]string(nil), policy.EdgeRouterRoles...)),
		ServiceRoles:    rest_model.Roles(append([]string(nil), policy.ServiceRoles...)),
	}
}
