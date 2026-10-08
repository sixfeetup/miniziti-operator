package entra

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

func TestDirectoryRejectsMalformedAssignments(t *testing.T) {
	for _, record := range []string{
		`null`, `{}`, `{"principalId":"g","appRoleId":"r"}`,
		`{"principalId":"g","principalType":null,"appRoleId":"r"}`,
		`{"principalId":"g","principalType":"Unknown","appRoleId":"r"}`,
		`{"principalType":"Group","appRoleId":"r"}`,
		`{"principalId":"g","principalType":"Group"}`,
	} {
		t.Run(record, func(t *testing.T) {
			d, _ := testDirectory(t, func(w http.ResponseWriter, r *http.Request) {
				if strings.HasSuffix(r.URL.Path, "/token") {
					tokenResponse(w)
					return
				}
				_, _ = fmt.Fprintf(w, `{"value":[{"principalId":"valid","principalType":"Group","appRoleId":"r"},%s]}`, record)
			})
			items, err := d.ListAppRoleAssignedTo(context.Background(), "sp")
			if err == nil || len(items) != 0 {
				t.Fatalf("malformed snapshot accepted: items=%+v err=%v", items, err)
			}
		})
	}
}

func TestDirectoryRejectsMalformedMembers(t *testing.T) {
	for _, record := range []string{
		`null`, `{}`, `{"id":"u","mail":"u@example.com","userPrincipalName":"u@example.com"}`,
		`{"@odata.type":null,"id":"u"}`, `{"@odata.type":"#microsoft.graph.unknown","id":"u"}`,
		`{"@odata.type":"#microsoft.graph.user","mail":"u@example.com","userPrincipalName":"u@example.com"}`,
		`{"@odata.type":"#microsoft.graph.user","id":"u","userPrincipalName":"u@example.com"}`,
	} {
		t.Run(record, func(t *testing.T) {
			d, _ := testDirectory(t, func(w http.ResponseWriter, r *http.Request) {
				if strings.HasSuffix(r.URL.Path, "/token") {
					tokenResponse(w)
					return
				}
				_, _ = fmt.Fprintf(w, `{"value":[{"@odata.type":"#microsoft.graph.user","id":"valid","mail":"v@example.com","userPrincipalName":"v@example.com"},%s]}`, record)
			})
			items, err := d.ListGroupUsers(context.Background(), "g")
			if err == nil || len(items) != 0 {
				t.Fatalf("malformed snapshot accepted: items=%+v err=%v", items, err)
			}
		})
	}
}

func TestDirectoryAllowsNullableMailAndNonUserMembers(t *testing.T) {
	d, _ := testDirectory(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/token") {
			tokenResponse(w)
			return
		}
		_, _ = fmt.Fprint(w, `{"value":[{"@odata.type":"#microsoft.graph.group","id":"g"},{"@odata.type":"#microsoft.graph.user","id":"u","mail":null,"userPrincipalName":"u@example.com"}]}`)
	})
	items, err := d.ListGroupUsers(context.Background(), "g")
	if err != nil || len(items) != 1 || items[0].ID != "u" || items[0].Mail != "" {
		t.Fatalf("items=%+v err=%v", items, err)
	}
}

func TestDirectoryRejectsMalformedRoles(t *testing.T) {
	for _, body := range []string{
		`{"id":"sp"}`, `{"id":"sp","appRoles":null}`,
		`{"id":"sp","appRoles":[null]}`,
		`{"id":"sp","appRoles":[{"id":"r","value":"beta","isEnabled":null,"allowedMemberTypes":["User"]}]}`,
		`{"id":"sp","appRoles":[{"id":"r","value":"beta","isEnabled":true,"allowedMemberTypes":["Unknown"]}]}`,
		`{"id":"sp","appRoles":[{"id":"r","value":"beta","allowedMemberTypes":["User"]}]}`,
		`{"id":"sp","appRoles":[{"id":"r","value":"beta","isEnabled":true}]}`,
		`{"id":"sp","appRoles":[{"value":"beta","isEnabled":true,"allowedMemberTypes":["User"]}]}`,
		`{"id":"sp","appRoles":[{"id":"r","isEnabled":true,"allowedMemberTypes":["User"]}]}`,
	} {
		t.Run(body, func(t *testing.T) {
			d, _ := testDirectory(t, func(w http.ResponseWriter, r *http.Request) {
				if strings.HasSuffix(r.URL.Path, "/token") {
					tokenResponse(w)
					return
				}
				_, _ = fmt.Fprint(w, body)
			})
			item, err := d.GetServicePrincipal(context.Background(), appID)
			if err == nil || item != nil {
				t.Fatalf("malformed roles accepted: item=%+v err=%v", item, err)
			}
		})
	}
}

func TestDirectoryAllowsDisabledRoles(t *testing.T) {
	d, _ := testDirectory(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/token") {
			tokenResponse(w)
			return
		}
		_, _ = fmt.Fprint(w, `{"id":"sp","appRoles":[{"id":"r","value":"beta","isEnabled":false,"allowedMemberTypes":["User"]}]}`)
	})
	principal, err := d.GetServicePrincipal(context.Background(), appID)
	if err != nil || principal == nil || len(principal.AppRoles) != 1 || principal.AppRoles[0].IsEnabled {
		t.Fatalf("principal=%+v err=%v", principal, err)
	}
}

func TestDirectoryRejectsMalformedAssignmentOnLaterPage(t *testing.T) {
	base := ""
	d, endpoint := testDirectory(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/token") {
			tokenResponse(w)
			return
		}
		if strings.HasSuffix(r.URL.Path, "/later") {
			_, _ = fmt.Fprint(w, `{"value":[{"principalId":"g","appRoleId":"r"}]}`)
			return
		}
		_, _ = fmt.Fprintf(w, `{"value":[{"principalId":"g","principalType":"Group","appRoleId":"r"}],"@odata.nextLink":%q}`, base+"/v1.0/later")
	})
	base = endpoint
	items, err := d.ListAppRoleAssignedTo(context.Background(), "sp")
	if err == nil || len(items) != 0 {
		t.Fatalf("partial snapshot returned: items=%+v err=%v", items, err)
	}
}
