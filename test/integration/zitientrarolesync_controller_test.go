package integration

import (
	"errors"
	"slices"
	"sync"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "example.com/miniziti-operator/api/v1alpha1"
	"example.com/miniziti-operator/internal/entra"
	openziti "example.com/miniziti-operator/internal/openziti/client"
)

var _ = Describe("ZitiEntraRoleSync controller", func() {
	It("syncs group roles without changing unmanaged attributes", func() {
		f := newRoleControllerFixture()
		id := f.seed("alice", "alice@example.com", "custom", "alpha")
		f.grant("alice@example.com")
		obj := f.start("alpha")
		Eventually(roleAttrs(id), 10*time.Second, 50*time.Millisecond).Should(Equal([]string{"custom", "beta"}))
		Eventually(f.reason(obj), 10*time.Second, 50*time.Millisecond).Should(Equal(integrationRoleSynced))
		stored := f.get(obj)
		Expect(fakeDirectoryState.assignmentReads(f.app)).To(BeNumerically(">", 0))
		Expect(fakeDirectoryState.groupReads(f.app)).To(BeNumerically(">", 0))
		Expect(stored.Status.ManagedAttributes).To(Equal([]string{"beta"}))
		Expect(apimeta.FindStatusCondition(stored.Status.Conditions,
			v1.ConditionTypeReady).Status).To(Equal(metav1.ConditionTrue))
		Eventually(f.normalEvents(obj),
			10*time.Second,
			50*time.Millisecond).Should(Equal(fakeOpenZiti.rolePatchCount(id)))
	})

	It("does not compete with ZitiIdentity by ID or adoption name", func() {
		for _, adopt := range []bool{false, true} {
			f := newRoleControllerFixture()
			id := f.namespace + "-owned"
			external := f.namespace + "@example.com"
			owner := &v1.ZitiIdentity{ObjectMeta: metav1.ObjectMeta{Name: "owned",
				Namespace: f.namespace},
				Spec: v1.ZitiIdentitySpec{Type: "User",
					Name:           id,
					RoleAttributes: []string{"manual"}}}
			var release func()
			if adopt {
				f.seed("owned", external, "manual")
				gate := make(chan struct{})
				var once sync.Once
				fakeOpenZiti.setRoleLookupGate(id, gate)
				release = func() {
					once.Do(func() { close(gate) })
					fakeOpenZiti.setRoleLookupGate(id,
						nil)
				}
				DeferCleanup(release)
			}
			Expect(k8sClient.Create(ctx, owner)).To(Succeed())
			f.objects = append(f.objects, owner)
			if !adopt {
				Eventually(func() string {
					fresh := &v1.ZitiIdentity{}
					_ = k8sClient.Get(ctx, client.ObjectKeyFromObject(owner), fresh)
					return fresh.Status.ID
				}, 10*time.Second, 50*time.Millisecond).ShouldNot(BeEmpty())
				fresh := &v1.ZitiIdentity{}
				Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(owner), fresh)).To(Succeed())
				id = fresh.Status.ID
				f.ids = append(f.ids, id)
				fakeOpenZiti.seedRoleIdentity(openziti.Identity{ID: id,
					Name:           owner.Spec.Name,
					ExternalID:     external,
					RoleAttributes: []string{"manual"}})
			}
			f.grant(external)
			obj := f.start()
			Eventually(f.reason(obj),
				10*time.Second,
				50*time.Millisecond).Should(Equal(integrationRoleSynced))
			Consistently(rolePatches(id), 500*time.Millisecond, 50*time.Millisecond).Should(BeZero())
			if adopt {
				fresh := &v1.ZitiIdentity{}
				Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(owner), fresh)).To(Succeed())
				Expect(fresh.Status.ID).To(BeEmpty())
				release()
				Eventually(func() string {
					fresh := &v1.ZitiIdentity{}
					_ = k8sClient.Get(ctx, client.ObjectKeyFromObject(owner), fresh)
					return fresh.Status.ID
				}, 10*time.Second, 50*time.Millisecond).Should(Equal(id))
			}
			// Run both again with conflicting desired attributes.
			fresh := &v1.ZitiIdentity{}
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(owner), fresh)).To(Succeed())
			fresh.Spec.RoleAttributes = []string{"owner-change"}
			Expect(k8sClient.Update(ctx, fresh)).To(Succeed())
			f.trigger(obj)
			Eventually(roleAttrs(id),
				10*time.Second,
				50*time.Millisecond).Should(Equal([]string{"owner-change"}))
			Consistently(rolePatches(id), 500*time.Millisecond, 50*time.Millisecond).Should(BeZero())
			f.cleanup()
		}
	})

	It("keeps retirement history while duplicates remain", func() {
		f := newRoleControllerFixture()
		dup1 := f.seed("dup1", "duplicate@example.com", "alpha")
		dup2 := f.seed("dup2", "DUPLICATE@example.com", "alpha")
		keeper := f.seed("keeper", "keeper@example.com", "beta")
		f.grant("keeper@example.com")
		obj := f.start("alpha", "beta")
		Eventually(f.reason(obj), 10*time.Second, 50*time.Millisecond).Should(Equal(integrationRolePending))
		before := f.get(obj)
		Expect(before.Status.ManagedAttributes).To(Equal([]string{"alpha", "beta"}))
		Expect(before.Status.LastSyncTime).To(BeNil())
		fakeOpenZiti.seedRoleIdentity(openziti.Identity{ID: dup2,
			Name:           dup2,
			ExternalID:     "resolved@example.com",
			RoleAttributes: []string{"alpha"}})
		f.trigger(obj)
		Eventually(roleAttrs(dup1), 10*time.Second, 50*time.Millisecond).Should(BeEmpty())
		Eventually(roleAttrs(dup2), 10*time.Second, 50*time.Millisecond).Should(BeEmpty())
		Eventually(f.reason(obj), 10*time.Second, 50*time.Millisecond).Should(Equal(integrationRoleSynced))
		Expect(f.get(obj).Status.ManagedAttributes).To(Equal([]string{"beta"}))
		Expect(fakeOpenZiti.roleAttrs(keeper)).To(Equal([]string{"beta"}))
	})

	It("recovers overlapping recorded claims with one winner", func() {
		for _, namespaceTie := range []bool{false, true} {
			f := newRoleControllerFixture()
			id := f.seed("alice", "alice@example.com", "custom")
			f.grant("alice@example.com")
			loserNS := f.namespace
			if namespaceTie {
				ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{GenerateName: "zz-entra-sync-"}}
				Expect(k8sClient.Create(ctx, ns)).To(Succeed())
				f.objects = append(f.objects, ns)
				loserNS = ns.Name
				secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "credentials",
					Namespace: ns.Name},
					Data: map[string][]byte{"clientId": []byte("reader"),
						"clientSecret": []byte("initial-secret")}}
				Expect(k8sClient.Create(ctx, secret)).To(Succeed())
				f.objects = append(f.objects, secret)
			}
			winner, loser := f.tiedPair(loserNS)
			f.claims(winner, "beta")
			f.claims(loser, "beta")
			f.activate(winner)
			f.activate(loser)
			Eventually(f.reason(winner),
				10*time.Second,
				50*time.Millisecond).Should(Equal(integrationRoleSynced))
			Eventually(f.reason(loser),
				10*time.Second,
				50*time.Millisecond).Should(Equal(integrationRoleConflict))
			Expect(fakeOpenZiti.rolePatchCount(id)).To(BeNumerically(">", 0))
			Consistently(f.reason(loser),
				500*time.Millisecond,
				50*time.Millisecond).Should(Equal(integrationRoleConflict))
			// Put the backend out of sync, then trigger the loser alone.
			before := fakeOpenZiti.rolePatchCount(id)
			fakeOpenZiti.seedRoleIdentity(openziti.Identity{ID: id,
				Name:           id,
				ExternalID:     "alice@example.com",
				RoleAttributes: []string{"custom"}})
			f.trigger(loser)
			Eventually(f.reason(loser), 10*time.Second, 50*time.Millisecond).Should(Equal(integrationRoleConflict))
			Consistently(rolePatches(id), 500*time.Millisecond, 50*time.Millisecond).Should(Equal(before))
			f.cleanup()
		}
	})

	It("keeps the first recorded owner despite an unrecorded contender", func() {
		f := newRoleControllerFixture()
		id := f.seed("alice", "alice@example.com", "custom")
		f.grant("alice@example.com")
		contender := f.paused("older-contender", f.namespace)
		owner := f.paused("owner", f.namespace)
		f.activate(owner)
		Eventually(f.reason(owner), 10*time.Second, 50*time.Millisecond).Should(Equal(integrationRoleSynced))
		before := fakeOpenZiti.rolePatchCount(id)
		f.activate(contender)
		Eventually(f.reason(contender),
			10*time.Second,
			50*time.Millisecond).Should(Equal(integrationRoleConflict))
		Expect(f.get(contender).Status.ManagedAttributes).To(BeEmpty())
		Consistently(rolePatches(id), 500*time.Millisecond, 50*time.Millisecond).Should(Equal(before))
	})

	It("retains claims across partial PATCH failure", func() {
		f := newRoleControllerFixture()
		good := f.seed("good", "good@example.com", "alpha")
		bad := f.seed("bad", "bad@example.com", "alpha")
		f.grant("good@example.com")
		f.grant("bad@example.com")
		fakeOpenZiti.failRolePatch(bad, errors.New("injected PATCH failure"))
		obj := f.start("alpha")
		Eventually(f.reason(obj), 10*time.Second, 50*time.Millisecond).Should(Equal(integrationRoleZitiError))
		stored := f.get(obj)
		Expect(stored.Status.ManagedAttributes).To(Equal([]string{"alpha", "beta"}))
		Expect(stored.Status.IdentitiesUpdated).To(Equal(1))
		Expect(fakeOpenZiti.roleAttrs(good)).To(Equal([]string{"beta"}))
		Expect(fakeOpenZiti.roleAttrs(bad)).To(Equal([]string{"alpha"}))
		fakeOpenZiti.failRolePatch(bad, nil)
		f.trigger(obj)
		Eventually(f.reason(obj), 10*time.Second, 50*time.Millisecond).Should(Equal(integrationRoleSynced))
		Expect(f.get(obj).Status.ManagedAttributes).To(Equal([]string{"beta"}))
	})

	It("makes no writes for invalid reads or remove-all plans", func() {
		for _, reason := range []string{"InvalidSpec", integrationRoleGraphError, "WouldRemoveAll"} {
			f := newRoleControllerFixture()
			id := f.seed("alice", "alice@example.com", "beta")
			if reason == integrationRoleGraphError {
				f.data.err = &entra.GraphError{StatusCode: 403, Message: "permission denied"}
			}
			obj := f.paused("sync", f.namespace)
			if reason != "InvalidSpec" {
				f.activate(obj)
			}
			Eventually(f.reason(obj), 10*time.Second, 50*time.Millisecond).Should(Equal(reason))
			Consistently(rolePatches(id), 500*time.Millisecond, 50*time.Millisecond).Should(BeZero())
			Expect(fakeOpenZiti.roleAttrs(id)).To(Equal([]string{"beta"}))
			f.cleanup()
		}
	})

	It("responds to spec changes without a status feedback loop", func() {
		f := newRoleControllerFixture()
		f.seed("alice", "alice@example.com", "custom")
		f.grant("alice@example.com")
		obj := f.start()
		Eventually(f.reason(obj), 10*time.Second, 50*time.Millisecond).Should(Equal(integrationRoleSynced))
		before := f.directoryCalls()
		f.trigger(obj)
		Eventually(f.directoryCalls, 10*time.Second, 50*time.Millisecond).Should(BeNumerically(">", before))
		Eventually(f.reason(obj), 10*time.Second, 50*time.Millisecond).Should(Equal(integrationRoleSynced))
		settled := f.directoryCalls()
		fresh := f.get(obj)
		fresh.Status.LastError = "status-only"
		Expect(k8sClient.Status().Update(ctx, fresh)).To(Succeed())
		Consistently(f.directoryCalls, 500*time.Millisecond, 50*time.Millisecond).Should(Equal(settled))
	})

	It("uses the next interval to read changed credentials", func() {
		f := newRoleControllerFixture()
		f.seed("alice", "alice@example.com", "custom")
		f.grant("alice@example.com")
		obj := f.paused("sync", f.namespace)
		Eventually(f.reason(obj), 10*time.Second, 50*time.Millisecond).Should(Equal("InvalidSpec"))
		fresh := f.get(obj)
		fresh.Spec.Interval = "1m"
		Expect(k8sClient.Update(ctx, fresh)).To(Succeed())
		Eventually(func() int64 {
			stored := f.get(obj)
			return stored.Status.ObservedGeneration
		}, 10*time.Second, 50*time.Millisecond).Should(Equal(fresh.Generation))
		f.activate(obj)
		Eventually(f.reason(obj), 10*time.Second, 50*time.Millisecond).Should(Equal(integrationRoleSynced))
		before := f.directoryCalls()
		secret := &corev1.Secret{}
		Expect(k8sClient.Get(ctx,
			client.ObjectKey{Namespace: f.namespace,
				Name: "credentials"},
			secret)).To(Succeed())
		rotatedAt := time.Now()
		secret.Data["clientSecret"] = []byte("rotated-secret")
		Expect(k8sClient.Update(ctx, secret)).To(Succeed())
		Consistently(f.directoryCalls, 500*time.Millisecond, 50*time.Millisecond).Should(Equal(before))
		Eventually(func() string { return fakeDirectoryState.lastSecret(f.app) },
			90*time.Second,
			100*time.Millisecond).Should(Equal("rotated-secret"))
		Expect(time.Since(rotatedAt)).To(BeNumerically(">=", 55*time.Second))
	})

	It("stops without cleanup when deleted", func() {
		f := newRoleControllerFixture()
		id := f.seed("alice", "alice@example.com", "custom")
		f.grant("alice@example.com")
		obj := f.start()
		Eventually(f.reason(obj), 10*time.Second, 50*time.Millisecond).Should(Equal(integrationRoleSynced))
		stored := f.get(obj)
		Expect(stored.Finalizers).To(BeEmpty())
		original := fakeOpenZiti.roleAttrs(id)
		before := fakeOpenZiti.rolePatchCount(id)
		Expect(k8sClient.Delete(ctx, stored)).To(Succeed())
		Consistently(rolePatches(id), 500*time.Millisecond, 50*time.Millisecond).Should(Equal(before))
		Expect(fakeOpenZiti.roleAttrs(id)).To(Equal(original))
		Expect(slices.Contains(original, "beta")).To(BeTrue())
	})
})
