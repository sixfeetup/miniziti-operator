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
	"context"
	"fmt"
	"os"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	ManagementSecretNameEnv      = "MINIZITI_MANAGEMENT_SECRET_NAME"
	ManagementSecretNamespaceEnv = "MINIZITI_MANAGEMENT_SECRET_NAMESPACE"
)

// SecretRefFromEnv resolves the management Secret reference from the operator runtime environment.
func SecretRefFromEnv() (SecretRef, error) {
	name := strings.TrimSpace(os.Getenv(ManagementSecretNameEnv))
	if name == "" {
		return SecretRef{}, fmt.Errorf("%s must not be empty", ManagementSecretNameEnv)
	}

	namespace := strings.TrimSpace(os.Getenv(ManagementSecretNamespaceEnv))
	if namespace == "" {
		return SecretRef{}, fmt.Errorf("%s must not be empty", ManagementSecretNamespaceEnv)
	}

	return SecretRef{
		Namespace: namespace,
		Name:      name,
	}, nil
}

// SecretConfigLoader reads and validates the operator's management credentials from a Secret.
type SecretConfigLoader struct {
	reader client.Reader
	ref    SecretRef
}

// NewSecretConfigLoader creates a Secret-backed management credential loader.
func NewSecretConfigLoader(reader client.Reader, ref SecretRef) *SecretConfigLoader {
	return &SecretConfigLoader{
		reader: reader,
		ref:    ref,
	}
}

// Load fetches the referenced Secret and converts it into runtime credentials.
func (l *SecretConfigLoader) Load(ctx context.Context) (ManagementConfig, error) {
	var secret corev1.Secret
	if err := l.reader.Get(ctx, l.ref.NamespacedName(), &secret); err != nil {
		return ManagementConfig{}, fmt.Errorf("load management secret %s/%s: %w", l.ref.Namespace, l.ref.Name, err)
	}

	return LoadManagementConfig(&secret)
}
