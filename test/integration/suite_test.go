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
)

var (
	testEnv   *envtest.Environment
	k8sClient client.Client
	ctx       context.Context
	cancel    context.CancelFunc
)

type fakeOpenZitiClient struct {
	mu         sync.Mutex
	nextID     int
	identities map[string]*openziti.Identity
}

func newFakeOpenZitiClient() *fakeOpenZitiClient {
	return &fakeOpenZitiClient{
		nextID:     1,
		identities: map[string]*openziti.Identity{},
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
	identity.ID = fmt.Sprintf("identity-%d", f.nextID)
	f.nextID++
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

func (f *fakeOpenZitiClient) GetService(context.Context, string) (*openziti.Service, error) {
	return nil, nil
}

func (f *fakeOpenZitiClient) FindServiceByName(context.Context, string) (*openziti.Service, error) {
	return nil, nil
}

func (f *fakeOpenZitiClient) CreateService(context.Context, openziti.Service) (*openziti.Service, error) {
	return nil, openziti.ErrNotImplemented
}

func (f *fakeOpenZitiClient) UpdateService(context.Context, openziti.Service) (*openziti.Service, error) {
	return nil, openziti.ErrNotImplemented
}

func (f *fakeOpenZitiClient) DeleteService(context.Context, string) error {
	return openziti.ErrNotImplemented
}

func (f *fakeOpenZitiClient) GetConfig(context.Context, string) (*openziti.ServiceConfig, error) {
	return nil, nil
}

func (f *fakeOpenZitiClient) CreateConfig(context.Context, openziti.ServiceConfig) (*openziti.ServiceConfig, error) {
	return nil, openziti.ErrNotImplemented
}

func (f *fakeOpenZitiClient) UpdateConfig(context.Context, openziti.ServiceConfig) (*openziti.ServiceConfig, error) {
	return nil, openziti.ErrNotImplemented
}

func (f *fakeOpenZitiClient) DeleteConfig(context.Context, string) error {
	return openziti.ErrNotImplemented
}

func (f *fakeOpenZitiClient) GetAccessPolicy(context.Context, string) (*openziti.AccessPolicy, error) {
	return nil, nil
}

func (f *fakeOpenZitiClient) FindAccessPolicyByName(context.Context, string) (*openziti.AccessPolicy, error) {
	return nil, nil
}

func (f *fakeOpenZitiClient) CreateAccessPolicy(context.Context, openziti.AccessPolicy) (*openziti.AccessPolicy, error) {
	return nil, openziti.ErrNotImplemented
}

func (f *fakeOpenZitiClient) UpdateAccessPolicy(context.Context, openziti.AccessPolicy) (*openziti.AccessPolicy, error) {
	return nil, openziti.ErrNotImplemented
}

func (f *fakeOpenZitiClient) DeleteAccessPolicy(context.Context, string) error {
	return openziti.ErrNotImplemented
}

func fakeIdentityFailure(name string) error {
	if strings.Contains(name, "fail-status") || strings.Contains(name, "fail-event") {
		return fmt.Errorf("simulated backend failure for %s", name)
	}
	return nil
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

	reconciler := &controller.ZitiIdentityReconciler{
		Client:          mgr.GetClient(),
		Scheme:          mgr.GetScheme(),
		Recorder:        mgr.GetEventRecorderFor("zitiidentity-controller"),
		IdentityService: identityservice.NewService(newFakeOpenZitiClient()),
	}
	Expect(reconciler.SetupWithManager(mgr)).To(Succeed())

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
