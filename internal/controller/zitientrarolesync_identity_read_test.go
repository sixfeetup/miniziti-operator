package controller

import (
	"context"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"example.com/miniziti-operator/internal/credentials"
	openziti "example.com/miniziti-operator/internal/openziti/client"
)

func TestRoleSyncMalformedZitiReadRetainsClaims(t *testing.T) {
	for _, phase := range []string{"list", "detail"} {
		t.Run(phase, func(t *testing.T) {
			f := newRoleSyncFixture(t, interceptor.Funcs{})
			patches, reads := 0, 0
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if r.Method == http.MethodPost {
					_, _ = fmt.Fprint(w, `{"data":{"token":"test-session"}}`)
					return
				}
				if r.Method == http.MethodPatch {
					patches++
					_, _ = fmt.Fprint(w, `{"data":{}}`)
					return
				}
				reads++
				identity := `{"id":"i1","name":"Alice","type":{"name":"User"},"externalId":"alice@example.com","roleAttributes":["custom","alpha"]}`
				if r.URL.Path == "/edge/management/v1/identities" {
					if phase == "list" {
						_, _ = fmt.Fprintf(w, `{"data":[%s,{"id":"i2","name":"Duplicate","externalId":"ALICE@example.com","roleAttributes":["alpha"]}]}`, identity)
					} else {
						_, _ = fmt.Fprintf(w, `{"data":[%s]}`, identity)
					}
					return
				}
				_, _ = fmt.Fprint(w, `{"data":{"id":"i1","name":"Alice","externalId":"alice@example.com","roleAttributes":["alpha"]}}`)
			}))
			defer server.Close()
			f.r.ZitiClient = openziti.New(func(context.Context) (credentials.ManagementConfig, error) {
				return credentials.ManagementConfig{ControllerURL: server.URL + "/edge/management/v1", Username: "admin", Password: "test-password", CABundlePEM: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw})}, nil
			})
			result := f.run()
			if reads == 0 || (phase == "detail" && reads < 2) {
				t.Fatalf("did not exercise malformed response: reads=%d err=%v", reads, result.Err)
			}
			if result.Err == nil || result.Reason != syncReasonZitiError || patches != 0 {
				t.Fatalf("malformed response authorized sync: result=%+v patches=%d", result, patches)
			}
			if phase == "list" {
				if result.WritePhaseStarted || !reflect.DeepEqual(f.obj.Status.ManagedAttributes, []string{"alpha"}) {
					t.Fatalf("incomplete snapshot recorded claims: %+v", result)
				}
			} else if result.Completion == nil || !slices.Contains(result.Completion.ManagedAttributes, "alpha") || result.Completion.AdvanceLastSyncTime {
				t.Fatalf("malformed detail confirmed retirement: %+v", result)
			}
		})
	}
}
