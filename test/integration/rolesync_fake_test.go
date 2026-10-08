package integration

import (
	"context"
	"fmt"

	openziti "example.com/miniziti-operator/internal/openziti/client"
)

func (f *fakeOpenZitiClient) setRoleLookupGate(name string, gate <-chan struct{}) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.identityLookupGates == nil {
		f.identityLookupGates = map[string]<-chan struct{}{}
	}
	f.identityLookupGates[name] = gate
}

func (f *fakeOpenZitiClient) seedRoleIdentity(identity openziti.Identity) {
	f.mu.Lock()
	defer f.mu.Unlock()
	identity.RoleAttributes = append([]string{}, identity.RoleAttributes...)
	f.identities[identity.ID] = &identity
}
func (f *fakeOpenZitiClient) rolePatchCount(id string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.patchCalls[id]
}
func (f *fakeOpenZitiClient) roleAttrs(id string) []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	if identity := f.identities[id]; identity != nil {
		return append([]string{}, identity.RoleAttributes...)
	}
	return nil
}
func (f *fakeOpenZitiClient) failRolePatch(id string, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.patchFailures == nil {
		f.patchFailures = map[string]error{}
	}
	f.patchFailures[id] = err
}
func (f *fakeOpenZitiClient) removeRoleFixture(id string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.identities, id)
	delete(f.patchCalls, id)
	delete(f.patchFailures, id)
}

func (f *fakeOpenZitiClient) PatchIdentityRoleAttributes(_ context.Context, id string, attrs []string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.patchCalls == nil {
		f.patchCalls = map[string]int{}
	}
	f.patchCalls[id]++
	if err := f.patchFailures[id]; err != nil {
		return err
	}
	identity, ok := f.identities[id]
	if !ok {
		return fmt.Errorf("identity %q not found", id)
	}
	identity.RoleAttributes = append([]string{}, attrs...)
	return nil
}
