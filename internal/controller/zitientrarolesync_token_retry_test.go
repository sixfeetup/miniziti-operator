package controller

import (
	"errors"
	"testing"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"example.com/miniziti-operator/internal/entra"
)

func TestRoleSyncTokenRetry(t *testing.T) {
	for _, tc := range []struct {
		status    int
		code      string
		retry     time.Duration
		want      time.Duration
		wantError bool
	}{
		{400, "invalid_client", 0, 24 * time.Hour, false},
		{401, "invalid_client", 0, 24 * time.Hour, false},
		{403, "unauthorized_client", 0, 24 * time.Hour, false},
		{400, "temporarily_unavailable", 0, 0, true},
		{400, "server_error", 0, 0, true},
		{500, "server_error", 0, 0, true},
		{503, "temporarily_unavailable", 0, 0, true},
		{429, "throttled", 0, 0, true},
		{429, "throttled", 2 * time.Minute, 2 * time.Minute, false},
		{503, "temporarily_unavailable", 2 * time.Minute, 2 * time.Minute, false},
	} {
		f := newRoleSyncFixture(t, interceptor.Funcs{})
		f.obj.Spec.Interval = "24h"
		f.directory.principalErr = &entra.TokenError{StatusCode: tc.status, Code: tc.code, Message: "safe token failure", RetryAfter: tc.retry}
		result := f.run()
		if result.Reason != syncReasonGraphError || result.RetryAfter != tc.retry || f.patches != 0 {
			t.Fatalf("case=%+v result=%+v patches=%d", tc, result, f.patches)
		}
		got, err := retryResult(24*time.Hour, result.Reason, result.Err, result.RetryAfter)
		if got.RequeueAfter != tc.want || (err != nil) != tc.wantError || (tc.wantError && !errors.Is(err, f.directory.principalErr)) {
			t.Fatalf("case=%+v requeue=%v err=%v", tc, got.RequeueAfter, err)
		}
	}
}
