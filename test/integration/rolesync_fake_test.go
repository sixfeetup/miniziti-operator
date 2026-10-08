package integration

import (
	"context"
	"fmt"
)

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
