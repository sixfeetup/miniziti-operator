package client

import (
	"strings"

	"github.com/openziti/edge-api/rest_model"
)

// Identity models the subset of OpenZiti identity state needed by the operator.
type Identity struct {
	ID             string
	Name           string
	Type           string
	RoleAttributes []string
	CreateOTT      bool
	ExternalID     string
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
	externalID := ""
	if detail.ExternalID != nil {
		externalID = *detail.ExternalID
	}
	return &Identity{
		ExternalID:     externalID,
		ID:             *detail.ID,
		Name:           *detail.Name,
		Type:           detail.Type.Name,
		RoleAttributes: roleAttributes,
	}
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
