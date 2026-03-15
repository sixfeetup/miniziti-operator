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
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	zitiv1alpha1 "example.com/miniziti-operator/api/v1alpha1"
	"example.com/miniziti-operator/internal/controller"
	"example.com/miniziti-operator/internal/credentials"
	openziti "example.com/miniziti-operator/internal/openziti/client"
	identityservice "example.com/miniziti-operator/internal/openziti/identity"
	policyservice "example.com/miniziti-operator/internal/openziti/policy"
	serviceservice "example.com/miniziti-operator/internal/openziti/service"
)

var (
	testEnv   *envtest.Environment
	k8sClient client.Client
	ctx       context.Context
	cancel    context.CancelFunc
)

type fakeOpenZitiClient struct {
	mu             sync.Mutex
	nextIdentity   int
	nextService    int
	nextConfig     int
	nextPolicy     int
	identities     map[string]*openziti.Identity
	services       map[string]*openziti.Service
	serviceConfigs map[string]*openziti.ServiceConfig
	accessPolicies map[string]*openziti.AccessPolicy
	policyFailures map[string]int
}

func newFakeOpenZitiClient() *fakeOpenZitiClient {
	return &fakeOpenZitiClient{
		nextIdentity:   1,
		nextService:    1,
		nextConfig:     1,
		nextPolicy:     1,
		identities:     map[string]*openziti.Identity{},
		services:       map[string]*openziti.Service{},
		serviceConfigs: map[string]*openziti.ServiceConfig{},
		accessPolicies: map[string]*openziti.AccessPolicy{},
		policyFailures: map[string]int{},
	}
}

func (f *fakeOpenZitiClient) Authenticate(context.Context, credentials.ManagementConfig) error {
	return nil
}

func (f *fakeOpenZitiClient) GetIdentity(_ context.Context, id string) (*openziti.Identity, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if identity, ok := f.identities[id]; ok {
		copy := *identity
		return &copy, nil
	}
	return nil, nil
}

func (f *fakeOpenZitiClient) FindIdentityByName(_ context.Context, name string) (*openziti.Identity, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, identity := range f.identities {
		if identity.Name == name {
			copy := *identity
			return &copy, nil
		}
	}
	return nil, nil
}

func (f *fakeOpenZitiClient) CreateIdentity(_ context.Context, identity openziti.Identity) (*openziti.Identity, error) {
	if err := fakeIdentityFailure(identity.Name); err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	identity.ID = fmt.Sprintf("identity-%d", f.nextIdentity)
	f.nextIdentity++
	copy := identity
	f.identities[identity.ID] = &copy
	return &identity, nil
}

func (f *fakeOpenZitiClient) UpdateIdentity(_ context.Context, identity openziti.Identity) (*openziti.Identity, error) {
	if err := fakeIdentityFailure(identity.Name); err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	copy := identity
	f.identities[identity.ID] = &copy
	return &identity, nil
}

func (f *fakeOpenZitiClient) DeleteIdentity(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.identities, id)
	return nil
}

func (f *fakeOpenZitiClient) GetEnrollmentJWT(_ context.Context, id string) (string, error) {
	return "jwt-for-" + id, nil
}

func (f *fakeOpenZitiClient) GetService(_ context.Context, id string) (*openziti.Service, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.getServiceLocked(id)
}

func (f *fakeOpenZitiClient) FindServiceByName(_ context.Context, name string) (*openziti.Service, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, service := range f.services {
		if service.Name == name {
			copy := *service
			return &copy, nil
		}
	}
	return nil, nil
}

func (f *fakeOpenZitiClient) CreateService(_ context.Context, service openziti.Service) (*openziti.Service, error) {
	if err := fakeServiceFailure(service.Name); err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	service.ID = fmt.Sprintf("service-%d", f.nextService)
	f.nextService++
	copy := service
	f.services[service.ID] = &copy
	return &service, nil
}

func (f *fakeOpenZitiClient) UpdateService(_ context.Context, service openziti.Service) (*openziti.Service, error) {
	if err := fakeServiceFailure(service.Name); err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.services[service.ID]; !ok {
		return nil, nil
	}
	copy := service
	f.services[service.ID] = &copy
	return &service, nil
}

