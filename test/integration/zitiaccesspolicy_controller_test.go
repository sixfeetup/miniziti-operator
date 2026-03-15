/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package integration

import (
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var zitiAccessPolicyGVK = schema.GroupVersionKind{
	Group:   "ziti.sixfeetup.com",
	Version: "v1alpha1",
	Kind:    "ZitiAccessPolicy",
}

func newZitiAccessPolicy(name string) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(zitiAccessPolicyGVK)
	obj.SetName(name)
	obj.SetNamespace("default")
	obj.Object["spec"] = map[string]any{
		"type": "Dial",
		"identitySelector": map[string]any{
			"matchRoleAttributes": []any{"devops"},
		},
		"serviceSelector": map[string]any{
			"matchNames": []any{"argocd"},
		},
	}
	return obj
}

func createReadyIdentity(k8sName, zitiName string, roleAttributes ...string) *unstructured.Unstructured {
	identity := newZitiIdentity(k8sName)
	Expect(unstructured.SetNestedField(identity.Object, zitiName, "spec", "name")).To(Succeed())
	Expect(unstructured.SetNestedStringSlice(identity.Object, roleAttributes, "spec", "roleAttributes")).To(Succeed())
	Expect(k8sClient.Create(ctx, identity)).To(Succeed())

	Eventually(func(g Gomega) {
		stored := &unstructured.Unstructured{}
		stored.SetGroupVersionKind(zitiIdentityGVK)
		g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(identity), stored)).To(Succeed())
		id, found, err := unstructured.NestedString(stored.Object, "status", "id")
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(found).To(BeTrue())
		g.Expect(id).NotTo(BeEmpty())
	}, 10*time.Second, 250*time.Millisecond).Should(Succeed())

	return identity
}

func createReadyService(k8sName, zitiName string, roleAttributes ...string) *unstructured.Unstructured {
	service := newZitiService(k8sName)
	Expect(unstructured.SetNestedField(service.Object, zitiName, "spec", "name")).To(Succeed())
	Expect(unstructured.SetNestedStringSlice(service.Object, roleAttributes, "spec", "roleAttributes")).To(Succeed())
	Expect(k8sClient.Create(ctx, service)).To(Succeed())

	Eventually(func(g Gomega) {
		stored := &unstructured.Unstructured{}
		stored.SetGroupVersionKind(zitiServiceGVK)
		g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(service), stored)).To(Succeed())
		id, found, err := unstructured.NestedString(stored.Object, "status", "id")
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(found).To(BeTrue())
		g.Expect(id).NotTo(BeEmpty())
	}, 10*time.Second, 250*time.Millisecond).Should(Succeed())

	return service
}

