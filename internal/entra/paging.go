package entra

import (
	"context"
	"errors"
	"net/url"
)

// An exact origin check also rejects credentials, suffix hosts, and nonstandard ports.
func (d *directory) validEndpoint(endpoint string) bool {
	u, err := url.Parse(endpoint)
	if err != nil {
		return false
	}
	base, err := url.Parse(d.baseURL)
	if err != nil {
		return false
	}
	return u.Scheme == base.Scheme && u.Host == base.Host && u.User == nil && u.Fragment == ""
}

func readPages[T any](ctx context.Context, d *directory, endpoint string) ([]T, error) {
	result := []T{}
	seen := map[string]bool{}
	for endpoint != "" {
		if seen[endpoint] {
			return nil, errors.New("graph pagination cycle")
		}
		seen[endpoint] = true
		var page struct {
			Value *[]T   `json:"value"`
			Next  string `json:"@odata.nextLink"`
		}
		if err := d.get(ctx, endpoint, &page); err != nil {
			return nil, err
		}
		if page.Value == nil {
			return nil, errors.New("graph page has no value array")
		}
		result = append(result, (*page.Value)...)
		endpoint = page.Next
	}
	return result, nil
}
