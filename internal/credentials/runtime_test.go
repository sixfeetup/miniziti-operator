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

import "testing"

func TestSecretRefFromEnv(t *testing.T) {
	t.Setenv(ManagementSecretNameEnv, "openziti-management")
	t.Setenv(ManagementSecretNamespaceEnv, "ziti")

	ref, err := SecretRefFromEnv()
	if err != nil {
		t.Fatalf("SecretRefFromEnv returned error: %v", err)
	}

	if ref.Name != "openziti-management" || ref.Namespace != "ziti" {
		t.Fatalf("SecretRefFromEnv returned %+v", ref)
	}
}

func TestSecretRefFromEnvRequiresValues(t *testing.T) {
	t.Setenv(ManagementSecretNameEnv, "")
	t.Setenv(ManagementSecretNamespaceEnv, "")

	if _, err := SecretRefFromEnv(); err == nil {
		t.Fatal("SecretRefFromEnv returned nil error for empty env")
	}
}
