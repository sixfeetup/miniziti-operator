package integration

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "example.com/miniziti-operator/api/v1alpha1"
	"example.com/miniziti-operator/internal/entra"
	openziti "example.com/miniziti-operator/internal/openziti/client"
)

var nextRoleFixture int

const integrationRoleSynced = "Synced"
const integrationRoleConflict = "Conflict"
const integrationRoleGraphError = "GraphError"
const integrationRolePending = "CleanupPending"
const integrationRoleZitiError = "ZitiError"

type roleControllerFixture struct {
	namespace, app string
	data           entraFixture
	ids            []string
	objects        []client.Object
	cleaned        bool
}

func newRoleControllerFixture() *roleControllerFixture {
	nextRoleFixture++
	f := &roleControllerFixture{app: fmt.Sprintf("10000000-0000-0000-0000-%012d", nextRoleFixture)}
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{GenerateName: "entra-sync-"}}
	Expect(k8sClient.Create(ctx, ns)).To(Succeed())
	f.namespace = ns.Name
	f.data = entraFixture{roles: []entra.AppRole{{ID: "beta-role",
		Value:              "beta",
		IsEnabled:          true,
		AllowedMemberTypes: []string{"User"}}},
		users: map[string][]entra.User{}}
	secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "credentials",
		Namespace: ns.Name},
		Data: map[string][]byte{"clientId": []byte("reader"),
			"clientSecret": []byte("initial-secret")}}
	Expect(k8sClient.Create(ctx, secret)).To(Succeed())
	f.objects = append(f.objects, secret, ns)
	DeferCleanup(f.cleanup)
	return f
}
func (f *roleControllerFixture) cleanup() {
	if f.cleaned {
		return
	}
	f.cleaned = true
	// Stop all syncs before deleting identity fixtures.
	for _, obj := range f.objects {
		if sync, ok := obj.(*v1.ZitiEntraRoleSync); ok {
			_ = k8sClient.Delete(ctx, sync)
		}
	}
	for _, obj := range f.objects {
		switch obj.(type) {
		case *v1.ZitiEntraRoleSync, *corev1.Namespace, *corev1.Secret:
			continue
		}
		_ = k8sClient.Delete(ctx, obj)
		Eventually(func() bool {
			fresh := obj.DeepCopyObject().(client.Object)
			return apierrors.IsNotFound(k8sClient.Get(ctx, client.ObjectKeyFromObject(obj), fresh))
		}, 10*time.Second, 50*time.Millisecond).Should(BeTrue())
	}
	for _, id := range f.ids {
		fakeOpenZiti.removeRoleFixture(id)
	}
	for _, obj := range f.objects {
		if _, ok := obj.(*corev1.Secret); ok {
			_ = k8sClient.Delete(ctx, obj)
		}
	}
	for _, obj := range f.objects {
		if _, ok := obj.(*corev1.Namespace); ok {
			_ = k8sClient.Delete(ctx, obj)
		}
	}
}
func (f *roleControllerFixture) seed(suffix, external string, attrs ...string) string {
	id := f.namespace + "-" + suffix
	fakeOpenZiti.seedRoleIdentity(openziti.Identity{ID: id,
		Name:           id,
		ExternalID:     external,
		RoleAttributes: attrs,
		Type:           "User"})
	f.ids = append(f.ids, id)
	return id
}
func (f *roleControllerFixture) grant(external string) {
	f.data.assignments = []entra.AppRoleAssignment{{AppRoleID: "beta-role",
		PrincipalType: "Group",
		PrincipalID:   "group"}}
	f.data.users["group"] = append(f.data.users["group"],
		entra.User{ID: external,
			Mail:              external,
			UserPrincipalName: external})
}
func (f *roleControllerFixture) publish() { fakeDirectoryState.replace(f.app, f.data) }
func (f *roleControllerFixture) paused(name, namespace string) *v1.ZitiEntraRoleSync {
	f.publish()
	obj := &v1.ZitiEntraRoleSync{ObjectMeta: metav1.ObjectMeta{Name: name,
		Namespace: namespace},
		Spec: v1.ZitiEntraRoleSyncSpec{TenantID: "00000000-0000-0000-0000-000000000001",
			AppID:                f.app,
			CredentialsSecretRef: corev1.LocalObjectReference{Name: "paused"}}}
	Expect(k8sClient.Create(ctx, obj)).To(Succeed())
	f.objects = append(f.objects, obj)
	return obj
}
func (f *roleControllerFixture) get(obj *v1.ZitiEntraRoleSync) *v1.ZitiEntraRoleSync {
	fresh := &v1.ZitiEntraRoleSync{}
	Eventually(func() error {
		return k8sClient.Get(ctx,
			client.ObjectKeyFromObject(obj),
			fresh)
	},
		5*time.Second,
		50*time.Millisecond).Should(Succeed())
	return fresh
}
func (f *roleControllerFixture) claims(obj *v1.ZitiEntraRoleSync, values ...string) {
	Eventually(func() error {
		current := &v1.ZitiEntraRoleSync{}
		if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(obj), current); err != nil {
			return err
		}
		current.Status.ManagedAttributes = append([]string{}, values...)
		return k8sClient.Status().Update(ctx, current)
	}, 5*time.Second, 50*time.Millisecond).Should(Succeed())
}
func (f *roleControllerFixture) activate(obj *v1.ZitiEntraRoleSync) {
	Eventually(func() error {
		fresh := &v1.ZitiEntraRoleSync{}
		if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(obj), fresh); err != nil {
			return err
		}
		fresh.Spec.CredentialsSecretRef.Name = "credentials"
		return k8sClient.Update(ctx, fresh)
	}, 5*time.Second, 50*time.Millisecond).Should(Succeed())
}
func (f *roleControllerFixture) start(claims ...string) *v1.ZitiEntraRoleSync {
	obj := f.paused("sync", f.namespace)
	f.claims(obj, claims...)
	f.activate(obj)
	return obj
}
func (f *roleControllerFixture) trigger(obj *v1.ZitiEntraRoleSync) {
	f.publish()
	Eventually(func() error {
		fresh := &v1.ZitiEntraRoleSync{}
		if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(obj), fresh); err != nil {
			return err
		}
		if fresh.Spec.Interval == "10m" {
			fresh.Spec.Interval = "11m"
		} else {
			fresh.Spec.Interval = "10m"
		}
		return k8sClient.Update(ctx, fresh)
	}, 5*time.Second, 50*time.Millisecond).Should(Succeed())
}
func (f *roleControllerFixture) reason(obj *v1.ZitiEntraRoleSync) func() string {
	return func() string {
		fresh := &v1.ZitiEntraRoleSync{}
		if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(obj), fresh); err != nil {
			return err.Error()
		}
		ready := apimeta.FindStatusCondition(fresh.Status.Conditions, v1.ConditionTypeReady)
		if ready == nil || ready.ObservedGeneration != fresh.Generation {
			return ""
		}
		return ready.Reason
	}
}
func (f *roleControllerFixture) normalEvents(obj *v1.ZitiEntraRoleSync) func() int {
	return func() int {
		var events corev1.EventList
		Expect(k8sClient.List(ctx, &events, client.InNamespace(obj.Namespace))).To(Succeed())
		total := 0
		for _, event := range events.Items {
			if event.InvolvedObject.UID == obj.UID &&
				event.Type == corev1.EventTypeNormal &&
				event.Reason == "RoleAttributesUpdated" {
				total += int(event.Count)
			}
		}
		return total
	}
}
func roleAttrs(id string) func() []string {
	return func() []string { return fakeOpenZiti.roleAttrs(id) }
}
func rolePatches(id string) func() int               { return func() int { return fakeOpenZiti.rolePatchCount(id) } }
func (f *roleControllerFixture) directoryCalls() int { return fakeDirectoryState.callCount(f.app) }
func (f *roleControllerFixture) tiedPair(namespace string) (*v1.ZitiEntraRoleSync, *v1.ZitiEntraRoleSync) {
	for i := 0; i < 5; i++ {
		loser := f.paused(fmt.Sprintf("zz-%d", i), namespace)
		winner := f.paused(fmt.Sprintf("aa-%d", i), f.namespace)
		if loser.CreationTimestamp.Equal(&winner.CreationTimestamp) {
			return winner, loser
		}
		Expect(k8sClient.Delete(ctx, loser)).To(Succeed())
		Expect(k8sClient.Delete(ctx, winner)).To(Succeed())
	}
	Fail("could not create a timestamp-tied pair")
	return nil, nil
}
