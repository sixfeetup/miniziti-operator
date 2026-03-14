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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var zitiServiceGVK = schema.GroupVersionKind{
	Group:   "ziti.sixfeetup.com",
	Version: "v1alpha1",
	Kind:    "ZitiService",
}

func newZitiService(name string) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(zitiServiceGVK)
	obj.SetName(name)
	obj.SetNamespace("default")
	obj.Object["spec"] = map[string]any{
		"name": name,
		"roleAttributes": []any{
			"argocd",
		},
		"configs": map[string]any{
			"intercept": map[string]any{
				"protocols": []any{"tcp"},
				"addresses": []any{"argocd.ziti"},
				"portRanges": []any{
					map[string]any{
						"low":  443,
						"high": 443,
					},
				},
			},
			"host": map[string]any{
				"protocol": "tcp",
				"address":  "argocd-server.argocd.svc.cluster.local",
				"port":     443,
			},
		},
	}
	return obj
}

var _ = Describe("ZitiService controller", func() {
	It("creates a service and reports ready state", func() {
		service := newZitiService("argocd")
		Expect(k8sClient.Create(ctx, service)).To(Succeed())

		Eventually(func(g Gomega) {
			stored := &unstructured.Unstructured{}
			stored.SetGroupVersionKind(zitiServiceGVK)
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(service), stored)).To(Succeed())

			status, found, err := unstructured.NestedMap(stored.Object, "status")
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(found).To(BeTrue())
			g.Expect(status["id"]).NotTo(BeEmpty())
			g.Expect(status["observedGeneration"]).To(Equal(stored.GetGeneration()))
		}, 10*time.Second, 250*time.Millisecond).Should(Succeed())

		var events corev1.EventList
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.List(ctx, &events, client.InNamespace(service.GetNamespace()))).To(Succeed())
			g.Expect(events.Items).NotTo(BeEmpty())
		}, 10*time.Second, 250*time.Millisecond).Should(Succeed())
	})

	It("updates connectivity details in place without changing the service id", func() {
		service := newZitiService("argocd-update")
		Expect(k8sClient.Create(ctx, service)).To(Succeed())

		stored := &unstructured.Unstructured{}
		stored.SetGroupVersionKind(zitiServiceGVK)
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(service), stored)).To(Succeed())
			id, found, err := unstructured.NestedString(stored.Object, "status", "id")
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(found).To(BeTrue())
			g.Expect(id).NotTo(BeEmpty())
		}, 10*time.Second, 250*time.Millisecond).Should(Succeed())

		originalID, _, _ := unstructured.NestedString(stored.Object, "status", "id")
		Expect(unstructured.SetNestedStringSlice(stored.Object, []string{"argocd-new.ziti"}, "spec", "configs", "intercept", "addresses")).To(Succeed())
		Expect(k8sClient.Update(ctx, stored)).To(Succeed())

		Eventually(func(g Gomega) {
			refreshed := &unstructured.Unstructured{}
			refreshed.SetGroupVersionKind(zitiServiceGVK)
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(service), refreshed)).To(Succeed())

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

	It("removes managed configs and the backend service on delete", func() {
		service := newZitiService("argocd-delete")
		Expect(k8sClient.Create(ctx, service)).To(Succeed())

		Eventually(func(g Gomega) {
			stored := &unstructured.Unstructured{}
			stored.SetGroupVersionKind(zitiServiceGVK)
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(service), stored)).To(Succeed())
			g.Expect(stored.GetFinalizers()).NotTo(BeEmpty())
		}, 10*time.Second, 250*time.Millisecond).Should(Succeed())

		Expect(k8sClient.Delete(ctx, service)).To(Succeed())

		Eventually(func(g Gomega) {
			stored := &unstructured.Unstructured{}
			stored.SetGroupVersionKind(zitiServiceGVK)
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(service), stored)).NotTo(Succeed())
		}, 10*time.Second, 250*time.Millisecond).Should(Succeed())
	})

	It("reports degraded status when config reconciliation fails", func() {
		service := newZitiService("argocd-failure")
		unstructured.SetNestedField(service.Object, "fail-config.ziti", "spec", "name")
		Expect(k8sClient.Create(ctx, service)).To(Succeed())

		Eventually(func(g Gomega) {
			stored := &unstructured.Unstructured{}
			stored.SetGroupVersionKind(zitiServiceGVK)
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(service), stored)).To(Succeed())

			lastError, found, err := unstructured.NestedString(stored.Object, "status", "lastError")
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(found).To(BeTrue())
			g.Expect(lastError).NotTo(BeEmpty())
		}, 10*time.Second, 250*time.Millisecond).Should(Succeed())
	})

	It("emits a warning event when config reconciliation fails", func() {
		service := newZitiService("argocd-event")
		unstructured.SetNestedField(service.Object, "fail-event.ziti", "spec", "name")
		Expect(k8sClient.Create(ctx, service)).To(Succeed())

		Eventually(func(g Gomega) {
			var events corev1.EventList
			g.Expect(k8sClient.List(ctx, &events, client.InNamespace(service.GetNamespace()))).To(Succeed())
			g.Expect(events.Items).NotTo(BeEmpty())

			foundWarning := false
			for _, event := range events.Items {
				if event.Type == corev1.EventTypeWarning {
					foundWarning = true
					break
				}
			}
			g.Expect(foundWarning).To(BeTrue())
		}, 10*time.Second, 250*time.Millisecond).Should(Succeed())
	})
})
