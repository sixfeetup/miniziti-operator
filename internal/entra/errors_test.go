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

func TestDirectoryClassifiesErrors(t *testing.T) {
	for _, tc := range []struct {
		status int
		retry  string
		want   time.Duration
	}{{404, "", 0}, {403, "", 0}, {429, "120", 120 * time.Second}, {503, "120", 120 * time.Second}, {503, "Thu, 01 Jan 2026 00:00:30 GMT", 30 * time.Second}} {
		t.Run(fmt.Sprint(tc.status, tc.retry), func(t *testing.T) {
			d, _ := testDirectory(t, func(w http.ResponseWriter, r *http.Request) {
				if strings.HasSuffix(r.URL.Path, "/token") {
					tokenResponse(w)
					return
				}
				w.Header().Set("Retry-After", tc.retry)
				w.WriteHeader(tc.status)
				_, _ = fmt.Fprint(w, `{"error":{"code":"Denied","message":"permission denied"}}`)
			})
			_, err := d.GetServicePrincipal(context.Background(), appID)
			if tc.status == 404 {
				if !errors.Is(err, ErrServicePrincipalNotFound) {
					t.Fatal(err)
				}
				return
			}
			var ge *GraphError
			if !errors.As(err, &ge) || ge.StatusCode != tc.status || ge.Code != "Denied" || ge.Message != "permission denied" || ge.RetryAfter != tc.want {
				t.Fatalf("error=%#v", err)
			}
		})
	}
}

func TestDirectorySanitizesTokenErrors(t *testing.T) {
	d, _ := testDirectory(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = fmt.Fprint(w, `{"error":"invalid_client","error_description":"AADSTS7000215: Invalid client secret reader-secret. Trace ID: trace-123. Correlation ID: correlation-123. Timestamp: timestamp-123.","access_token":"echo-token"}`)
	})
	_, err := d.GetServicePrincipal(context.Background(), appID)
	if err == nil || !strings.Contains(err.Error(), "AADSTS7000215") {
		t.Fatalf("%v", err)
	}
	for _, s := range []string{"reader-secret", "echo-token", "trace-123", "correlation-123", "timestamp-123"} {
		if strings.Contains(err.Error(), s) {
			t.Fatalf("leaked %s: %v", s, err)
		}
	}
}

func TestDirectorySanitizesGraphErrors(t *testing.T) {
	d, _ := testDirectory(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/token") {
			tokenResponse(w)
			return
		}
		w.WriteHeader(http.StatusForbidden)
		_, _ = fmt.Fprint(w, `{"error":{"code":"Denied","message":"reader-secret test-token"}}`)
	})
	_, err := d.ListGroupUsers(context.Background(), "g")
	if err == nil || strings.Contains(err.Error(), "reader-secret") || strings.Contains(err.Error(), "test-token") {
		t.Fatalf("unsafe error: %v", err)
	}
}
