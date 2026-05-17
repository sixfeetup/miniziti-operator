package client

import (
	"context"
	"strings"

	"github.com/openziti/edge-api/rest_management_api_client"
	edgerouterapi "github.com/openziti/edge-api/rest_management_api_client/edge_router"
	"github.com/openziti/edge-api/rest_model"
)

func (c *ManagementClient) FindEdgeRouterByName(ctx context.Context, name string) (*EdgeRouter, error) {
	return useAuthenticatedClient(ctx, c, c.loadCurrentConfig, func(api *rest_management_api_client.ZitiEdgeManagement) (*EdgeRouter, error) {
		var offset int64
		limit := int64(100)

		for {
			resp, err := api.EdgeRouter.ListEdgeRouters(
				edgerouterapi.NewListEdgeRoutersParamsWithContext(ctx).WithLimit(&limit).WithOffset(&offset),
				nil,
			)
			if err != nil {
				return nil, wrapAPICallError("list edge routers", err)
			}
			if resp.Payload == nil {
				return nil, nil
			}

			count := 0
			for _, item := range resp.Payload.Data {
				count++
				router := edgeRouterFromDetail(item)
				if router != nil && router.Name == name {
					return router, nil
				}
			}
			if count == 0 || count < int(limit) {
				return nil, nil
			}
			offset += int64(count)
		}
	})
}

func edgeRouterFromDetail(detail *rest_model.EdgeRouterDetail) *EdgeRouter {
	if detail == nil || detail.ID == nil || detail.Name == nil || strings.TrimSpace(*detail.Name) == "" {
		return nil
	}
	return &EdgeRouter{ID: strings.TrimSpace(*detail.ID), Name: strings.TrimSpace(*detail.Name)}
}
