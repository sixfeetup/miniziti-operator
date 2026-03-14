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
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestLoadManagementConfig(t *testing.T) {
	t.Parallel()

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: "miniziti-system", Name: "openziti-management"},
		Data: map[string][]byte{
			ControllerURLKey: []byte(" https://ziti.example.com/edge/management/v1 "),
			UsernameKey:      []byte(" admin "),
			PasswordKey:      []byte(" change-me "),
		},
	}

	cfg, err := LoadManagementConfig(secret)
	if err != nil {
		t.Fatalf("LoadManagementConfig returned error: %v", err)
	}

	if cfg.ControllerURL != "https://ziti.example.com/edge/management/v1" {
		t.Fatalf("unexpected controller URL: %q", cfg.ControllerURL)
	}
	if cfg.Username != "admin" {
		t.Fatalf("unexpected username: %q", cfg.Username)
	}
	if cfg.Password != "change-me" {
		t.Fatalf("unexpected password: %q", cfg.Password)
	}
}

func TestLoadManagementConfigRejectsInvalidSecret(t *testing.T) {
	t.Parallel()

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: "miniziti-system", Name: "openziti-management"},
		Data: map[string][]byte{
			ControllerURLKey: []byte("relative-path"),
			UsernameKey:      []byte("admin"),
		},
	}

	_, err := LoadManagementConfig(secret)
	if err == nil {
		t.Fatal("expected error for invalid secret")
	}
	if !strings.Contains(err.Error(), `missing "password"`) && !strings.Contains(err.Error(), `"controllerUrl" must be an absolute URL`) {
		t.Fatalf("unexpected error: %v", err)
	}
}
