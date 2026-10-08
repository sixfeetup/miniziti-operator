package integration

import (
	"context"
	"errors"
	"sync"

	"example.com/miniziti-operator/internal/entra"
)

type entraFixture struct {
	roles       []entra.AppRole
	assignments []entra.AppRoleAssignment
	users       map[string][]entra.User
	err         error
}
type fakeEntra struct {
	mu                          sync.Mutex
	fixtures                    map[string]entraFixture
	calls                       map[string]int
	secrets                     map[string]string
	assignmentCalls, groupCalls map[string]int
}

func newFakeEntra() *fakeEntra {
	return &fakeEntra{fixtures: map[string]entraFixture{}, calls: map[string]int{}, secrets: map[string]string{},
		assignmentCalls: map[string]int{}, groupCalls: map[string]int{}}
}
func (f *fakeEntra) replace(app string, fixture entraFixture) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.fixtures[app] = cloneEntraFixture(fixture)
}
func (f *fakeEntra) callCount(app string) int { f.mu.Lock(); defer f.mu.Unlock(); return f.calls[app] }
func (f *fakeEntra) groupReads(app string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.groupCalls[app]
}
func (f *fakeEntra) assignmentReads(app string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.assignmentCalls[app]
}
func (f *fakeEntra) lastSecret(app string) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.secrets[app]
}
func (f *fakeEntra) factory(_, _, secret string) entra.Directory {
	return &fakeDirectory{state: f, secret: secret}
}
func cloneEntraFixture(source entraFixture) entraFixture {
	result := source
	result.roles = append([]entra.AppRole{}, source.roles...)
	for i := range result.roles {
		result.roles[i].AllowedMemberTypes = append([]string{}, source.roles[i].AllowedMemberTypes...)
	}
	result.assignments = append([]entra.AppRoleAssignment{}, source.assignments...)
	result.users = map[string][]entra.User{}
	for id, users := range source.users {
		result.users[id] = append([]entra.User{}, users...)
	}
	return result
}

type fakeDirectory struct {
	state       *fakeEntra
	secret, app string
	fixture     entraFixture
}

func (d *fakeDirectory) GetServicePrincipal(_ context.Context, app string) (*entra.ServicePrincipal, error) {
	d.state.mu.Lock()
	defer d.state.mu.Unlock()
	d.state.calls[app]++
	d.state.secrets[app] = d.secret
	fixture, ok := d.state.fixtures[app]
	if !ok {
		return nil, errors.New("no Entra test fixture registered")
	}
	d.app = app
	d.fixture = cloneEntraFixture(fixture)
	if fixture.err != nil {
		return nil, fixture.err
	}
	return &entra.ServicePrincipal{ID: app, AppRoles: d.fixture.roles}, nil
}
func (d *fakeDirectory) ListAppRoleAssignedTo(context.Context, string) ([]entra.AppRoleAssignment, error) {
	d.state.mu.Lock()
	d.state.assignmentCalls[d.app]++
	d.state.mu.Unlock()
	return append([]entra.AppRoleAssignment{}, d.fixture.assignments...), nil
}
func (d *fakeDirectory) ListGroupUsers(_ context.Context, group string) ([]entra.User, error) {
	d.state.mu.Lock()
	d.state.groupCalls[d.app]++
	d.state.mu.Unlock()
	return append([]entra.User{}, d.fixture.users[group]...), nil
}
