package controller

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	v1alpha1 "example.com/miniziti-operator/api/v1alpha1"
	"example.com/miniziti-operator/internal/entra"
	openziti "example.com/miniziti-operator/internal/openziti/client"
)

const roleSyncNamespace = "sync-ns"
const roleSyncName = "sync"
const roleSyncGraphError = "GraphError"
const roleSyncConflict = "Conflict"
const roleSyncCleanupPending = "CleanupPending"
const roleSyncZitiError = "ZitiError"

type roleSyncDirectory struct {
	principal                              *entra.ServicePrincipal
	assignments                            []entra.AppRoleAssignment
	members                                map[string][]entra.User
	principalErr, assignmentErr, memberErr error
}

func (d *roleSyncDirectory) GetServicePrincipal(context.Context, string) (*entra.ServicePrincipal, error) {
	return d.principal, d.principalErr
}
func (d *roleSyncDirectory) ListAppRoleAssignedTo(context.Context, string) ([]entra.AppRoleAssignment, error) {
	return d.assignments, d.assignmentErr
}
func (d *roleSyncDirectory) ListGroupUsers(_ context.Context, id string) ([]entra.User, error) {
	return d.members[id], d.memberErr
}

type roleSyncReader struct {
	client.Reader
	lists    int
	listHook func(client.ObjectList, int) error
}

func (s *roleSyncReader) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	s.lists++
	if s.listHook != nil {
		if err := s.listHook(list, s.lists); err != nil {
			return err
		}
	}
	return s.Reader.List(ctx, list, opts...)
}

type roleSyncFixture struct {
	r          *ZitiEntraRoleSyncReconciler
	obj        *v1alpha1.ZitiEntraRoleSync
	directory  *roleSyncDirectory
	backend    *openziti.FakeClient
	identities []openziti.Identity
	patches    int
	attrs      [][]string
	reader     *roleSyncReader
}

func newRoleSyncFixture(t *testing.T, intercept interceptor.Funcs) *roleSyncFixture {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := v1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	obj := &v1alpha1.ZitiEntraRoleSync{ObjectMeta: metav1.ObjectMeta{Name: roleSyncName, Namespace: roleSyncNamespace, CreationTimestamp: metav1.NewTime(time.Unix(100, 0)), Generation: 1}, Spec: v1alpha1.ZitiEntraRoleSyncSpec{TenantID: "00000000-0000-0000-0000-000000000001", AppID: "00000000-0000-0000-0000-000000000002", CredentialsSecretRef: corev1.LocalObjectReference{Name: "entra"}}, Status: v1alpha1.ZitiEntraRoleSyncStatus{ManagedAttributes: []string{"alpha"}}}
	secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "entra", Namespace: roleSyncNamespace}, Data: map[string][]byte{"clientId": []byte("reader"), "clientSecret": []byte("secret")}}
	c := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(obj).WithObjects(obj, secret).WithInterceptorFuncs(intercept).Build()
	f := &roleSyncFixture{obj: obj, identities: []openziti.Identity{{ID: "i1", Name: "Alice", ExternalID: "alice@example.com", RoleAttributes: []string{"custom", "alpha"}}}}
	f.directory = &roleSyncDirectory{principal: &entra.ServicePrincipal{ID: "sp", AppRoles: []entra.AppRole{{ID: "beta", Value: "beta", IsEnabled: true, AllowedMemberTypes: []string{"User"}}}}, assignments: []entra.AppRoleAssignment{{PrincipalID: "g", PrincipalType: "Group", AppRoleID: "beta"}}, members: map[string][]entra.User{"g": {{ID: "u1", Mail: "alice@example.com", UserPrincipalName: "alice@example.com"}}}}
	f.backend = &openziti.FakeClient{
		ListIdentitiesFunc: func(context.Context) ([]openziti.Identity, error) { return f.identities, nil },
		GetIdentityFunc: func(_ context.Context, id string) (*openziti.Identity, error) {
			for _, i := range f.identities {
				if i.ID == id {
					return &i, nil
				}
			}
			return nil, nil
		},
		PatchIdentityRoleAttributesFunc: func(_ context.Context, _ string, attrs []string) error {
			f.patches++
			f.attrs = append(f.attrs, append([]string{}, attrs...))
			return nil
		},
	}
	f.reader = &roleSyncReader{Reader: c}
	f.r = &ZitiEntraRoleSyncReconciler{Client: c, APIReader: f.reader, Scheme: scheme, ZitiClient: f.backend, DirectoryFactory: func(_, _, _ string) entra.Directory { return f.directory }}
	return f
}
func (f *roleSyncFixture) run() syncRunResult { return f.r.runSync(context.Background(), f.obj) }