func (f *fakeOpenZitiClient) DeleteService(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.services, id)
	return nil
}

func (f *fakeOpenZitiClient) GetConfig(_ context.Context, id string) (*openziti.ServiceConfig, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if cfg, ok := f.serviceConfigs[id]; ok {
		copy := *cfg
		copy.Payload = clonePayload(cfg.Payload)
		return &copy, nil
	}
	return nil, nil
}

func (f *fakeOpenZitiClient) CreateConfig(_ context.Context, cfg openziti.ServiceConfig) (*openziti.ServiceConfig, error) {
	if err := fakeConfigFailure(cfg.Name); err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	cfg.ID = fmt.Sprintf("config-%d", f.nextConfig)
	f.nextConfig++
	copy := cfg
	copy.Payload = clonePayload(cfg.Payload)
	f.serviceConfigs[cfg.ID] = &copy
	result := copy
	return &result, nil
}

func (f *fakeOpenZitiClient) UpdateConfig(_ context.Context, cfg openziti.ServiceConfig) (*openziti.ServiceConfig, error) {
	if err := fakeConfigFailure(cfg.Name); err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.serviceConfigs[cfg.ID]; !ok {
		return nil, nil
	}
	copy := cfg
	copy.Payload = clonePayload(cfg.Payload)
	f.serviceConfigs[cfg.ID] = &copy
	result := copy
	return &result, nil
}

func (f *fakeOpenZitiClient) DeleteConfig(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.serviceConfigs, id)
	return nil
}

func (f *fakeOpenZitiClient) GetAccessPolicy(_ context.Context, id string) (*openziti.AccessPolicy, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if policy, ok := f.accessPolicies[id]; ok {
		copy := *policy
		copy.IdentityRoles = append([]string(nil), policy.IdentityRoles...)
		copy.ServiceRoles = append([]string(nil), policy.ServiceRoles...)
		copy.IdentityRolesRaw = append([]string(nil), policy.IdentityRolesRaw...)
		copy.ServiceRolesRaw = append([]string(nil), policy.ServiceRolesRaw...)
		return &copy, nil
	}
	return nil, nil
}

func (f *fakeOpenZitiClient) FindAccessPolicyByName(_ context.Context, name string) (*openziti.AccessPolicy, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, policy := range f.accessPolicies {
		if policy.Name == name {
			copy := *policy
			copy.IdentityRoles = append([]string(nil), policy.IdentityRoles...)
			copy.ServiceRoles = append([]string(nil), policy.ServiceRoles...)
			copy.IdentityRolesRaw = append([]string(nil), policy.IdentityRolesRaw...)
			copy.ServiceRolesRaw = append([]string(nil), policy.ServiceRolesRaw...)
			return &copy, nil
		}
	}
	return nil, nil
}

func (f *fakeOpenZitiClient) CreateAccessPolicy(_ context.Context, policy openziti.AccessPolicy) (*openziti.AccessPolicy, error) {
	if err := f.fakePolicyFailure(policy.Name); err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	policy.ID = fmt.Sprintf("policy-%d", f.nextPolicy)
	f.nextPolicy++
	copy := cloneAccessPolicy(policy)
	f.accessPolicies[policy.ID] = &copy
	return &copy, nil
}

func (f *fakeOpenZitiClient) UpdateAccessPolicy(_ context.Context, policy openziti.AccessPolicy) (*openziti.AccessPolicy, error) {
	if err := f.fakePolicyFailure(policy.Name); err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.accessPolicies[policy.ID]; !ok {
		return nil, nil
	}
	copy := cloneAccessPolicy(policy)
	f.accessPolicies[policy.ID] = &copy
	return &copy, nil
}

func (f *fakeOpenZitiClient) DeleteAccessPolicy(_ context.Context, id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.accessPolicies, id)
	return nil
}

func fakeIdentityFailure(name string) error {
	if strings.Contains(name, "fail-status") || strings.Contains(name, "fail-event") {
		return fmt.Errorf("simulated backend failure for %s", name)
	}
	return nil
}

func fakeServiceFailure(name string) error {
	if strings.Contains(name, "fail-service") {
		return fmt.Errorf("simulated service failure for %s", name)
	}
	return nil
}

