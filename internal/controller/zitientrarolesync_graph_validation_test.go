package controller

import (
	"context"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"example.com/miniziti-operator/internal/entra"
	openziti "example.com/miniziti-operator/internal/openziti/client"
)

type roleSyncGraphTransport func(*http.Request) (*http.Response, error)

func (f roleSyncGraphTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// Exercise the real Graph decoder and controller without external credentials.
// These tests are serial because NewDirectory uses http.DefaultTransport.
func TestRoleSyncMalformedGraphSnapshotNeverWrites(t *testing.T) {
	for _, kind := range []string{"assignment", "membership", "missing-mail", "role"} {
		t.Run(kind, func(t *testing.T) {
			f := newRoleSyncFixture(t, interceptor.Funcs{})
			f.identities = []openziti.Identity{
				{ID: "alice", ExternalID: "alice@example.com", RoleAttributes: []string{"beta"}},
				{ID: "bob", ExternalID: "bob@example.com", RoleAttributes: []string{"beta"}},
			}
			before := f.obj.Status.DeepCopy()
			original := http.DefaultTransport
			t.Cleanup(func() { http.DefaultTransport = original })
			http.DefaultTransport = roleSyncGraphTransport(func(r *http.Request) (*http.Response, error) {
				body := ""
				switch {
				case strings.HasSuffix(r.URL.Path, "/token"):
					body = `{"access_token":"test-token","token_type":"Bearer","expires_in":3600}`
				case strings.Contains(r.URL.Path, "servicePrincipals(appId="):
					body = `{"id":"sp","appRoles":[{"id":"beta","value":"beta","isEnabled":true,"allowedMemberTypes":["User"]}]}`
					if kind == "role" {
						body = `{"id":"sp","appRoles":[{"id":"beta","value":"beta","allowedMemberTypes":["User"]}]}`
					}
				case strings.HasSuffix(r.URL.Path, "/appRoleAssignedTo"):
					body = `{"value":[{"principalId":"alice-group","principalType":"Group","appRoleId":"beta"},{"principalId":"bob-group","principalType":"Group","appRoleId":"beta"}]}`
					if kind == "assignment" {
						body = `{"value":[{"principalId":"alice-group","principalType":"Group","appRoleId":"beta"},{"principalId":"bob-group","appRoleId":"beta"}]}`
					}
				case strings.Contains(r.URL.Path, "/groups/alice-group/"):
					body = `{"value":[{"@odata.type":"#microsoft.graph.user","id":"alice","mail":"alice@example.com","userPrincipalName":"alice@example.com"}]}`
				case strings.Contains(r.URL.Path, "/groups/bob-group/"):
					body = `{"value":[{"@odata.type":"#microsoft.graph.user","id":"bob","mail":"bob@example.com","userPrincipalName":"bob@example.com"}]}`
					if kind == "membership" {
						body = `{"value":[{"id":"bob","mail":"bob@example.com","userPrincipalName":"bob@example.com"}]}`
					}
					if kind == "missing-mail" {
						body = `{"value":[{"@odata.type":"#microsoft.graph.user","id":"bob","userPrincipalName":"bob@example.com"}]}`
					}
				default:
					t.Fatalf("unexpected Graph request: %s", r.URL)
				}
				return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"application/json"}}, Body: io.NopCloser(strings.NewReader(body))}, nil
			})
			f.r.DirectoryFactory = entra.NewDirectory
			result := f.r.runSync(context.Background(), f.obj)
			if result.Err == nil || result.Reason != "GraphError" || f.patches != 0 || result.WritePhaseStarted || !reflect.DeepEqual(before, &f.obj.Status) {
				t.Fatalf("malformed %s authorized writes: reason=%s err=%v patches=%d claims=%v", kind, result.Reason, result.Err, f.patches, f.obj.Status.ManagedAttributes)
			}
			if err := f.r.persistSyncResult(context.Background(), f.obj, result); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(before.ManagedAttributes, f.obj.Status.ManagedAttributes) || !reflect.DeepEqual(before.LastSyncTime, f.obj.Status.LastSyncTime) || f.obj.Status.LastError == "" {
				t.Fatalf("malformed snapshot changed successful sync state: %+v", f.obj.Status)
			}
		})
	}
}
