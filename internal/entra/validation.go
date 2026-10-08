package entra

import (
	"encoding/json"
	"errors"
)

// Wire types preserve presence where a zero value is also legitimate data.
// In particular, disabled roles and nullable mail must not look like omitted fields.
type graphRole struct {
	AppRole
	IsEnabled *bool `json:"isEnabled"`
}
type graphServicePrincipal struct {
	ID       string       `json:"id"`
	AppRoles *[]graphRole `json:"appRoles"`
}
type graphMember struct {
	User
	Type string          `json:"@odata.type"`
	Mail json.RawMessage `json:"mail"`
}

func (p graphServicePrincipal) snapshot() (*ServicePrincipal, error) {
	if p.ID == "" || p.AppRoles == nil {
		return nil, errors.New("incomplete Graph service principal")
	}
	result := &ServicePrincipal{ID: p.ID, AppRoles: []AppRole{}}
	for _, wire := range *p.AppRoles {
		if wire.ID == "" || wire.Value == "" || wire.IsEnabled == nil || len(wire.AllowedMemberTypes) == 0 {
			return nil, errors.New("incomplete Graph app role")
		}
		for _, kind := range wire.AllowedMemberTypes {
			if kind != "User" && kind != "Application" {
				return nil, errors.New("unknown Graph app role member type")
			}
		}
		role := wire.AppRole
		role.IsEnabled = *wire.IsEnabled
		result.AppRoles = append(result.AppRoles, role)
	}
	return result, nil
}

func validateAssignments(assignments []AppRoleAssignment) error {
	for _, assignment := range assignments {
		if assignment.PrincipalID == "" || assignment.AppRoleID == "" {
			return errors.New("incomplete Graph app role assignment")
		}
		switch assignment.PrincipalType {
		case "Group", "User", "ServicePrincipal":
		default:
			return errors.New("missing or unknown Graph assignment principal type")
		}
	}
	return nil
}

func groupUsers(members []graphMember) ([]User, error) {
	users := []User{}
	for _, member := range members {
		if member.ID == "" {
			return nil, errors.New("incomplete Graph group member")
		}
		switch member.Type {
		case "#microsoft.graph.group", "#microsoft.graph.device", "#microsoft.graph.servicePrincipal", "#microsoft.graph.orgContact":
			continue
		case "#microsoft.graph.user":
		default:
			return nil, errors.New("missing or unknown Graph group member type")
		}
		if member.UserPrincipalName == "" {
			return nil, ErrLimitedUserData
		}
		user := member.User
		if len(member.Mail) == 0 || json.Unmarshal(member.Mail, &user.Mail) != nil {
			return nil, errors.New("missing or invalid Graph user mail")
		}
		users = append(users, user)
	}
	return users, nil
}
