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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var zitiIdentityGVK = schema.GroupVersionKind{
	Group:   "ziti.sixfeetup.com",
	Version: "v1alpha1",
	Kind:    "ZitiIdentity",
}

func newZitiIdentity(name string) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(zitiIdentityGVK)
	obj.SetName(name)
	obj.SetNamespace("default")
	obj.Object["spec"] = map[string]any{
		"name": "alice@example.com",
		"type": "User",
		"roleAttributes": []any{
			"employee",
			"devops",
		},
	}
	return obj
}

var _ = Describe("ZitiIdentity controller", func() {
	It("creates an identity and reports ready state", func() {
		identity := newZitiIdentity("alice")
		Expect(k8sClient.Create(ctx, identity)).To(Succeed())

		stored := identity.DeepCopy()
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(identity), stored)).To(Succeed())

			status, found, err := unstructured.NestedMap(stored.Object, "status")
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(found).To(BeTrue())
			g.Expect(status["id"]).NotTo(BeEmpty())
			g.Expect(status["observedGeneration"]).To(Equal(int64(1)))

			conditions, found, err := unstructured.NestedSlice(stored.Object, "status", "conditions")
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(found).To(BeTrue())
			g.Expect(conditions).NotTo(BeEmpty())
		}, 10*time.Second, 250*time.Millisecond).Should(Succeed())

		var events corev1.EventList
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.List(ctx, &events, client.InNamespace(identity.GetNamespace()))).To(Succeed())
			g.Expect(events.Items).NotTo(BeEmpty())
		}, 10*time.Second, 250*time.Millisecond).Should(Succeed())
	})

	It("updates the existing identity without changing the backend object id", func() {
		identity := newZitiIdentity("alice-update")
		Expect(k8sClient.Create(ctx, identity)).To(Succeed())

		stored := identity.DeepCopy()
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(identity), stored)).To(Succeed())
			id, found, err := unstructured.NestedString(stored.Object, "status", "id")
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(found).To(BeTrue())
			g.Expect(id).NotTo(BeEmpty())
		}, 10*time.Second, 250*time.Millisecond).Should(Succeed())

		originalID, _, _ := unstructured.NestedString(stored.Object, "status", "id")
		Expect(unstructured.SetNestedStringSlice(stored.Object, []string{"employee", "platform"}, "spec", "roleAttributes")).To(Succeed())
		Expect(k8sClient.Update(ctx, stored)).To(Succeed())

		Eventually(func(g Gomega) {
			refreshed := &unstructured.Unstructured{}
			refreshed.SetGroupVersionKind(zitiIdentityGVK)
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(identity), refreshed)).To(Succeed())

			id, found, err := unstructured.NestedString(refreshed.Object, "status", "id")
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(found).To(BeTrue())
			g.Expect(id).To(Equal(originalID))

			observedGeneration, found, err := unstructured.NestedInt64(refreshed.Object, "status", "observedGeneration")
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(found).To(BeTrue())
			g.Expect(observedGeneration).To(Equal(int64(2)))
		}, 10*time.Second, 250*time.Millisecond).Should(Succeed())
	})
})

var _ = BeforeEach(func() {
	// Keep object timestamps deterministic for assertions that depend on generation/status transitions.
	metav1.Now()
})
