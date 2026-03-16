/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package client

import (
	"context"
	"errors"
	"testing"

	"github.com/openziti/edge-api/rest_management_api_client"
	"github.com/openziti/edge-api/rest_model"

	"example.com/miniziti-operator/internal/credentials"
)

const validTestCABundlePEM = `-----BEGIN CERTIFICATE-----
MIIBiTCCATCgAwIBAgIUeJJrt+qagkXkEt8LX66XMGkr8WYwCgYIKoZIzj0EAwIw
IzEhMB8GA1UEAxMYeml0aS1jb250cm9sbGVyLXdlYi1yb290MB4XDTI2MDMxNjA3
MDAzNFoXDTM2MDMyMzA3MDAzNFowIzEhMB8GA1UEAxMYeml0aS1jb250cm9sbGVy
LXdlYi1yb290MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEbpWW03nmrlRoitX2
gapjY7wOKp3HA8BvUogsQJNSgzkmhWGA8CCtv/pExPPg8GuzsABui5HJgP0pkrWH
FSjwqKNCMEAwDgYDVR0PAQH/BAQDAgGGMA8GA1UdEwEB/wQFMAMBAf8wHQYDVR0O
BBYEFINKpkWO22ymwAXHEP7NPh6w1+M7MAoGCCqGSM49BAMCA0cAMEQCIFfRzhuI
4FDGio1RB4uoUpIPbiIalZ8+1VQn0vieX/iTAiAw4KsHOdnY6fK8PI5pkXMo72jD
MP8Q0ipOOsgCxcLg0Q==
-----END CERTIFICATE-----
`

type codedError struct {
	code int
}

func (e codedError) Error() string {
	return "coded error"
}

func (e codedError) Code() int {
	return e.code
}

func TestUseAuthenticatedClientReauthenticatesOnUnauthorized(t *testing.T) {
	ctx := context.Background()
	cfg := credentials.ManagementConfig{
		ControllerURL: "https://controller.example.com/edge/management/v1",
		Username:      "admin",
		Password:      "secret",
	}

	authCalls := 0
	client := &ManagementClient{
		loadConfig: func(context.Context) (credentials.ManagementConfig, error) {
			return cfg, nil
		},
		authenticate: func(context.Context, credentials.ManagementConfig) (*rest_management_api_client.ZitiEdgeManagement, error) {
			authCalls++
			return &rest_management_api_client.ZitiEdgeManagement{}, nil
		},
	}

	runCalls := 0
	result, err := useAuthenticatedClient(ctx, client, client.loadCurrentConfig, func(*rest_management_api_client.ZitiEdgeManagement) (string, error) {
		runCalls++
		if runCalls == 1 {
			return "", codedError{code: 401}
		}
		return "ok", nil
	})
	if err != nil {
		t.Fatalf("useAuthenticatedClient returned error: %v", err)
	}
	if result != "ok" {
		t.Fatalf("useAuthenticatedClient returned %q", result)
	}
	if authCalls != 2 {
		t.Fatalf("expected 2 authentications, got %d", authCalls)
	}
	if runCalls != 2 {
		t.Fatalf("expected 2 operation attempts, got %d", runCalls)
	}
}

func TestUseAuthenticatedClientReturnsOperationError(t *testing.T) {
	ctx := context.Background()
	expectedErr := errors.New("boom")
	cfg := credentials.ManagementConfig{
		ControllerURL: "https://controller.example.com/edge/management/v1",
		Username:      "admin",
		Password:      "secret",
	}

	client := &ManagementClient{
		loadConfig: func(context.Context) (credentials.ManagementConfig, error) {
			return cfg, nil
		},
		authenticate: func(context.Context, credentials.ManagementConfig) (*rest_management_api_client.ZitiEdgeManagement, error) {
			return &rest_management_api_client.ZitiEdgeManagement{}, nil
		},
	}

	_, err := useAuthenticatedClient(ctx, client, client.loadCurrentConfig, func(*rest_management_api_client.ZitiEdgeManagement) (string, error) {
		return "", expectedErr
	})
	if !errors.Is(err, expectedErr) {
		t.Fatalf("useAuthenticatedClient returned %v, want %v", err, expectedErr)
	}
}

func TestRootCAsFromConfig(t *testing.T) {
	pool, err := rootCAsFromConfig(credentials.ManagementConfig{
		CABundlePEM: []byte(validTestCABundlePEM),
	})
	if err != nil {
		t.Fatalf("rootCAsFromConfig returned error: %v", err)
	}
	if pool == nil {
		t.Fatal("expected cert pool")
	}
}

func TestToIdentityCreateIncludesOTTEnrollment(t *testing.T) {
	created := toIdentityCreate(Identity{
		Name:      "alice@example.com",
		Type:      "User",
		CreateOTT: true,
	})

	if created.Enrollment == nil || !created.Enrollment.Ott {
		t.Fatalf("expected OTT enrollment, got %#v", created.Enrollment)
	}
}

func TestToServiceCreateIncludesConfigIDs(t *testing.T) {
	created := toServiceCreate(Service{
		Name:      "argocd",
		ConfigIDs: []string{"cfg-intercept", "cfg-host"},
	})

	if len(created.Configs) != 2 {
		t.Fatalf("expected 2 config ids, got %#v", created.Configs)
	}
}

func TestToConfigCreateUsesResolvedConfigTypeID(t *testing.T) {
	created := toConfigCreate(ServiceConfig{
		Name:    "argocd-intercept",
		Payload: map[string]any{"addresses": []string{"argocd.ziti"}},
	}, "resolved-config-type-id")

	if created.ConfigTypeID == nil || *created.ConfigTypeID != "resolved-config-type-id" {
		t.Fatalf("expected resolved config type id, got %#v", created.ConfigTypeID)
	}
	if created.Data == nil {
		t.Fatal("expected config payload data")
	}
}

func TestServiceFromEnvelopeIncludesConfigIDs(t *testing.T) {
	service := serviceFromEnvelope(&rest_model.DetailServiceEnvelope{
		Data: &rest_model.ServiceDetail{
			BaseEntity: rest_model.BaseEntity{ID: stringPtr("service-1")},
			Name:       stringPtr("argocd"),
			Configs:    []string{"cfg-intercept", "cfg-host"},
		},
	})

	if service == nil {
		t.Fatal("expected service")
	}
	if len(service.ConfigIDs) != 2 {
		t.Fatalf("expected config ids, got %#v", service.ConfigIDs)
	}
}

func stringPtr(value string) *string {
	return &value
}
