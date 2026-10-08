package integration

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "example.com/miniziti-operator/api/v1alpha1"
)

var _ = Describe("ZitiEntraRoleSync API", func() {
	var ns *corev1.Namespace
	var resource *v1.ZitiEntraRoleSync
	BeforeEach(func() {
		ns = &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{GenerateName: "entra-api-"}}
		Expect(k8sClient.Create(context.Background(), ns)).To(Succeed())
		resource = &v1.ZitiEntraRoleSync{ObjectMeta: metav1.ObjectMeta{Name: "sync", Namespace: ns.Name},
			Spec: v1.ZitiEntraRoleSyncSpec{TenantID: "00000000-0000-0000-0000-000000000001", AppID: "00000000-0000-0000-0000-000000000002", CredentialsSecretRef: corev1.LocalObjectReference{Name: "credentials"}}}
	})
	AfterEach(func() {
		_ = k8sClient.Delete(context.Background(), resource)
		Expect(k8sClient.Delete(context.Background(), ns)).To(Succeed())
	})
	It("defaults userProperty and interval", func() {
		Expect(k8sClient.Create(context.Background(), resource)).To(Succeed())
		Expect(resource.Spec.UserProperty).To(Equal("mail"))
		Expect(resource.Spec.Interval).To(Equal("10m"))
	})
	DescribeTable("rejects invalid configuration", func(change func(*v1.ZitiEntraRoleSyncSpec)) {
		change(&resource.Spec)
		Expect(apierrors.IsInvalid(k8sClient.Create(context.Background(), resource))).To(BeTrue())
	},
		Entry("tenant", func(s *v1.ZitiEntraRoleSyncSpec) { s.TenantID = "not-a-guid" }),
		Entry("app", func(s *v1.ZitiEntraRoleSyncSpec) { s.AppID = "not-a-guid" }),
		Entry("secret", func(s *v1.ZitiEntraRoleSyncSpec) { s.CredentialsSecretRef.Name = "" }),
		Entry("property", func(s *v1.ZitiEntraRoleSyncSpec) { s.UserProperty = "email" }),
		Entry("short interval", func(s *v1.ZitiEntraRoleSyncSpec) { s.Interval = "59s" }),
		Entry("invalid interval", func(s *v1.ZitiEntraRoleSyncSpec) { s.Interval = "nonsense" }),
	)
	DescribeTable("accepts allowed properties and the minimum interval", func(property string) {
		resource.Spec.UserProperty = property
		resource.Spec.Interval = "1m"
		Expect(k8sClient.Create(context.Background(), resource)).To(Succeed())
	}, Entry("mail", "mail"), Entry("UPN", "userPrincipalName"), Entry("object ID", "id"))
})
