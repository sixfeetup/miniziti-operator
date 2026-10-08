package entra

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestDirectoryTransientTokenErrorKeepsAADSTSCode(t *testing.T) {
	d, _ := testDirectory(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = fmt.Fprint(w, `{"error":"temporarily_unavailable","error_description":"AADSTS90033: Retry later."}`)
	})
	_, err := d.GetServicePrincipal(context.Background(), appID)
	var token *TokenError
	if !errors.As(err, &token) || token.Code != "AADSTS90033" || !token.Transient() {
		t.Fatalf("transient provider error was treated as permanent: %#v", err)
	}
}

func TestDirectoryTokenFailureMetadata(t *testing.T) {
	for _, tc := range []struct {
		status    int
		code      string
		retry     string
		wantRetry time.Duration
		transient bool
	}{
		{400, "invalid_client", "", 0, false},
		{401, "invalid_client", "", 0, false},
		{403, "unauthorized_client", "", 0, false},
		{400, "temporarily_unavailable", "", 0, true},
		{400, "server_error", "", 0, true},
		{429, "throttled", "120", 120 * time.Second, true},
		{503, "temporarily_unavailable", "120", 120 * time.Second, true},
		{503, "temporarily_unavailable", "Thu, 01 Jan 2026 00:00:30 GMT", 30 * time.Second, true},
		{500, "server_error", "", 0, true},
		{502, "", "", 0, true},
	} {
		t.Run(fmt.Sprintf("%d/%s/%s", tc.status, tc.code, tc.retry), func(t *testing.T) {
			d, _ := testDirectory(t, func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Retry-After", tc.retry)
				w.WriteHeader(tc.status)
				_, _ = fmt.Fprintf(w, `{"error":%q,"error_description":"reader-secret echo-token retry later. Trace ID: unstable","access_token":"echo-token"}`, tc.code)
			})
			_, err := d.GetServicePrincipal(context.Background(), appID)
			var token *TokenError
			if !errors.As(err, &token) || token.StatusCode != tc.status || token.RetryAfter != tc.wantRetry || token.Transient() != tc.transient {
				t.Fatalf("error=%#v", err)
			}
			for _, value := range []string{"reader-secret", "echo-token", "unstable"} {
				if strings.Contains(err.Error(), value) {
					t.Fatalf("unsafe token error: %v", err)
				}
			}
		})
	}
}
