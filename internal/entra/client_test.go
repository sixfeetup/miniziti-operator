package entra

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const appID = "00000000-0000-0000-0000-000000000002"
const testUserEmail = "u@example.com"

func testDirectory(t *testing.T, handler http.HandlerFunc) (*directory, string) {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return newDirectory("tenant-1", "reader-id", "reader-secret", clientOptions{graphURL: server.URL + "/v1.0", loginURL: server.URL, transport: server.Client().Transport, now: func() time.Time { return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) }}), server.URL
}
func tokenResponse(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = fmt.Fprint(w, `{"access_token":"test-token","token_type":"Bearer","expires_in":3600}`)
}

func TestDirectoryTokenAndRequests(t *testing.T) {
	tokens, reads := 0, 0
	d, _ := testDirectory(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/token") {
			tokens++
			if r.URL.Path != "/tenant-1/oauth2/v2.0/token" {
				t.Errorf("token path=%s", r.URL.Path)
			}
			if err := r.ParseForm(); err != nil {
				t.Error(err)
			}
			for key, want := range map[string]string{"grant_type": "client_credentials", "scope": "https://graph.microsoft.com/.default", "client_id": "reader-id", "client_secret": "reader-secret"} {
				if r.Form.Get(key) != want {
					t.Errorf("%s=%q", key, r.Form.Get(key))
				}
			}
			tokenResponse(w)
			return
		}
		reads++
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Error("missing token")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"value":[]}`)
	})
	for i := 0; i < 2; i++ {
		if _, err := d.ListAppRoleAssignedTo(context.Background(), "sp-1"); err != nil {
			t.Fatal(err)
		}
	}
	if tokens != 1 || reads != 2 {
		t.Fatalf("tokens=%d reads=%d", tokens, reads)
	}
}

func TestDirectoryServicePrincipal(t *testing.T) {
	d, _ := testDirectory(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/token") {
			tokenResponse(w)
			return
		}
		if r.URL.Path != "/v1.0/servicePrincipals(appId='"+appID+"')" || r.URL.Query().Get("$select") != "id,appRoles" {
			t.Errorf("request=%s", r.URL)
		}
		_, _ = fmt.Fprint(w, `{"id":"sp-1","appRoles":[{"id":"r1","value":"alpha","isEnabled":true,"allowedMemberTypes":["User"]},{"id":"r2","value":"beta","isEnabled":false,"allowedMemberTypes":["Application","User"]}]}`)
	})
	got, err := d.GetServicePrincipal(context.Background(), appID)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "sp-1" || len(got.AppRoles) != 2 || got.AppRoles[1].IsEnabled || len(got.AppRoles[1].AllowedMemberTypes) != 2 {
		t.Fatalf("%+v", got)
	}
}

func TestDirectoryGroupUsers(t *testing.T) {
	for _, limited := range []bool{false, true} {
		t.Run(fmt.Sprint(limited), func(t *testing.T) {
			d, _ := testDirectory(t, func(w http.ResponseWriter, r *http.Request) {
				if strings.HasSuffix(r.URL.Path, "/token") {
					tokenResponse(w)
					return
				}
				if r.URL.Path != "/v1.0/groups/group-1/members" || r.URL.RawQuery != "" {
					t.Errorf("request=%s", r.URL)
				}
				upn := testUserEmail
				if limited {
					upn = ""
				}
				_, _ = fmt.Fprintf(w, `{"value":[{"@odata.type":"#microsoft.graph.group","id":"g"},{"@odata.type":"#microsoft.graph.user","id":"u","mail":"u@example.com","userPrincipalName":%q}]}`, upn)
			})
			got, err := d.ListGroupUsers(context.Background(), "group-1")
			if limited {
				if !errors.Is(err, ErrLimitedUserData) || len(got) != 0 {
					t.Fatalf("got=%v err=%v", got, err)
				}
				return
			}
			if err != nil || len(got) != 1 || got[0].Mail != testUserEmail || got[0].UserPrincipalName != testUserEmail {
				t.Fatalf("got=%v err=%v", got, err)
			}
		})
	}
}
