package integration

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"

	zitiv1alpha1 "example.com/miniziti-operator/api/v1alpha1"
	openziti "example.com/miniziti-operator/internal/openziti/client"
)

var _ = Describe("allowed edge routers", func() {
	It("preserves additional exact names through admission and readback", func() {
		service := newZitiService("router-admission")
		setAllowedRouters(service, []string{"admission-client"})
		Expect(k8sClient.Create(ctx, service)).To(Succeed())
		DeferCleanup(func() { Expect(k8sClient.Delete(ctx, service)).To(Succeed()) })
		Eventually(func(g Gomega) {
			stored := newUnstructuredWithGVK(zitiServiceGVK)
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(service), stored)).To(Succeed())
			names, found, err := unstructured.NestedStringSlice(stored.Object, "spec", "router", "additionalAllowedEdgeRouterNames")
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(found).To(BeTrue())
			g.Expect(names).To(Equal([]string{"admission-client"}))
		}, 5*time.Second, 100*time.Millisecond).Should(Succeed())
	})

	DescribeTable("rejects blank additional names at admission", func(name, item string) {
		service := newZitiService(name)
		setAllowedRouters(service, []string{item})
		err := k8sClient.Create(ctx, service)
		if err == nil {
			DeferCleanup(func() { Expect(k8sClient.Delete(ctx, service)).To(Succeed()) })
		}
		Expect(apierrors.IsInvalid(err)).To(BeTrue(), "expected an API validation error, got %v", err)
	},
		Entry("empty", "router-blank-empty", ""),
		Entry("whitespace", "router-blank-space", " \t\n "),
	)

	It("updates and removes exact router grants in place while preserving the last good policies on failure", func() {
		// These edge routers deliberately have no matching Router identities.
		fakeOpenZiti.mu.Lock()
		fakeOpenZiti.edgeRouters["allowed-client-2"] = &openziti.EdgeRouter{ID: "allowed-client-2", Name: "allowed-client-two"}
		fakeOpenZiti.edgeRouters["allowed-client-3"] = &openziti.EdgeRouter{ID: "allowed-client-3", Name: "allowed-client-three"}
		fakeOpenZiti.mu.Unlock()

		service := newZitiService("router-lifecycle")
		setAllowedRouters(service, []string{})
		Expect(k8sClient.Create(ctx, service)).To(Succeed())
		DeferCleanup(func() { Expect(k8sClient.Delete(ctx, service)).To(Succeed()) })
		original := awaitAllowedRouterState(service, []string{"@edge-router-1"}, "Reconciled", nil)

		updateAllowedRouters(service, []string{"allowed-client-two", "ziti-prod-router"})
		awaitAllowedRouterState(service, []string{"@allowed-client-2", "@edge-router-1"}, "Reconciled", original)

		// The first router resolves but must not partially replace the prior set.
		updateAllowedRouters(service, []string{"allowed-client-three", "missing-client"})
		awaitAllowedRouterState(service, []string{"@allowed-client-2", "@edge-router-1"}, "RouterResolutionFailed", original)

		updateAllowedRouters(service, []string{"allowed-client-three"})
		awaitAllowedRouterState(service, []string{"@allowed-client-3", "@edge-router-1"}, "Reconciled", original)

		updateAllowedRouters(service, []string{})
		awaitAllowedRouterState(service, []string{"@edge-router-1"}, "Reconciled", original)

		// Omission also removes any previously allowed additional router.
		updateAllowedRouters(service, []string{"allowed-client-two"})
		awaitAllowedRouterState(service, []string{"@allowed-client-2", "@edge-router-1"}, "Reconciled", original)
		updateAllowedRouters(service, nil)
		awaitAllowedRouterState(service, []string{"@edge-router-1"}, "Reconciled", original)
	})
})

func setAllowedRouters(service *unstructured.Unstructured, names []string) {
	Expect(unstructured.SetNestedField(service.Object, "ziti-prod-router", "spec", "router", "name")).To(Succeed())
	if names == nil {
		unstructured.RemoveNestedField(service.Object, "spec", "router", "additionalAllowedEdgeRouterNames")
	} else {
		Expect(unstructured.SetNestedStringSlice(service.Object, names, "spec", "router", "additionalAllowedEdgeRouterNames")).To(Succeed())
	}
}

func updateAllowedRouters(service *unstructured.Unstructured, names []string) {
	Expect(retry.RetryOnConflict(retry.DefaultRetry, func() error {
		stored := newUnstructuredWithGVK(zitiServiceGVK)
		if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(service), stored); err != nil {
			return err
		}
		setAllowedRouters(stored, names)
		return k8sClient.Update(ctx, stored)
	})).To(Succeed())
}

func awaitAllowedRouterState(service client.Object, roles []string, reason string, original *zitiv1alpha1.ZitiServiceStatus) *zitiv1alpha1.ZitiServiceStatus {
	var stored zitiv1alpha1.ZitiService
	Eventually(func(g Gomega) {
		g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(service), &stored)).To(Succeed())
		g.Expect(stored.Status.ObservedGeneration).To(Equal(stored.Generation))
		ready := meta.FindStatusCondition(stored.Status.Conditions, zitiv1alpha1.ConditionTypeReady)
		g.Expect(ready).NotTo(BeNil())
		g.Expect(ready.ObservedGeneration).To(Equal(stored.Generation))
		g.Expect(ready.Reason).To(Equal(reason))
		if reason == "Reconciled" {
			g.Expect(ready.Status).To(Equal(metav1.ConditionTrue))
			g.Expect(stored.Status.LastError).To(BeEmpty())
		} else {
			g.Expect(ready.Status).To(Equal(metav1.ConditionFalse))
			g.Expect(stored.Status.LastError).To(ContainSubstring("missing-client"))
		}
		g.Expect(stored.Status.ID).NotTo(BeEmpty())
		g.Expect(stored.Status.BindPolicyID).NotTo(BeEmpty())
		g.Expect(stored.Status.ServiceEdgeRouterPolicyID).NotTo(BeEmpty())
		if original != nil {
			g.Expect(stored.Status.ID).To(Equal(original.ID))
			g.Expect(stored.Status.ConfigIDs).To(Equal(original.ConfigIDs))
			g.Expect(stored.Status.BindPolicyID).To(Equal(original.BindPolicyID))
			g.Expect(stored.Status.ServiceEdgeRouterPolicyID).To(Equal(original.ServiceEdgeRouterPolicyID))
		}
		fakeOpenZiti.mu.Lock()
		defer fakeOpenZiti.mu.Unlock()
		bind := fakeOpenZiti.accessPolicies[stored.Status.BindPolicyID]
		g.Expect(bind).NotTo(BeNil())
		g.Expect(bind.Type).To(Equal("Bind"))
		g.Expect(bind.IdentityRolesRaw).To(Equal([]string{"@router-identity-1"}))
		g.Expect(bind.ServiceRolesRaw).To(Equal([]string{"@" + stored.Status.ID}))
		policy := fakeOpenZiti.serviceEdgeRouterPolicies[stored.Status.ServiceEdgeRouterPolicyID]
		g.Expect(policy).NotTo(BeNil())
		g.Expect(policy.EdgeRouterRoles).To(Equal(roles))
		g.Expect(policy.ServiceRoles).To(Equal([]string{"@" + stored.Status.ID}))
	}, 10*time.Second, 100*time.Millisecond).Should(Succeed())
	return stored.Status.DeepCopy()
}
