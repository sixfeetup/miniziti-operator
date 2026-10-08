package entra

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/oauth2"
)

var (
	ErrServicePrincipalNotFound = errors.New("entra application service principal not found")
	ErrLimitedUserData          = errors.New("graph user data is incomplete; grant User.ReadBasic.All")
	aadCode                     = regexp.MustCompile(`AADSTS[0-9]+`)
)

type GraphError struct {
	StatusCode    int
	Code, Message string
	RetryAfter    time.Duration
}

func (e *GraphError) Error() string {
	return fmt.Sprintf("Graph HTTP %d %s: %s", e.StatusCode, e.Code, e.Message)
}

type TokenError struct{ Code, Message string }

func (e *TokenError) Error() string { return "Entra token " + e.Code + ": " + e.Message }

func sanitize(s string, secrets ...string) string {
	for _, secret := range secrets {
		if secret != "" {
			s = strings.ReplaceAll(s, secret, "[redacted]")
		}
	}
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > 512 {
		s = s[:512]
	}
	return s
}

func safeTokenError(err error, secret string) error {
	var retrieve *oauth2.RetrieveError
	if !errors.As(err, &retrieve) {
		if errors.Is(err, context.Canceled) {
			return context.Canceled
		}
		if errors.Is(err, context.DeadlineExceeded) {
			return context.DeadlineExceeded
		}
		return errors.New("entra token request failed")
	}
	var body struct {
		Code         string `json:"error"`
		Description  string `json:"error_description"`
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		IDToken      string `json:"id_token"`
	}
	_ = json.Unmarshal(retrieve.Body, &body)
	code := aadCode.FindString(body.Description)
	if code == "" {
		code = body.Code
	}
	description := body.Description
	// Drop unstable diagnostics even when the provider omits sentence punctuation.
	for _, marker := range []string{"Trace ID:", "Correlation ID:", "Timestamp:"} {
		if i := strings.Index(description, marker); i >= 0 {
			description = description[:i]
		}
	}
	if i := strings.Index(description, ". "); i >= 0 {
		description = description[:i+1]
	}
	return &TokenError{Code: sanitize(code, secret, body.AccessToken, body.RefreshToken, body.IDToken), Message: sanitize(description, secret, body.AccessToken, body.RefreshToken, body.IDToken)}
}

func (d *directory) responseError(resp *http.Response, token string) error {
	var body struct {
		Error struct{ Code, Message string } `json:"error"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&body)
	result := &GraphError{StatusCode: resp.StatusCode, Code: sanitize(body.Error.Code, d.config.ClientSecret, token), Message: sanitize(body.Error.Message, d.config.ClientSecret, token)}
	if result.Message == "" {
		result.Message = http.StatusText(resp.StatusCode)
	}
	if resp.StatusCode == 429 || resp.StatusCode == 503 {
		value := resp.Header.Get("Retry-After")
		if seconds, err := strconv.ParseInt(value, 10, 64); err == nil && seconds > 0 && seconds <= int64((time.Duration(1<<63-1))/time.Second) {
			result.RetryAfter = time.Duration(seconds) * time.Second
		} else if retry, err := http.ParseTime(value); err == nil {
			result.RetryAfter = retry.Sub(d.now())
			if result.RetryAfter < 0 {
				result.RetryAfter = 0
			}
		}
	}
	return result
}
