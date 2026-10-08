package entra

import "context"

// Directory supplies complete snapshots. On error, list calls return no partial data.
type Directory interface {
	GetServicePrincipal(context.Context, string) (*ServicePrincipal, error)
	ListAppRoleAssignedTo(context.Context, string) ([]AppRoleAssignment, error)
	ListGroupUsers(context.Context, string) ([]User, error)
}
type Factory func(tenantID, clientID, clientSecret string) Directory

type ServicePrincipal struct {
	ID       string    `json:"id"`
	AppRoles []AppRole `json:"appRoles"`
}
type AppRole struct {
	ID                 string   `json:"id"`
	Value              string   `json:"value"`
	AllowedMemberTypes []string `json:"allowedMemberTypes"`
	IsEnabled          bool     `json:"isEnabled"`
}
type AppRoleAssignment struct {
	PrincipalID   string `json:"principalId"`
	PrincipalType string `json:"principalType"`
	AppRoleID     string `json:"appRoleId"`
}
type User struct {
	ID                string `json:"id"`
	DisplayName       string `json:"displayName"`
	Mail              string `json:"mail"`
	UserPrincipalName string `json:"userPrincipalName"`
}
