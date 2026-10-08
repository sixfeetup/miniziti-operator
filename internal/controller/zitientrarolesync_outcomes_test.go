package controller

import (
	"context"
	"reflect"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	v1alpha1 "example.com/miniziti-operator/api/v1alpha1"
	"example.com/miniziti-operator/internal/rolesync"
)

func TestRoleSyncLateConflictDoesNotMutatePlan(t *testing.T) {
	f := newRoleSyncFixture(t, interceptor.Funcs{})
	ctx := context.Background()
	input, err := f.r.readGraph(ctx, f.directory, f.obj)
	if err != nil {
		t.Fatal(err)
	}
	input.Identities = f.identities
	input.Self, input.OtherClaims, input.IdentityOwners, err = f.r.readOwnership(ctx, f.obj)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := rolesync.BuildPlan(input)
	if err != nil {
		t.Fatal(err)
	}
	before := plan.Ownership
	other := &v1alpha1.ZitiEntraRoleSync{ObjectMeta: metav1.ObjectMeta{Name: "new-owner", Namespace: "other"}, Status: v1alpha1.ZitiEntraRoleSyncStatus{ManagedAttributes: []string{"beta"}}}
	if err := f.r.Create(ctx, other); err != nil {
		t.Fatal(err)
	}
	outcome := f.r.applyRolePatch(ctx, f.obj, plan, plan.Patches[0])
	if outcome.Err != nil || outcome.Kind != rolesync.Conflicted || !reflect.DeepEqual(outcome.ConflictValues, []string{"beta"}) || !outcome.StopsWrites() || f.patches != 0 {
		t.Fatalf("outcome=%+v patches=%d", outcome, f.patches)
	}
	completion := rolesync.Complete(rolesync.CompletionInput{Plan: plan, PreviousValues: input.Self.ManagedAttributes, ClaimRecorded: true, Outcomes: []rolesync.IdentityOutcome{outcome}})
	if message := roleSyncMessage(syncRunResult{Plan: &plan, Completion: &completion, Reason: completion.Reason}); message != "Attributes owned by another role sync: beta" {
		t.Fatalf("message=%q", message)
	}
	if !reflect.DeepEqual(before, plan.Ownership) {
		t.Fatalf("late ownership read changed the planned snapshot: before=%+v after=%+v", before, plan.Ownership)
	}
}
