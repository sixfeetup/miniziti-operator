package controller

import (
	"context"
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	v1alpha1 "example.com/miniziti-operator/api/v1alpha1"
	"example.com/miniziti-operator/internal/rolesync"
)

const roleSyncSynced = "Synced"

func storeRoleSyncStatus(t *testing.T, f *roleSyncFixture) {
	t.Helper()
	var obj v1alpha1.ZitiEntraRoleSync
	if err := f.r.Get(context.Background(), client.ObjectKeyFromObject(f.obj), &obj); err != nil {
		t.Fatal(err)
	}
	obj.Status = *f.obj.Status.DeepCopy()
	if err := f.r.Status().Update(context.Background(), &obj); err != nil {
		t.Fatal(err)
	}
	f.obj.ResourceVersion = obj.ResourceVersion
}

func TestRoleSyncStatusResults(t *testing.T) {
	previous := metav1.NewTime(time.Unix(10, 0))
	for _, reason := range []string{roleSyncSynced, roleSyncCleanupPending, roleSyncZitiError} {
		f := newRoleSyncFixture(t, interceptor.Funcs{})
		f.obj.Status.ManagedAttributes = []string{"alpha", "beta"}
		f.obj.Status.LastSyncTime = previous.DeepCopy()
		f.obj.Status.ID = "must-clear"
		storeRoleSyncStatus(t, f)
		completion := &rolesync.Completion{ManagedAttributes: []string{"alpha", "beta"}, Reason: reason}
		if reason == roleSyncSynced {
			completion.ManagedAttributes = []string{"beta"}
			completion.Ready = true
			completion.AdvanceLastSyncTime = true
		}
		result := syncRunResult{Completion: completion, Reason: reason, WritePhaseStarted: true}
		if err := f.r.persistSyncResult(context.Background(), f.obj, result); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(f.obj.Status.ManagedAttributes, completion.ManagedAttributes) || f.obj.Status.ID != "" {
			t.Fatalf("%+v", f.obj.Status)
		}
		if reason == roleSyncSynced {
			if !f.obj.Status.LastSyncTime.After(previous.Time) {
				t.Fatal("timestamp did not advance")
			}
		} else if !f.obj.Status.LastSyncTime.Equal(&previous) {
			t.Fatal("timestamp advanced")
		}
	}
}
func TestRoleSyncStatusUsesWritePhaseCounters(t *testing.T) {
	f := newRoleSyncFixture(t, interceptor.Funcs{})
	unmatched := []rolesync.UnmatchedMember{}
	for i := 0; i < 60; i++ {
		unmatched = append(unmatched, rolesync.UnmatchedMember{ID: "user", Reason: "MissingProperty"})
	}
	result := syncRunResult{Plan: &rolesync.PlanResult{MatchedIdentities: 2, MembersWithoutIdentity: 3, UnmatchedMembers: unmatched}, Completion: &rolesync.Completion{ManagedAttributes: []string{"alpha", "beta"}, IdentitiesUpdated: 1, Reason: roleSyncCleanupPending}, WritePhaseStarted: true, Reason: roleSyncCleanupPending}
	if err := f.r.persistSyncResult(context.Background(), f.obj, result); err != nil {
		t.Fatal(err)
	}
	if f.obj.Status.MatchedIdentities != 2 || f.obj.Status.IdentitiesUpdated != 1 || f.obj.Status.MembersWithoutIdentity != 3 || len(f.obj.Status.UnmatchedMembers) != 50 {
		t.Fatalf("%+v", f.obj.Status)
	}
	before := f.obj.Status.DeepCopy()
	if err := f.r.persistSyncResult(context.Background(), f.obj, syncRunResult{Reason: roleSyncGraphError, Err: errors.New("read failed")}); err != nil {
		t.Fatal(err)
	}
	if f.obj.Status.MatchedIdentities != before.MatchedIdentities || f.obj.Status.IdentitiesUpdated != before.IdentitiesUpdated || f.obj.Status.MembersWithoutIdentity != before.MembersWithoutIdentity || !reflect.DeepEqual(f.obj.Status.UnmatchedMembers, before.UnmatchedMembers) {
		t.Fatal("read error replaced counters")
	}
}
func TestRoleSyncIdenticalFailureDoesNotRepeatWarning(t *testing.T) {
	f := newRoleSyncFixture(t, interceptor.Funcs{})
	recorder := record.NewFakeRecorder(10)
	f.r.Recorder = recorder
	result := syncRunResult{Reason: roleSyncGraphError, Err: errors.New("stable failure")}
	if err := f.r.persistSyncResult(context.Background(), f.obj, result); err != nil {
		t.Fatal(err)
	}
	before := apimeta.FindStatusCondition(f.obj.Status.Conditions, v1alpha1.ConditionTypeReady).LastTransitionTime
	if err := f.r.persistSyncResult(context.Background(), f.obj, result); err != nil {
		t.Fatal(err)
	}
	after := apimeta.FindStatusCondition(f.obj.Status.Conditions, v1alpha1.ConditionTypeReady).LastTransitionTime
	if !before.Equal(&after) {
		t.Fatal("identical failure changed transition time")
	}
	if len(recorder.Events) != 1 {
		t.Fatalf("events=%d", len(recorder.Events))
	}
	if !strings.HasPrefix(<-recorder.Events, "Warning") {
		t.Fatal("expected Warning")
	}
	result.Reason = roleSyncZitiError
	result.Err = errors.New("changed failure")
	if err := f.r.persistSyncResult(context.Background(), f.obj, result); err != nil {
		t.Fatal(err)
	}
	if len(recorder.Events) != 1 {
		t.Fatal("changed failure emitted no event")
	}
}
func TestRoleSyncFinalStatusFailurePreservesRecordedHistory(t *testing.T) {
	updates := 0
	f := newRoleSyncFixture(t, interceptor.Funcs{SubResourceUpdate: func(ctx context.Context, c client.Client, sub string, obj client.Object, opts ...client.SubResourceUpdateOption) error {
		updates++
		if updates == 2 {
			return errors.New("final status rejected")
		}
		return c.SubResource(sub).Update(ctx, obj, opts...)
	}})
	previous := metav1.NewTime(time.Unix(10, 0))
	f.obj.Status.LastSyncTime = previous.DeepCopy()
	storeRoleSyncStatus(t, f)
	updates = 0
	f.backend.PatchIdentityRoleAttributesFunc = func(_ context.Context, _ string, attrs []string) error {
		f.patches++
		f.identities[0].RoleAttributes = append([]string{}, attrs...)
		return nil
	}
	_, err := f.r.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKeyFromObject(f.obj)})
	if err == nil || f.patches != 1 {
		t.Fatalf("err=%v patches=%d", err, f.patches)
	}
	var stored v1alpha1.ZitiEntraRoleSync
	if err := f.r.Get(context.Background(), client.ObjectKeyFromObject(f.obj), &stored); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(stored.Status.ManagedAttributes, []string{"alpha", "beta"}) || !stored.Status.LastSyncTime.Equal(&previous) {
		t.Fatalf("%+v", stored.Status)
	}
	result := f.r.runSync(context.Background(), &stored)
	if result.Plan == nil || !slices.Contains(result.Plan.ManagedValues, "alpha") {
		t.Fatalf("lost retirement history: %+v", result)
	}
}