var _ = Describe("ZitiAccessPolicy controller", func() {
	It("creates a dial policy from selectors and reports ready state", func() {
		createReadyIdentity("alice-policy-create", "alice-policy-create@example.com", "employee", "devops")
		createReadyService("argocd-policy-create", "argocd", "argocd")

		policy := newZitiAccessPolicy("argocd-devops-dial")
		Expect(k8sClient.Create(ctx, policy)).To(Succeed())

		Eventually(func(g Gomega) {
			stored := &unstructured.Unstructured{}
			stored.SetGroupVersionKind(zitiAccessPolicyGVK)
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(policy), stored)).To(Succeed())

			status, found, err := unstructured.NestedMap(stored.Object, "status")
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(found).To(BeTrue())
			g.Expect(status["id"]).NotTo(BeEmpty())
			g.Expect(status["observedGeneration"]).To(Equal(stored.GetGeneration()))
		}, 10*time.Second, 250*time.Millisecond).Should(Succeed())

		var events corev1.EventList
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.List(ctx, &events, client.InNamespace(policy.GetNamespace()))).To(Succeed())
			g.Expect(events.Items).NotTo(BeEmpty())
		}, 10*time.Second, 250*time.Millisecond).Should(Succeed())
	})

	It("updates selector criteria in place without changing the backend policy id", func() {
		createReadyIdentity("alice-policy-update", "alice-policy-update@example.com", "employee", "devops")
		createReadyIdentity("bob-policy-update", "bob-policy-update@example.com", "employee", "platform")
		createReadyService("argocd-policy-update", "argocd-update", "argocd")
		createReadyService("argocd-policy-alt", "argocd-alt", "gitops")

		policy := newZitiAccessPolicy("argocd-platform-dial")
		Expect(unstructured.SetNestedStringSlice(policy.Object, []string{"argocd-update"}, "spec", "serviceSelector", "matchNames")).To(Succeed())
		Expect(k8sClient.Create(ctx, policy)).To(Succeed())

		stored := &unstructured.Unstructured{}
		stored.SetGroupVersionKind(zitiAccessPolicyGVK)
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(policy), stored)).To(Succeed())
			id, found, err := unstructured.NestedString(stored.Object, "status", "id")
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(found).To(BeTrue())
			g.Expect(id).NotTo(BeEmpty())
		}, 10*time.Second, 250*time.Millisecond).Should(Succeed())

		originalID, _, _ := unstructured.NestedString(stored.Object, "status", "id")
		Expect(unstructured.SetNestedStringSlice(stored.Object, []string{"platform"}, "spec", "identitySelector", "matchRoleAttributes")).To(Succeed())
		Expect(unstructured.SetNestedStringSlice(stored.Object, []string{"argocd-alt"}, "spec", "serviceSelector", "matchNames")).To(Succeed())
		Expect(k8sClient.Update(ctx, stored)).To(Succeed())

		Eventually(func(g Gomega) {
			refreshed := &unstructured.Unstructured{}
			refreshed.SetGroupVersionKind(zitiAccessPolicyGVK)
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(policy), refreshed)).To(Succeed())

			id, found, err := unstructured.NestedString(refreshed.Object, "status", "id")
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(found).To(BeTrue())
			g.Expect(id).To(Equal(originalID))

			observedGeneration, found, err := unstructured.NestedInt64(refreshed.Object, "status", "observedGeneration")
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(found).To(BeTrue())
			g.Expect(observedGeneration).To(Equal(refreshed.GetGeneration()))
		}, 10*time.Second, 250*time.Millisecond).Should(Succeed())
	})

	It("reports degraded status when selectors match zero identities or services", func() {
		policy := newZitiAccessPolicy("argocd-zero-match")
		Expect(unstructured.SetNestedStringSlice(policy.Object, []string{"nobody"}, "spec", "identitySelector", "matchRoleAttributes")).To(Succeed())
		Expect(unstructured.SetNestedStringSlice(policy.Object, []string{"missing-service"}, "spec", "serviceSelector", "matchNames")).To(Succeed())
		Expect(k8sClient.Create(ctx, policy)).To(Succeed())

		Eventually(func(g Gomega) {
			stored := &unstructured.Unstructured{}
			stored.SetGroupVersionKind(zitiAccessPolicyGVK)
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(policy), stored)).To(Succeed())

			lastError, found, err := unstructured.NestedString(stored.Object, "status", "lastError")
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(found).To(BeTrue())
			g.Expect(lastError).To(ContainSubstring("selector"))
		}, 10*time.Second, 250*time.Millisecond).Should(Succeed())
	})

	It("retries transient backend failures until the policy converges", func() {
		createReadyIdentity("alice-policy-retry", "alice-policy-retry@example.com", "employee", "devops")
		createReadyService("argocd-policy-retry", "argocd-retry", "argocd")

		policy := newZitiAccessPolicy("argocd-retry")
		Expect(unstructured.SetNestedStringSlice(policy.Object, []string{"argocd-retry"}, "spec", "serviceSelector", "matchNames")).To(Succeed())
		Expect(k8sClient.Create(ctx, policy)).To(Succeed())

		Eventually(func(g Gomega) {
			stored := &unstructured.Unstructured{}
			stored.SetGroupVersionKind(zitiAccessPolicyGVK)
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(policy), stored)).To(Succeed())

			id, found, err := unstructured.NestedString(stored.Object, "status", "id")
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(found).To(BeTrue())
			g.Expect(id).NotTo(BeEmpty())

			lastError, _, err := unstructured.NestedString(stored.Object, "status", "lastError")
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(lastError).To(BeEmpty())
		}, 15*time.Second, 250*time.Millisecond).Should(Succeed())
	})

	It("removes the managed policy on delete", func() {
		createReadyIdentity("alice-policy-delete", "alice-policy-delete@example.com", "employee", "devops")
		createReadyService("argocd-policy-delete", "argocd-delete-policy", "argocd")

		policy := newZitiAccessPolicy("argocd-delete")
		Expect(unstructured.SetNestedStringSlice(policy.Object, []string{"argocd-delete-policy"}, "spec", "serviceSelector", "matchNames")).To(Succeed())
		Expect(k8sClient.Create(ctx, policy)).To(Succeed())

		Eventually(func(g Gomega) {
			stored := &unstructured.Unstructured{}
			stored.SetGroupVersionKind(zitiAccessPolicyGVK)
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(policy), stored)).To(Succeed())
			g.Expect(stored.GetFinalizers()).NotTo(BeEmpty())
		}, 10*time.Second, 250*time.Millisecond).Should(Succeed())

		Expect(k8sClient.Delete(ctx, policy)).To(Succeed())

		Eventually(func(g Gomega) {
			stored := &unstructured.Unstructured{}
			stored.SetGroupVersionKind(zitiAccessPolicyGVK)
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(policy), stored)).NotTo(Succeed())
		}, 10*time.Second, 250*time.Millisecond).Should(Succeed())
	})

	It("reports degraded status when backend policy sync fails", func() {
		createReadyIdentity("alice-policy-failure", "alice-policy-failure@example.com", "employee", "devops")
		createReadyService("argocd-policy-failure", "argocd-failure-policy", "argocd")

		policy := newZitiAccessPolicy("fail-status-policy")
		Expect(unstructured.SetNestedStringSlice(policy.Object, []string{"argocd-failure-policy"}, "spec", "serviceSelector", "matchNames")).To(Succeed())
		Expect(k8sClient.Create(ctx, policy)).To(Succeed())

		Eventually(func(g Gomega) {
			stored := &unstructured.Unstructured{}
			stored.SetGroupVersionKind(zitiAccessPolicyGVK)
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(policy), stored)).To(Succeed())

			lastError, found, err := unstructured.NestedString(stored.Object, "status", "lastError")
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(found).To(BeTrue())
			g.Expect(lastError).NotTo(BeEmpty())
		}, 10*time.Second, 250*time.Millisecond).Should(Succeed())
	})

	It("emits a warning event when backend policy sync fails", func() {
		createReadyIdentity("alice-policy-event", "alice-policy-event@example.com", "employee", "devops")
		createReadyService("argocd-policy-event", "argocd-event-policy", "argocd")

		policy := newZitiAccessPolicy("fail-event-policy")
		Expect(unstructured.SetNestedStringSlice(policy.Object, []string{"argocd-event-policy"}, "spec", "serviceSelector", "matchNames")).To(Succeed())
		Expect(k8sClient.Create(ctx, policy)).To(Succeed())

		Eventually(func(g Gomega) {
			var events corev1.EventList
			g.Expect(k8sClient.List(ctx, &events, client.InNamespace(policy.GetNamespace()))).To(Succeed())
			g.Expect(events.Items).NotTo(BeEmpty())

			foundWarning := false
			for _, event := range events.Items {
				if event.Type == corev1.EventTypeWarning && event.InvolvedObject.Name == policy.GetName() {
					foundWarning = true
					break
				}
			}
			g.Expect(foundWarning).To(BeTrue(), fmt.Sprintf("expected warning event for %s", policy.GetName()))
		}, 10*time.Second, 250*time.Millisecond).Should(Succeed())
	})
})
