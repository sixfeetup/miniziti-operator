package entra

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

const graphURL = "https://graph.microsoft.com/v1.0"

// clientOptions is private: custom endpoints are only available to package tests.
type clientOptions struct {
	graphURL, loginURL string
	transport          http.RoundTripper
	now                func() time.Time
}
type directory struct {
	http    *http.Client
	config  clientcredentials.Config
	baseURL string
	now     func() time.Time
	mu      sync.Mutex
	source  oauth2.TokenSource
}

func NewDirectory(tenantID, clientID, clientSecret string) Directory {
	return newDirectory(tenantID, clientID, clientSecret, clientOptions{})
}
func newDirectory(tenantID, clientID, clientSecret string, opts clientOptions) *directory {
	if opts.graphURL == "" {
		opts.graphURL = graphURL
	}
	if opts.loginURL == "" {
		opts.loginURL = "https://login.microsoftonline.com"
	}
	if opts.now == nil {
		opts.now = time.Now
	}
	return &directory{
		http:    &http.Client{Transport: opts.transport, Timeout: 30 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }},
		config:  clientcredentials.Config{ClientID: clientID, ClientSecret: clientSecret, TokenURL: opts.loginURL + "/" + url.PathEscape(tenantID) + "/oauth2/v2.0/token", Scopes: []string{"https://graph.microsoft.com/.default"}, AuthStyle: oauth2.AuthStyleInParams},
		baseURL: strings.TrimRight(opts.graphURL, "/"), now: opts.now,
	}
}

func (d *directory) token(ctx context.Context) (*oauth2.Token, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.source == nil {
		d.source = d.config.TokenSource(context.WithValue(ctx, oauth2.HTTPClient, d.http))
	}
	token, err := d.source.Token()
	if err != nil {
		return nil, safeTokenError(err, d.config.ClientSecret)
	}
	return token, nil
}

func (d *directory) get(ctx context.Context, endpoint string, out any) error {
	if !d.validEndpoint(endpoint) {
		return errors.New("graph endpoint is outside the allowed origin")
	}
	token, err := d.token(ctx)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return errors.New("invalid Graph request")
	}
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	resp, err := d.http.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return errors.New("graph request failed")
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return d.responseError(resp, token.AccessToken)
	}
	decoder := json.NewDecoder(resp.Body)
	if err := decoder.Decode(out); err != nil {
		return errors.New("invalid Graph JSON response")
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return errors.New("graph response has trailing data")
	}
	return nil
}

func (d *directory) GetServicePrincipal(ctx context.Context, appID string) (*ServicePrincipal, error) {
	endpoint := d.baseURL + "/servicePrincipals(appId='" + url.PathEscape(appID) + "')?$select=id,appRoles"
	var result ServicePrincipal
	if err := d.get(ctx, endpoint, &result); err != nil {
		var ge *GraphError
		if errors.As(err, &ge) && ge.StatusCode == 404 {
			return nil, ErrServicePrincipalNotFound
		}
		return nil, err
	}
	if result.ID == "" {
		return nil, errors.New("graph response has no service principal ID")
	}
	return &result, nil
}

func (d *directory) ListAppRoleAssignedTo(ctx context.Context, id string) ([]AppRoleAssignment, error) {
	return readPages[AppRoleAssignment](ctx, d, d.baseURL+"/servicePrincipals/"+url.PathEscape(id)+"/appRoleAssignedTo")
}

func (d *directory) ListGroupUsers(ctx context.Context, id string) ([]User, error) {
	type member struct {
		User
		Type string `json:"@odata.type"`
	}
	members, err := readPages[member](ctx, d, d.baseURL+"/groups/"+url.PathEscape(id)+"/members")
	if err != nil {
		return nil, err
	}
	users := []User{}
	for _, m := range members {
		if m.Type != "#microsoft.graph.user" {
			continue
		}
		if m.UserPrincipalName == "" {
			return nil, ErrLimitedUserData
		}
		users = append(users, m.User)
	}
	return users, nil
}
