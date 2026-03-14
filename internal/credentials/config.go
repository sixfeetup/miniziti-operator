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

package credentials

import (
	"fmt"
	"net/url"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

const (
	ControllerURLKey = "controllerUrl"
	UsernameKey      = "username"
	PasswordKey      = "password"
)

// SecretRef identifies the namespaced Secret that stores management credentials.
type SecretRef struct {
	Namespace string
	Name      string
}

// NamespacedName returns the Secret reference as a Kubernetes namespaced name.
func (r SecretRef) NamespacedName() types.NamespacedName {
	return types.NamespacedName{Namespace: r.Namespace, Name: r.Name}
}

// ManagementConfig contains the operator's runtime credentials for the OpenZiti management API.
type ManagementConfig struct {
	ControllerURL string
	Username      string
	Password      string
}

// LoadManagementConfig extracts and validates management credentials from a Secret.
func LoadManagementConfig(secret *corev1.Secret) (ManagementConfig, error) {
	if secret == nil {
		return ManagementConfig{}, fmt.Errorf("management secret is required")
	}

	cfg := ManagementConfig{
		ControllerURL: strings.TrimSpace(string(secret.Data[ControllerURLKey])),
		Username:      strings.TrimSpace(string(secret.Data[UsernameKey])),
		Password:      strings.TrimSpace(string(secret.Data[PasswordKey])),
	}

	if err := cfg.Validate(); err != nil {
		return ManagementConfig{}, fmt.Errorf("invalid management secret %s/%s: %w", secret.Namespace, secret.Name, err)
	}

	return cfg, nil
}

// Validate ensures the credential payload is complete and syntactically usable.
func (c ManagementConfig) Validate() error {
	if c.ControllerURL == "" {
		return fmt.Errorf("missing %q", ControllerURLKey)
	}
	parsed, err := url.Parse(c.ControllerURL)
	if err != nil {
		return fmt.Errorf("parse %q: %w", ControllerURLKey, err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return fmt.Errorf("%q must be an absolute URL", ControllerURLKey)
	}
	if c.Username == "" {
		return fmt.Errorf("missing %q", UsernameKey)
	}
	if c.Password == "" {
		return fmt.Errorf("missing %q", PasswordKey)
	}
	return nil
}
