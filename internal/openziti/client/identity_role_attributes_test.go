package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"testing"

	transport "github.com/go-openapi/runtime/client"
	"github.com/go-openapi/strfmt"
	"github.com/openziti/edge-api/rest_management_api_client"

	"example.com/miniziti-operator/internal/credentials"
)

func identityHTTPClient(t *testing.T, handler http.HandlerFunc) (*ManagementClient, *int) {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	u, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	logins := new(int)
	return &ManagementClient{
		loadConfig: func(context.Context) (credentials.ManagementConfig, error) {
			return credentials.ManagementConfig{ControllerURL: server.URL}, nil
		},
		authenticate: func(context.Context, credentials.ManagementConfig) (*rest_management_api_client.ZitiEdgeManagement, error) {
			*logins++
			return rest_management_api_client.New(transport.New(u.Host, "/edge/management/v1", []string{"http"}), strfmt.Default), nil
		},
	}, logins
}

func TestIdentityExternalIDMapping(t *testing.T) {
	for _, external := range []string{`"User@example.com"`, "null"} {
		t.Run(external, func(t *testing.T) {
			c, _ := identityHTTPClient(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				identity := fmt.Sprintf(`{"id":"identity-1","name":"Alice","type":{"id":"User","name":"User"},"externalId":%s,"roleAttributes":[]}`, external)
				if r.URL.Path == "/edge/management/v1/identities" {
					_, _ = fmt.Fprintf(w, `{"data":[%s],"meta":{"pagination":{"limit":100,"offset":0,"totalCount":1}}}`, identity)
				} else {
					_, _ = fmt.Fprintf(w, `{"data":%s}`, identity)
				}
			})
			want := ""
			if external != "null" {
				want = "User@example.com"
			}
			detail, err := c.GetIdentity(context.Background(), "identity-1")
			if err != nil {
				t.Fatal(err)
			}
			if detail == nil || detail.ExternalID != want {
				t.Fatalf("detail=%+v, want externalId %q", detail, want)
			}
			list, err := c.ListIdentities(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if len(list) != 1 || list[0].ExternalID != want {
				t.Fatalf("list=%+v", list)
			}
		})
	}
}

func TestPatchIdentityRoleAttributesOnlyWritesAttributes(t *testing.T) {
	var body map[string]any
	c, _ := identityHTTPClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PATCH" || r.URL.Path != "/edge/management/v1/identities/identity-1" {
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"data":{}}`)
	})
	if err := c.PatchIdentityRoleAttributes(context.Background(), "identity-1", []string{"beta"}); err != nil {
		t.Fatal(err)
	}
	if len(body) != 1 || !reflect.DeepEqual(body["roleAttributes"], []any{"beta"}) {
		t.Fatalf("unsafe PATCH: %#v", body)
	}
}

func TestPatchIdentityRoleAttributesClearsWithEmptyArray(t *testing.T) {
	for _, attrs := range [][]string{nil, {}} {
		c, _ := identityHTTPClient(t, func(w http.ResponseWriter, r *http.Request) {
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Error(err)
			}
			a, ok := body["roleAttributes"].([]any)
			if len(body) != 1 || !ok || a == nil || len(a) != 0 {
				t.Errorf("clear body=%#v", body)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprint(w, `{"data":{}}`)
		})
		if err := c.PatchIdentityRoleAttributes(context.Background(), "identity-1", attrs); err != nil {
			t.Fatal(err)
		}
	}
}

func TestPatchIdentityRoleAttributesDoesNotCreateOnNotFound(t *testing.T) {
	patches, creates := 0, 0
	c, _ := identityHTTPClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "PATCH" {
			patches++
		}
		if r.Method == "POST" {
			creates++
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = fmt.Fprint(w, `{"error":{"code":"NOT_FOUND","message":"deleted"}}`)
	})
	if err := c.PatchIdentityRoleAttributes(context.Background(), "identity-1", nil); err == nil {
		t.Fatal("404 must fail")
	}
	if patches != 1 || creates != 0 {
		t.Fatalf("patches=%d creates=%d", patches, creates)
	}
}

func TestPatchIdentityRoleAttributesReauthenticates(t *testing.T) {
	patches := 0
	c, logins := identityHTTPClient(t, func(w http.ResponseWriter, r *http.Request) {
		patches++
		w.Header().Set("Content-Type", "application/json")
		if patches == 1 {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = fmt.Fprint(w, `{"error":{"code":"UNAUTHORIZED","message":"expired"}}`)
			return
		}
		_, _ = fmt.Fprint(w, `{"data":{}}`)
	})
	if err := c.PatchIdentityRoleAttributes(context.Background(), "identity-1", nil); err != nil {
		t.Fatal(err)
	}
	if patches != 2 || *logins != 2 {
		t.Fatalf("patches=%d logins=%d", patches, *logins)
	}
}
