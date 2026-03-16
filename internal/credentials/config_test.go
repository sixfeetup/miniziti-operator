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

func TestLoadManagementConfig(t *testing.T) {
	t.Parallel()

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: "ziti", Name: "openziti-management"},
		Data: map[string][]byte{
			ControllerURLKey: []byte(" https://ziti.example.com/edge/management/v1 "),
			UsernameKey:      []byte(" admin "),
			PasswordKey:      []byte(" change-me "),
			CABundleKey:      []byte(validTestCABundlePEM),
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
	if len(cfg.CABundlePEM) == 0 {
		t.Fatal("expected ca bundle to be loaded")
	}
}

func TestLoadManagementConfigRejectsInvalidSecret(t *testing.T) {
	t.Parallel()

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Namespace: "ziti", Name: "openziti-management"},
		Data: map[string][]byte{
			ControllerURLKey: []byte("relative-path"),
			UsernameKey:      []byte("admin"),
			CABundleKey:      []byte("not pem"),
		},
	}

	_, err := LoadManagementConfig(secret)
	if err == nil {
		t.Fatal("expected error for invalid secret")
	}
	if !strings.Contains(err.Error(), `missing "password"`) &&
		!strings.Contains(err.Error(), `"controllerUrl" must be an absolute URL`) &&
		!strings.Contains(err.Error(), `"caBundle" must contain valid PEM certificates`) {
		t.Fatalf("unexpected error: %v", err)
	}
}
