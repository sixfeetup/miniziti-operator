package controller

import (
	"context"
	"errors"
	"testing"
	"time"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"example.com/miniziti-operator/internal/entra"
	openziti "example.com/miniziti-operator/internal/openziti/client"
	"example.com/miniziti-operator/internal/rolesync"
)

func TestRoleSyncRetryClasses(t *testing.T) {
	for _, tc := range []struct {
		reason string
		err    error
	}{
		{"Synced", nil}, {"CleanupPending", nil}, {"InvalidSpec", errors.New("bad config")}, {"Conflict", nil}, {"WouldRemoveAll", nil},
		{roleSyncGraphError, &entra.TokenError{Code: "AADSTS7000215", Message: "rejected"}},
		{roleSyncGraphError, &entra.GraphError{StatusCode: 401}}, {roleSyncGraphError, &entra.GraphError{StatusCode: 403}},
		{roleSyncGraphError, entra.ErrServicePrincipalNotFound}, {roleSyncGraphError, entra.ErrLimitedUserData}, {roleSyncGraphError, rolesync.ErrNoUserRoles},
	} {
		got, err := retryResult(10*time.Minute, tc.reason, tc.err, 0)
		if got.RequeueAfter != 10*time.Minute || err != nil {
			t.Fatalf("%+v: got=%+v err=%v", tc, got, err)
		}
	}
}
func TestRoleSyncTransientRetry(t *testing.T) {
	for _, tc := range []struct {
		reason string
		err    error
	}{
		{roleSyncGraphError, errors.New("network failed")}, {roleSyncGraphError, context.DeadlineExceeded}, {roleSyncGraphError, &entra.GraphError{StatusCode: 500}}, {roleSyncZitiError, errors.New("ziti failed")},
	} {
		got, err := retryResult(10*time.Minute, tc.reason, tc.err, 0)
		if err == nil || got.RequeueAfter != 0 {
			t.Fatalf("%+v: got=%+v err=%v", tc, got, err)
		}
	}
	for _, status := range []int{429, 503} {
		got, err := retryResult(10*time.Minute, roleSyncGraphError, &entra.GraphError{StatusCode: status, RetryAfter: 120 * time.Second}, 120*time.Second)
		if err != nil || got.RequeueAfter != 120*time.Second {
			t.Fatalf("got=%+v err=%v", got, err)
		}
	}
}
func TestRoleSyncDeletionAndNotFound(t *testing.T) {
	for _, deleting := range []bool{false, true} {
		f := newRoleSyncFixture(t, interceptor.Funcs{})
		graphCalls, zitiCalls := 0, 0
		f.r.DirectoryFactory = func(_, _, _ string) entra.Directory { graphCalls++; return f.directory }
		f.backend.ListIdentitiesFunc = func(context.Context) ([]openziti.Identity, error) { zitiCalls++; return f.identities, nil }
		if deleting {
			// An unrelated finalizer holds deletion open; this controller adds none.
			f.obj.Finalizers = []string{"test-hold"}
			if err := f.r.Update(context.Background(), f.obj); err != nil {
				t.Fatal(err)
			}
		}
		if err := f.r.Delete(context.Background(), f.obj); err != nil {
			t.Fatal(err)
		}
		_, err := f.r.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(f.obj)})
		if err != nil || graphCalls != 0 || zitiCalls != 0 {
			t.Fatalf("err=%v Graph=%d Ziti=%d", err, graphCalls, zitiCalls)
		}
		if !deleting && len(f.obj.Finalizers) != 0 {
			t.Fatal("unexpected finalizer")
		}
	}
}
