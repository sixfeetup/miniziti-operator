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

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	zitiv1alpha1 "example.com/miniziti-operator/api/v1alpha1"
)

const EnrollmentJWTKey = "jwt"

// ReconcileEnrollmentSecret ensures the requested JWT Secret exists and is owned by the identity.
func ReconcileEnrollmentSecret(
	ctx context.Context,
	kubeClient client.Client,
	scheme *runtime.Scheme,
	identity *zitiv1alpha1.ZitiIdentity,
	jwt string,
) (string, error) {
	name := identity.Spec.Enrollment.JWTSecretName
	if name == "" {
		return "", fmt.Errorf("jwt secret name is required when enrollment secret output is enabled")
	}

	key := types.NamespacedName{Namespace: identity.Namespace, Name: name}
	var secret corev1.Secret
	err := kubeClient.Get(ctx, key, &secret)
	if err == nil {
		if !metav1.IsControlledBy(&secret, identity) {
			return "", fmt.Errorf("enrollment secret %s is not owned by %s", key.String(), identity.Name)
		}
		return secret.Name, nil
	}
	if !apierrors.IsNotFound(err) {
		return "", err
	}

	secret = corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: identity.Namespace,
		},
		Type: corev1.SecretTypeOpaque,
		StringData: map[string]string{
			EnrollmentJWTKey: jwt,
		},
	}
	if err := ctrl.SetControllerReference(identity, &secret, scheme); err != nil {
		return "", err
	}
	if err := kubeClient.Create(ctx, &secret); err != nil {
		return "", err
	}
	return secret.Name, nil
}

// DeleteEnrollmentSecret removes an operator-owned enrollment Secret when it exists.
func DeleteEnrollmentSecret(
	ctx context.Context,
	kubeClient client.Client,
	identity *zitiv1alpha1.ZitiIdentity,
	secretName string,
) error {
	if secretName == "" {
		return nil
	}

	key := types.NamespacedName{Namespace: identity.Namespace, Name: secretName}
	var secret corev1.Secret
	if err := kubeClient.Get(ctx, key, &secret); err != nil {
		return client.IgnoreNotFound(err)
	}
	if !metav1.IsControlledBy(&secret, identity) {
		return nil
	}
	return client.IgnoreNotFound(kubeClient.Delete(ctx, &secret))
}
