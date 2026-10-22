package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"testing"
)

const validIdentityJSON = `{"id":"identity-1","name":"Alice","type":{"id":"User","name":"User"},"externalId":"alice@example.com","roleAttributes":["alpha"]}`

func TestIdentityDetailRejectsMalformedSuccess(t *testing.T) {
	for _, body := range []string{
		`{}`, `{"data":null}`,
		`{"data":{"id":"identity-1","name":"Alice","externalId":"alice@example.com","roleAttributes":["alpha"]}}`,
		`{"data":{"id":"identity-1","type":{"name":"User"},"roleAttributes":[]}}`,
		`{"data":{"id":"","name":"Alice","type":{"name":"User"},"roleAttributes":[]}}`,
		`{"data":{"id":"identity-1","name":" ","type":{"name":"User"},"roleAttributes":[]}}`,
		`{"data":{"name":"Alice","type":{"name":"User"},"roleAttributes":[]}}`,
		`{"data":{"id":"identity-1","name":"Alice","type":{"name":" "},"roleAttributes":null}}`,
	} {
		c, _ := identityHTTPClient(t, func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprint(w, body)
		})
		got, err := c.GetIdentity(context.Background(), "identity-1")
		if err == nil || got != nil {
			t.Fatalf("malformed success is not a deletion: body=%s got=%+v err=%v", body, got, err)
		}
	}
}

func TestIdentityReadsAcceptOptionalRoleAttributes(t *testing.T) {
	for _, tc := range []struct {
		name           string
		attributeField string
		wantAttributes []string
	}{
		{name: "null", attributeField: `,"roleAttributes":null`},
		{name: "omitted"},
		{name: "empty", attributeField: `,"roleAttributes":[]`},
		{name: "populated", attributeField: `,"roleAttributes":["alpha","beta"]`, wantAttributes: []string{"alpha", "beta"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Router identities can have null attributes in controller responses.
			identity := fmt.Sprintf(`{"id":"router-1","name":"hosting-router","type":{"id":"Router","name":"Router"},"externalId":null%s}`, tc.attributeField)
			c, _ := identityHTTPClient(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch r.URL.Path {
				case "/edge/management/v1/identities/router-1":
					_, _ = fmt.Fprintf(w, `{"data":%s}`, identity)
				case "/edge/management/v1/identities":
					_, _ = fmt.Fprintf(w, `{"data":[%s],"meta":{"pagination":{"limit":100,"offset":0,"totalCount":1}}}`, identity)
				default:
					t.Errorf("unexpected identity request: %s", r.URL.Path)
					http.NotFound(w, r)
				}
			})
			checkIdentity := func(t *testing.T, got *Identity) {
				t.Helper()
				if got == nil || got.ID != "router-1" || got.Name != "hosting-router" || got.Type != "Router" || !slices.Equal(got.RoleAttributes, tc.wantAttributes) {
					t.Fatalf("identity=%+v, want hosting router with attributes %v", got, tc.wantAttributes)
				}
			}
			t.Run("detail", func(t *testing.T) {
				got, err := c.GetIdentity(context.Background(), "router-1")
				if err != nil {
					t.Fatal(err)
				}
				checkIdentity(t, got)
			})
			t.Run("list", func(t *testing.T) {
				got, err := c.ListIdentities(context.Background())
				if err != nil || len(got) != 1 {
					t.Fatalf("list=%+v err=%v", got, err)
				}
				checkIdentity(t, &got[0])
			})
		})
	}
}

func TestIdentityDetailOnly404ConfirmsDeletion(t *testing.T) {
	c, _ := identityHTTPClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = fmt.Fprint(w, `{"error":{"code":"NOT_FOUND","message":"deleted"}}`)
	})
	got, err := c.GetIdentity(context.Background(), "identity-1")
	if got != nil || err != nil {
		t.Fatalf("confirmed 404: got=%+v err=%v", got, err)
	}
}

func TestIdentityListRejectsMalformedSuccess(t *testing.T) {
	for _, body := range []string{
		`{}`, `{"data":null}`,
		`{"data":[null]}`,
		fmt.Sprintf(`{"data":[%s,{"id":"identity-2","name":"Other","externalId":"ALICE@example.com","roleAttributes":["alpha"]}]}`, validIdentityJSON),
	} {
		c, _ := identityHTTPClient(t, func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprint(w, body)
		})
		got, err := c.ListIdentities(context.Background())
		if err == nil || got != nil {
			t.Fatalf("incomplete list must fail: body=%s got=%+v err=%v", body, got, err)
		}
	}
}

func TestIdentityListDiscardsEarlierPagesOnMalformedItem(t *testing.T) {
	calls := 0
	c, _ := identityHTTPClient(t, func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		if calls == 1 {
			items := make([]json.RawMessage, 100)
			for i := range items {
				items[i] = json.RawMessage(validIdentityJSON)
			}
			if err := json.NewEncoder(w).Encode(map[string]any{"data": items}); err != nil {
				t.Error(err)
			}
			return
		}
		_, _ = fmt.Fprint(w, `{"data":[{"id":"identity-2","externalId":"ALICE@example.com"}]}`)
	})
	got, err := c.ListIdentities(context.Background())
	if calls != 2 || got != nil || err == nil {
		t.Fatalf("partial snapshot escaped: calls=%d identities=%d err=%v", calls, len(got), err)
	}
}

func TestIdentityListAcceptsCompleteEmptyList(t *testing.T) {
	c, _ := identityHTTPClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, `{"data":[]}`)
	})
	got, err := c.ListIdentities(context.Background())
	if len(got) != 0 || err != nil {
		t.Fatalf("valid empty list: got=%+v err=%v", got, err)
	}
}