func TestRoleSyncReadFailureDoesNotWrite(t *testing.T) {
	for _, kind := range []string{"secret", "key", "token", "graph", "roles", "identities", "ownership"} {
		t.Run(kind, func(t *testing.T) {
			f := newRoleSyncFixture(t, interceptor.Funcs{})
			switch kind {
			case "secret":
				s := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "entra", Namespace: roleSyncNamespace}}
				if err := f.r.Delete(context.Background(), s); err != nil {
					t.Fatal(err)
				}
			case "key":
				var s corev1.Secret
				if err := f.r.Get(context.Background(), client.ObjectKey{Namespace: roleSyncNamespace, Name: "entra"}, &s); err != nil {
					t.Fatal(err)
				}
				delete(s.Data, "clientSecret")
				if err := f.r.Update(context.Background(), &s); err != nil {
					t.Fatal(err)
				}
			case "token":
				f.directory.principalErr = &entra.TokenError{Code: "AADSTS7000215", Message: "bad credential"}
			case "graph":
				f.directory.assignmentErr = &entra.GraphError{StatusCode: 503, Message: "unavailable"}
			case "roles":
				f.directory.principal.AppRoles = nil
			case "identities":
				f.backend.ListIdentitiesFunc = func(context.Context) ([]openziti.Identity, error) { return nil, errors.New("read failed") }
			case "ownership":
				f.reader.listHook = func(client.ObjectList, int) error { return errors.New("ownership failed") }
			}
			result := f.run()
			if result.Err == nil || f.patches != 0 || !reflect.DeepEqual(f.obj.Status.ManagedAttributes, []string{"alpha"}) {
				t.Fatalf("result=%+v patches=%d status=%v", result, f.patches, f.obj.Status.ManagedAttributes)
			}
		})
	}
}

func TestRoleSyncUsesUncachedOwnership(t *testing.T) {
	f := newRoleSyncFixture(t, interceptor.Funcs{List: func(ctx context.Context, c client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
		if _, ok := list.(*v1alpha1.ZitiEntraRoleSyncList); ok {
			return nil
		}
		return c.List(ctx, list, opts...)
	}})
	reader := fake.NewClientBuilder().WithScheme(f.r.Scheme).WithStatusSubresource(f.obj).WithObjects(f.obj, &v1alpha1.ZitiEntraRoleSync{ObjectMeta: metav1.ObjectMeta{Name: "owner", Namespace: "other", CreationTimestamp: metav1.NewTime(time.Unix(200, 0))}, Status: v1alpha1.ZitiEntraRoleSyncStatus{ManagedAttributes: []string{"beta"}}}).Build()
	f.reader.Reader = reader
	result := f.run()
	if f.reader.lists == 0 || result.Reason != roleSyncConflict || f.patches != 0 {
		t.Fatalf("result=%+v lists=%d patches=%d", result, f.reader.lists, f.patches)
	}
}

func TestRoleSyncIncompleteGraphReadDoesNotWrite(t *testing.T) {
	f := newRoleSyncFixture(t, interceptor.Funcs{})
	f.directory.memberErr = errors.New("later page failed")
	result := f.run()
	if f.patches != 0 || result.Reason != roleSyncGraphError || !reflect.DeepEqual(f.obj.Status.ManagedAttributes, []string{"alpha"}) {
		t.Fatalf("%+v", result)
	}
}

func TestRoleSyncRecordsBeforeWrite(t *testing.T) {
	t.Run("ordered", func(t *testing.T) {
		f := newRoleSyncFixture(t, interceptor.Funcs{})
		f.backend.PatchIdentityRoleAttributesFunc = func(context.Context, string, []string) error {
			f.patches++
			var got v1alpha1.ZitiEntraRoleSync
			if err := f.r.Get(context.Background(), client.ObjectKeyFromObject(f.obj), &got); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got.Status.ManagedAttributes, []string{"alpha", "beta"}) {
				t.Fatalf("unrecorded claim: %v", got.Status.ManagedAttributes)
			}
			return nil
		}
		result := f.run()
		if f.patches != 1 || !result.WritePhaseStarted {
			t.Fatalf("%+v patches=%d", result, f.patches)
		}
	})
	t.Run("rejected", func(t *testing.T) {
		f := newRoleSyncFixture(t, interceptor.Funcs{SubResourceUpdate: func(context.Context, client.Client, string, client.Object, ...client.SubResourceUpdateOption) error {
			return errors.New("status rejected")
		}})
		result := f.run()
		if result.Err == nil || f.patches != 0 || result.WritePhaseStarted {
			t.Fatalf("%+v", result)
		}
	})
}

func TestRoleSyncExcludesIdentityOwner(t *testing.T) {
	for _, byName := range []bool{false, true} {
		f := newRoleSyncFixture(t, interceptor.Funcs{})
		owner := &v1alpha1.ZitiIdentity{ObjectMeta: metav1.ObjectMeta{Name: "claimed", Namespace: "other", Finalizers: []string{"test-owner"}}, Spec: v1alpha1.ZitiIdentitySpec{Name: "Alice"}}
		if !byName {
			owner.Spec.Name = "Other"
			owner.Status.ID = "i1"
		}
		if err := f.r.Create(context.Background(), owner); err != nil {
			t.Fatal(err)
		}
		if err := f.r.Delete(context.Background(), owner); err != nil {
			t.Fatal(err)
		}
		result := f.run()
		if f.patches != 0 || result.Completion == nil || result.Completion.Reason != roleSyncCleanupPending {
			t.Fatalf("%+v", result)
		}
	}
}