func fakeConfigFailure(name string) error {
	if strings.Contains(name, "fail-config") || strings.Contains(name, "fail-event") {
		return fmt.Errorf("simulated config failure for %s", name)
	}
	return nil
}

func (f *fakeOpenZitiClient) fakePolicyFailure(name string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if strings.Contains(name, "fail-status") || strings.Contains(name, "fail-event") {
		return fmt.Errorf("simulated policy failure for %s", name)
	}
	if strings.Contains(name, "retry") {
		failures := f.policyFailures[name]
		if failures == 0 {
			f.policyFailures[name] = 1
			return fmt.Errorf("simulated transient policy failure for %s", name)
		}
	}
	return nil
}

func (f *fakeOpenZitiClient) getServiceLocked(id string) (*openziti.Service, error) {
	if service, ok := f.services[id]; ok {
		copy := *service
		return &copy, nil
	}
	return nil, nil
}

func clonePayload(payload map[string]any) map[string]any {
	if payload == nil {
		return nil
	}
	copy := make(map[string]any, len(payload))
	for key, value := range payload {
		switch typed := value.(type) {
		case []string:
			copy[key] = append([]string(nil), typed...)
		case []map[string]int32:
			cloned := make([]map[string]int32, 0, len(typed))
			for _, item := range typed {
				cloned = append(cloned, map[string]int32{
					"low":  item["low"],
					"high": item["high"],
				})
			}
			copy[key] = cloned
		default:
			copy[key] = value
		}
	}
	return copy
}

func cloneAccessPolicy(policy openziti.AccessPolicy) openziti.AccessPolicy {
	policy.IdentityRoles = append([]string(nil), policy.IdentityRoles...)
	policy.ServiceRoles = append([]string(nil), policy.ServiceRoles...)
	policy.IdentityRolesRaw = append([]string(nil), policy.IdentityRolesRaw...)
	policy.ServiceRolesRaw = append([]string(nil), policy.ServiceRolesRaw...)
	return policy
}

func TestControllers(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "integration suite")
}

var _ = BeforeSuite(func() {
	ctrl.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	ctx, cancel = context.WithCancel(context.Background())

	Expect(zitiv1alpha1.AddToScheme(scheme.Scheme)).To(Succeed())

	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: false,
	}

	cfg, err := testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme:                 scheme.Scheme,
		Metrics:                metricsserver.Options{BindAddress: "0"},
		HealthProbeBindAddress: "0",
		LeaderElection:         false,
	})
	Expect(err).NotTo(HaveOccurred())

	fakeClient := newFakeOpenZitiClient()

	reconciler := &controller.ZitiIdentityReconciler{
		Client:          mgr.GetClient(),
		Scheme:          mgr.GetScheme(),
		Recorder:        mgr.GetEventRecorderFor("zitiidentity-controller"),
		IdentityService: identityservice.NewService(fakeClient),
	}
	Expect(reconciler.SetupWithManager(mgr)).To(Succeed())

	serviceReconciler := &controller.ZitiServiceReconciler{
		Client:         mgr.GetClient(),
		Scheme:         mgr.GetScheme(),
		Recorder:       mgr.GetEventRecorderFor("zitiservice-controller"),
		ServiceManager: serviceservice.NewService(fakeClient),
	}
	Expect(serviceReconciler.SetupWithManager(mgr)).To(Succeed())

	policyReconciler := &controller.ZitiAccessPolicyReconciler{
		Client:        mgr.GetClient(),
		Scheme:        mgr.GetScheme(),
		Recorder:      mgr.GetEventRecorderFor("zitiaccesspolicy-controller"),
		PolicyService: policyservice.NewService(fakeClient),
	}
	Expect(policyReconciler.SetupWithManager(mgr)).To(Succeed())

	go func() {
		defer GinkgoRecover()
		Expect(mgr.Start(ctx)).To(Succeed())
	}()

	k8sClient = mgr.GetClient()
	Expect(k8sClient).NotTo(BeNil())
})

var _ = AfterSuite(func() {
	cancel()
	if testEnv != nil {
		Expect(testEnv.Stop()).To(Succeed())
	}
})
