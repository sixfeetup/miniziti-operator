package controller

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"time"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1alpha1 "example.com/miniziti-operator/api/v1alpha1"
	"example.com/miniziti-operator/internal/entra"
	"example.com/miniziti-operator/internal/rolesync"
)

var roleSyncGUID = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

func validateRoleSyncSpec(resource *v1alpha1.ZitiEntraRoleSync) (time.Duration, error) {
	interval := 10 * time.Minute
	if resource.Spec.Interval != "" {
		parsed, err := time.ParseDuration(resource.Spec.Interval)
		if err != nil {
			return interval, errors.New("invalid sync interval")
		}
		interval = parsed
	}
	if interval < time.Minute {
		return 10 * time.Minute, errors.New("sync interval must be at least 1m")
	}
	if !roleSyncGUID.MatchString(resource.Spec.TenantID) || !roleSyncGUID.MatchString(resource.Spec.AppID) {
		return interval, errors.New("tenantId and appId must be GUIDs")
	}
	if resource.Spec.CredentialsSecretRef.Name == "" {
		return interval, errors.New("credentials Secret name is required")
	}
	switch resource.Spec.UserProperty {
	case "", "mail", "userPrincipalName", "id":
	default:
		return interval, errors.New("unsupported userProperty")
	}
	return interval, nil
}

func (r *ZitiEntraRoleSyncReconciler) readDirectory(ctx context.Context, resource *v1alpha1.ZitiEntraRoleSync) (entra.Directory, error) {
	var secret corev1.Secret
	if err := r.Get(ctx, client.ObjectKey{Namespace: resource.Namespace, Name: resource.Spec.CredentialsSecretRef.Name}, &secret); err != nil {
		return nil, fmt.Errorf("read Entra credentials Secret: %w", err)
	}
	id, key := string(secret.Data["clientId"]), string(secret.Data["clientSecret"])
	if id == "" || key == "" {
		return nil, errors.New("credentials Secret requires clientId and clientSecret")
	}
	return r.DirectoryFactory(resource.Spec.TenantID, id, key), nil
}

func (r *ZitiEntraRoleSyncReconciler) readGraph(ctx context.Context, directory entra.Directory, resource *v1alpha1.ZitiEntraRoleSync) (rolesync.Input, error) {
	input := rolesync.Input{MembersByGroup: map[string][]entra.User{}}
	principal, err := directory.GetServicePrincipal(ctx, resource.Spec.AppID)
	if err != nil {
		return input, err
	}
	if principal == nil {
		return input, errors.New("graph returned no service principal")
	}
	input.ServicePrincipal = *principal
	userRoles := false
	enabled := map[string]bool{}
	for _, role := range principal.AppRoles {
		if slices.Contains(role.AllowedMemberTypes, "User") {
			userRoles = true
			if role.IsEnabled {
				enabled[role.ID] = true
			}
		}
	}
	if !userRoles {
		return input, rolesync.ErrNoUserRoles
	}
	input.Assignments, err = directory.ListAppRoleAssignedTo(ctx, principal.ID)
	if err != nil {
		return input, err
	}
	groups := []string{}
	for _, assignment := range input.Assignments {
		if assignment.PrincipalType == "Group" && assignment.AppRoleID != "00000000-0000-0000-0000-000000000000" && enabled[assignment.AppRoleID] && !slices.Contains(groups, assignment.PrincipalID) {
			groups = append(groups, assignment.PrincipalID)
		}
	}
	slices.Sort(groups)
	for _, id := range groups {
		users, err := directory.ListGroupUsers(ctx, id)
		if err != nil {
			return input, err
		}
		// Enforce the permission guard even on injected Directory implementations.
		for _, user := range users {
			if user.UserPrincipalName == "" {
				return input, entra.ErrLimitedUserData
			}
		}
		input.MembersByGroup[id] = users
	}
	return input, nil
}

func graphRunFailure(err error) syncRunResult {
	result := syncRunResult{Reason: syncReasonGraphError, Err: err}
	var graphError *entra.GraphError
	if errors.As(err, &graphError) {
		result.RetryAfter = graphError.RetryAfter
	}
	return result
}

func roleClaim(resource *v1alpha1.ZitiEntraRoleSync) rolesync.SyncClaim {
	return rolesync.SyncClaim{Resource: rolesync.ResourceKey{Namespace: resource.Namespace, Name: resource.Name}, CreationTimestamp: resource.CreationTimestamp.Time, ManagedAttributes: append([]string(nil), resource.Status.ManagedAttributes...)}
}

func (r *ZitiEntraRoleSyncReconciler) readOwnership(ctx context.Context, key client.ObjectKey) (rolesync.SyncClaim, []rolesync.SyncClaim, []rolesync.IdentityOwner, error) {
	var self v1alpha1.ZitiEntraRoleSync
	if err := r.APIReader.Get(ctx, key, &self); err != nil {
		return rolesync.SyncClaim{}, nil, nil, err
	}
	var syncs v1alpha1.ZitiEntraRoleSyncList
	if err := r.APIReader.List(ctx, &syncs); err != nil {
		return rolesync.SyncClaim{}, nil, nil, err
	}
	var identities v1alpha1.ZitiIdentityList
	if err := r.APIReader.List(ctx, &identities); err != nil {
		return rolesync.SyncClaim{}, nil, nil, err
	}
	others := []rolesync.SyncClaim{}
	for i := range syncs.Items {
		item := &syncs.Items[i]
		if client.ObjectKeyFromObject(item) != key {
			others = append(others, roleClaim(item))
		}
	}
	owners := make([]rolesync.IdentityOwner, 0, len(identities.Items))
	for _, identity := range identities.Items {
		owners = append(owners, rolesync.IdentityOwner{Resource: rolesync.ResourceKey{Namespace: identity.Namespace, Name: identity.Name}, IdentityID: identity.Status.ID, IdentityName: identity.Spec.Name})
	}
	return roleClaim(&self), others, owners, nil
}

func (r *ZitiEntraRoleSyncReconciler) recordRoleClaims(ctx context.Context, resource *v1alpha1.ZitiEntraRoleSync, self rolesync.SyncClaim, values []string) error {
	if sameRoleSet(self.ManagedAttributes, values) {
		return nil
	}
	var current v1alpha1.ZitiEntraRoleSync
	if err := r.APIReader.Get(ctx, client.ObjectKeyFromObject(resource), &current); err != nil {
		return err
	}
	if !sameRoleSet(current.Status.ManagedAttributes, self.ManagedAttributes) {
		return errors.New("recorded claims changed during sync")
	}
	current.Status.ManagedAttributes = append([]string(nil), values...)
	if err := r.Status().Update(ctx, &current); err != nil {
		return err
	}
	resource.ResourceVersion = current.ResourceVersion
	resource.Status.ManagedAttributes = append([]string(nil), values...)
	return nil
}

func sameRoleSet(a, b []string) bool {
	a, b = append([]string{}, a...), append([]string{}, b...)
	slices.Sort(a)
	slices.Sort(b)
	return slices.Equal(slices.Compact(a), slices.Compact(b))
}
