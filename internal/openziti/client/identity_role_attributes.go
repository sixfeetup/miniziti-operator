package client

import (
	"context"

	"github.com/openziti/edge-api/rest_management_api_client"
	identityapi "github.com/openziti/edge-api/rest_management_api_client/identity"
	"github.com/openziti/edge-api/rest_model"
)

// PatchIdentityRoleAttributes changes no other identity fields.
func (c *ManagementClient) PatchIdentityRoleAttributes(ctx context.Context, id string, attrs []string) error {
	attributes := rest_model.Attributes(append([]string{}, attrs...))
	_, err := useAuthenticatedClient(ctx, c, c.loadCurrentConfig, func(api *rest_management_api_client.ZitiEdgeManagement) (struct{}, error) {
		_, err := api.Identity.PatchIdentity(identityapi.NewPatchIdentityParamsWithContext(ctx).WithID(id).WithIdentity(&rest_model.IdentityPatch{RoleAttributes: &attributes}), nil)
		if err != nil {
			return struct{}{}, wrapAPICallError("patch identity role attributes", err)
		}
		return struct{}{}, nil
	})
	return err
}

func (f *FakeClient) PatchIdentityRoleAttributes(ctx context.Context, id string, attrs []string) error {
	if f.PatchIdentityRoleAttributesFunc != nil {
		return f.PatchIdentityRoleAttributesFunc(ctx, id, attrs)
	}
	return nil
}
