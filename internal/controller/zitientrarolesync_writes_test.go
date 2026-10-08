package controller

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	v1alpha1 "example.com/miniziti-operator/api/v1alpha1"
	"example.com/miniziti-operator/internal/entra"
	openziti "example.com/miniziti-operator/internal/openziti/client"
)

func TestRoleSyncFreshReadPreservesUnmanagedEdit(t *testing.T) {
	f := newRoleSyncFixture(t, interceptor.Funcs{})
	f.backend.GetIdentityFunc = func(context.Context, string) (*openziti.Identity, error) {
		i := f.identities[0]
		i.RoleAttributes = []string{"custom", "alpha", "manual"}
		return &i, nil
	}
	result := f.run()
	if result.Reason != "Synced" || f.patches != 1 || !reflect.DeepEqual(f.attrs[0], []string{"custom", "manual", "beta"}) {
		t.Fatalf("result=%+v patches=%v", result, f.attrs)
	}
}
func TestRoleSyncFreshIdentityOwnerSkipsPatch(t *testing.T) {
	for _, byName := range []bool{false, true} {
		f := newRoleSyncFixture(t, interceptor.Funcs{})
		f.backend.GetIdentityFunc = func(context.Context, string) (*openziti.Identity, error) {
			owner := &v1alpha1.ZitiIdentity{ObjectMeta: metav1.ObjectMeta{Name: "new", Namespace: "other"}, Spec: v1alpha1.ZitiIdentitySpec{Name: "Alice"}}
			if !byName {
				owner.Spec.Name = "Other"
				owner.Status.ID = "i1"
			}
			if err := f.r.Create(context.Background(), owner); err != nil {
				t.Fatal(err)
			}
			i := f.identities[0]
			return &i, nil
		}
		result := f.run()
		if f.patches != 0 || result.Completion == nil || result.Completion.Reason != roleSyncCleanupPending || !slices.Contains(result.Completion.ManagedAttributes, "alpha") {
			t.Fatalf("%+v patches=%d", result, f.patches)
		}
	}
}
func TestRoleSyncExternalIDChangeSkipsPatch(t *testing.T) {
	for _, external := range []string{"", "other@example.com"} {
		f := newRoleSyncFixture(t, interceptor.Funcs{})
		f.backend.GetIdentityFunc = func(context.Context, string) (*openziti.Identity, error) {
			i := f.identities[0]
			i.ExternalID = external
			return &i, nil
		}
		result := f.run()
		if f.patches != 0 || result.Completion == nil || !slices.Contains(result.Completion.ManagedAttributes, "alpha") {
			t.Fatalf("%+v", result)
		}
	}
}
func TestRoleSyncDeletedFreshIdentity(t *testing.T) {
	f := newRoleSyncFixture(t, interceptor.Funcs{})
	creates := 0
	f.backend.GetIdentityFunc = func(context.Context, string) (*openziti.Identity, error) { return nil, nil }
	f.backend.CreateIdentityFunc = func(context.Context, openziti.Identity) (*openziti.Identity, error) {
		creates++
		return nil, errors.New("unexpected create")
	}
	result := f.run()
	if f.patches != 0 || creates != 0 || result.Completion == nil || slices.Contains(result.Completion.ManagedAttributes, "alpha") {
		t.Fatalf("%+v patches=%d creates=%d", result, f.patches, creates)
	}
}
func TestRoleSyncLateClaimUsesSameResolver(t *testing.T) {
	for _, age := range []int64{50, 200} {
		t.Run(time.Unix(age, 0).String(), func(t *testing.T) {
			f := newRoleSyncFixture(t, interceptor.Funcs{})
			f.backend.GetIdentityFunc = func(context.Context, string) (*openziti.Identity, error) {
				other := &v1alpha1.ZitiEntraRoleSync{ObjectMeta: metav1.ObjectMeta{Name: "other", Namespace: "other", CreationTimestamp: metav1.NewTime(time.Unix(age, 0))}, Status: v1alpha1.ZitiEntraRoleSyncStatus{ManagedAttributes: []string{"beta"}}}
				if err := f.r.Create(context.Background(), other); err != nil {
					t.Fatal(err)
				}
				// fake status subresources clear status at create just like the API.
				other.Status.ManagedAttributes = []string{"beta"}
				if err := f.r.Status().Update(context.Background(), other); err != nil {
					t.Fatal(err)
				}
				i := f.identities[0]
				return &i, nil
			}
			result := f.run()
			if age > 100 {
				if f.patches != 1 {
					t.Fatalf("%+v", result)
				}
			} else if f.patches != 0 || result.Reason != roleSyncConflict || result.Completion == nil || !reflect.DeepEqual(result.Completion.ManagedAttributes, []string{"alpha", "beta"}) {
				t.Fatalf("%+v patches=%d", result, f.patches)
			}
		})
	}
}
func twoRoleSyncIdentities(f *roleSyncFixture) {
	f.identities = append(f.identities, openziti.Identity{ID: "i2", ExternalID: "bob@example.com", RoleAttributes: []string{"alpha"}})
	f.directory.members["g"] = append(f.directory.members["g"], entra.User{ID: "u2", Mail: "bob@example.com", UserPrincipalName: "bob@example.com"})
}
func TestRoleSyncPartialPatchFailureContinues(t *testing.T) {
	f := newRoleSyncFixture(t, interceptor.Funcs{})
	twoRoleSyncIdentities(f)
	f.backend.PatchIdentityRoleAttributesFunc = func(context.Context, string, []string) error {
		f.patches++
		if f.patches == 1 {
			return errors.New("first failed")
		}
		return nil
	}
	result := f.run()
	if f.patches != 2 || result.IdentitiesUpdated != 1 || result.Completion == nil || result.Completion.Reason != roleSyncZitiError || !reflect.DeepEqual(result.Completion.ManagedAttributes, []string{"alpha", "beta"}) {
		t.Fatalf("%+v patches=%d", result, f.patches)
	}
}
func TestRoleSyncDeadlineAfterFirstPatch(t *testing.T) {
	f := newRoleSyncFixture(t, interceptor.Funcs{})
	twoRoleSyncIdentities(f)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	f.backend.PatchIdentityRoleAttributesFunc = func(context.Context, string, []string) error { f.patches++; cancel(); return nil }
	result := f.r.runSync(ctx, f.obj)
	if f.patches != 1 || result.Completion == nil || result.Completion.Reason != roleSyncZitiError || !reflect.DeepEqual(result.Completion.ManagedAttributes, []string{"alpha", "beta"}) || result.Completion.AdvanceLastSyncTime {
		t.Fatalf("%+v patches=%d", result, f.patches)
	}
}
func TestRoleSyncFreshReadFailureDoesNotPatch(t *testing.T) {
	for _, ownership := range []bool{false, true} {
		f := newRoleSyncFixture(t, interceptor.Funcs{})
		f.backend.GetIdentityFunc = func(context.Context, string) (*openziti.Identity, error) {
			if ownership {
				f.reader.listHook = func(client.ObjectList, int) error { return errors.New("fresh owner list failed") }
				i := f.identities[0]
				return &i, nil
			}
			return nil, errors.New("fresh identity read failed")
		}
		result := f.run()
		if f.patches != 0 || result.Completion == nil || result.Completion.Reason != roleSyncZitiError || !reflect.DeepEqual(result.Completion.ManagedAttributes, []string{"alpha", "beta"}) {
			t.Fatalf("%+v", result)
		}
	}
}
func TestRoleSyncFreshNoopDoesNotPatch(t *testing.T) {
	f := newRoleSyncFixture(t, interceptor.Funcs{})
	f.backend.GetIdentityFunc = func(context.Context, string) (*openziti.Identity, error) {
		i := f.identities[0]
		i.RoleAttributes = []string{"beta", "custom"}
		return &i, nil
	}
	result := f.run()
	if f.patches != 0 || result.Completion == nil || !result.Completion.Ready || slices.Contains(result.Completion.ManagedAttributes, "alpha") {
		t.Fatalf("%+v patches=%d", result, f.patches)
	}
}
