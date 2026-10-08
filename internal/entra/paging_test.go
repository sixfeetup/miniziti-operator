package entra

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDirectoryPagedAssignmentsAndUsers(t *testing.T) {
	for _, members := range []bool{false, true} {
		t.Run(fmt.Sprint(members), func(t *testing.T) {
			calls := 0
			var origin string
			d, url := testDirectory(t, func(w http.ResponseWriter, r *http.Request) {
				if strings.HasSuffix(r.URL.Path, "/token") {
					tokenResponse(w)
					return
				}
				calls++
				item := `{"principalId":"g","principalType":"Group","appRoleId":"r"}`
				if members {
					item = `{"@odata.type":"#microsoft.graph.user","id":"u","mail":null,"userPrincipalName":"u@example.com"}`
				}
				switch calls {
				case 1:
					_, _ = fmt.Fprintf(w, `{"value":[%s],"@odata.nextLink":%q}`, item, origin+"/v1.0/page2")
				case 2:
					_, _ = fmt.Fprintf(w, `{"value":[],"@odata.nextLink":%q}`, origin+"/v1.0/page3")
				case 3:
					_, _ = fmt.Fprintf(w, `{"value":[%s]}`, item)
				default:
					t.Error("too many requests")
				}
			})
			origin = url
			n := 0
			var err error
			if members {
				var got []User
				got, err = d.ListGroupUsers(context.Background(), "g")
				n = len(got)
			} else {
				var got []AppRoleAssignment
				got, err = d.ListAppRoleAssignedTo(context.Background(), "s")
				n = len(got)
			}
			if err != nil || calls != 3 || n != 2 {
				t.Fatalf("n=%d calls=%d err=%v", n, calls, err)
			}
		})
	}
}

func TestDirectoryRejectsUnsafePaginationAndRedirects(t *testing.T) {
	foreignCalls := 0
	foreign := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { foreignCalls++; tokenResponse(w) }))
	defer foreign.Close()
	for _, next := range []string{"http://graph.microsoft.com/v1.0/p", foreign.URL, "https://graph.microsoft.com.evil.invalid/p", "https://user:pass@graph.microsoft.com/p", "https://graph.microsoft.com:444/p"} {
		t.Run(next, func(t *testing.T) {
			d, _ := testDirectory(t, func(w http.ResponseWriter, r *http.Request) {
				if strings.HasSuffix(r.URL.Path, "/token") {
					tokenResponse(w)
					return
				}
				_, _ = fmt.Fprintf(w, `{"value":[],"@odata.nextLink":%q}`, next)
			})
			got, err := d.ListAppRoleAssignedTo(context.Background(), "sp")
			if err == nil || len(got) != 0 {
				t.Fatalf("got=%v err=%v", got, err)
			}
		})
	}
	t.Run("redirect", func(t *testing.T) {
		d, _ := testDirectory(t, func(w http.ResponseWriter, r *http.Request) {
			if strings.HasSuffix(r.URL.Path, "/token") {
				tokenResponse(w)
				return
			}
			http.Redirect(w, r, foreign.URL, http.StatusFound)
		})
		if _, err := d.ListGroupUsers(context.Background(), "g"); err == nil {
			t.Fatal("accepted redirect")
		}
	})
	if foreignCalls != 0 {
		t.Fatalf("leaked to foreign host: %d", foreignCalls)
	}
}

func TestDirectoryRejectsPaginationCycles(t *testing.T) {
	calls := 0
	var origin string
	d, url := testDirectory(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/token") {
			tokenResponse(w)
			return
		}
		calls++
		next := origin + "/v1.0/page2"
		if calls == 2 {
			next = origin + "/v1.0/servicePrincipals/sp/appRoleAssignedTo"
		}
		_, _ = fmt.Fprintf(w, `{"value":[{"principalId":"g","principalType":"Group","appRoleId":"r"}],"@odata.nextLink":%q}`, next)
	})
	origin = url
	got, err := d.ListAppRoleAssignedTo(context.Background(), "sp")
	if err == nil || len(got) != 0 || calls != 2 {
		t.Fatalf("got=%v err=%v calls=%d", got, err, calls)
	}
}

func TestDirectoryRejectsMalformedLaterPage(t *testing.T) {
	for _, bad := range []string{`{"value":`, `{"value":[]} {"value":[]}`} {
		t.Run(bad, func(t *testing.T) {
			var origin string
			d, url := testDirectory(t, func(w http.ResponseWriter, r *http.Request) {
				if strings.HasSuffix(r.URL.Path, "/token") {
					tokenResponse(w)
					return
				}
				if r.URL.Path == "/v1.0/page2" {
					_, _ = fmt.Fprint(w, bad)
					return
				}
				_, _ = fmt.Fprintf(w, `{"value":[{"principalId":"g"}],"@odata.nextLink":%q}`, origin+"/v1.0/page2")
			})
			origin = url
			got, err := d.ListAppRoleAssignedTo(context.Background(), "sp")
			if err == nil || len(got) != 0 {
				t.Fatalf("got=%v err=%v", got, err)
			}
		})
	}
}
