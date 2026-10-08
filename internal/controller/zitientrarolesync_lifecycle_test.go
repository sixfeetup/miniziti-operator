package controller

import (
	"context"
	"reflect"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	v1alpha1 "example.com/miniziti-operator/api/v1alpha1"
	openziti "example.com/miniziti-operator/internal/openziti/client"
)

type lifecycleReader struct {
	client.Reader
	beforeSelfRead func()
}

func (r *lifecycleReader) Get(ctx context.Context, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
	if _, ok := obj.(*v1alpha1.ZitiEntraRoleSync); ok && r.beforeSelfRead != nil {
		r.beforeSelfRead()
	}
	return r.Reader.Get(ctx, key, obj, opts...)
}

func TestRoleSyncAbortsReplacedOrDeletingResource(t *testing.T) {
	for _, phase := range []string{"snapshot", "claim", "patch", "status"} {
		for _, change := range []string{"replace", "delete"} {
			t.Run(phase+"/"+change, func(t *testing.T) {
				ctx := context.Background()
				f := newRoleSyncFixture(t, interceptor.Funcs{})
				var original v1alpha1.ZitiEntraRoleSync
				if err := f.r.Get(ctx, client.ObjectKeyFromObject(f.obj), &original); err != nil {
					t.Fatal(err)
				}
				original.UID = "original"
				if change == "delete" {
					original.Finalizers = []string{"test-hold"}
				}
				if err := f.r.Update(ctx, &original); err != nil {
					t.Fatal(err)
				}
				f.obj = original.DeepCopy()
				changed := false
				mutate := func() {
					if changed {
						return
					}
					changed = true
					if err := f.r.Delete(ctx, &original); err != nil {
						t.Fatal(err)
					}
					if change == "replace" {
						replacement := original.DeepCopy()
						replacement.ObjectMeta = metav1.ObjectMeta{Name: original.Name, Namespace: original.Namespace, UID: "replacement", Generation: 1}
						replacement.Spec.AppID = "00000000-0000-0000-0000-000000000099"
						replacement.Status = v1alpha1.ZitiEntraRoleSyncStatus{ManagedAttributes: []string{"replacement-only"}}
						if phase == "claim" {
							replacement.Status.ManagedAttributes = original.Status.ManagedAttributes
						}
						if err := f.r.Create(ctx, replacement); err != nil {
							t.Fatal(err)
						}
					}
				}
				switch phase {
				case "snapshot":
					f.backend.ListIdentitiesFunc = func(context.Context) ([]openziti.Identity, error) {
						mutate()
						return f.identities, nil
					}
				case "claim":
					selfReads := 0
					f.r.APIReader = &lifecycleReader{Reader: f.reader, beforeSelfRead: func() {
						selfReads++
						if selfReads == 2 {
							mutate()
						}
					}}
				case "patch":
					get := f.backend.GetIdentityFunc
					f.backend.GetIdentityFunc = func(ctx context.Context, id string) (*openziti.Identity, error) {
						mutate()
						return get(ctx, id)
					}
				}
				result := f.run()
				if phase == "status" {
					mutate()
				} else if result.Err == nil || f.patches != 0 {
					t.Fatalf("old run continued: result=%+v patches=%d", result, f.patches)
				}
				if !changed {
					t.Fatal("resource lifecycle change was not exercised")
				}
				var before v1alpha1.ZitiEntraRoleSync
				if err := f.r.Get(ctx, client.ObjectKeyFromObject(f.obj), &before); err != nil {
					t.Fatal(err)
				}
				if err := f.r.persistSyncResult(ctx, f.obj, result); err == nil {
					t.Fatal("old result was persisted on replaced/deleting resource")
				}
				var after v1alpha1.ZitiEntraRoleSync
				if err := f.r.Get(ctx, client.ObjectKeyFromObject(f.obj), &after); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(before.Status, after.Status) {
					t.Fatalf("status changed: before=%+v after=%+v", before.Status, after.Status)
				}
			})
		}
	}
}
