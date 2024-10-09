package token

import (
	"context"
	cryptorand "crypto/rand"
	"encoding/base64"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	AuthTokenNameSuffix = "capi-auth-token"
)

// Reconcile ensures that a token secret exists for the given cluster.
func Reconcile(ctx context.Context, c client.Client, clusterKey client.ObjectKey) error {
	if _, err := getSecret(ctx, c, clusterKey); err != nil {
		if apierrors.IsNotFound(err) {
			if _, err := generateAndStore(ctx, c, clusterKey); err != nil {
				return fmt.Errorf("failed to generate and store token: %w", err)
			}
			return nil
		}
	}

	return nil
}

// Lookup retrieves the token for the given cluster.
func Lookup(ctx context.Context, c client.Client, clusterKey client.ObjectKey) (string, error) {
	secret, err := getSecret(ctx, c, clusterKey)
	if err != nil {
		return "", fmt.Errorf("failed to get secret: %w", err)
	}

	v, ok := secret.Data["token"]
	if !ok {
		return "", fmt.Errorf("token not found in secret")
	}

	return string(v), nil
}

// name returns the name of the token secret, computed by convention using the name of the cluster.
func authTokenName(clusterName string) string {
	return fmt.Sprintf("%s-%s", clusterName, AuthTokenNameSuffix)
}

// getSecret retrieves the token secret for the given cluster.
func getSecret(ctx context.Context, c client.Client, clusterKey client.ObjectKey) (*corev1.Secret, error) {
	s := &corev1.Secret{}
	key := client.ObjectKey{
		Name:      authTokenName(clusterKey.Name),
		Namespace: clusterKey.Namespace,
	}
	if err := c.Get(ctx, key, s); err != nil {
		return nil, fmt.Errorf("failed to get secret: %w", err)
	}

	return s, nil
}

// generateAndStore generates a new token and stores it in a secret.
func generateAndStore(ctx context.Context, c client.Client, clusterKey client.ObjectKey) (*corev1.Secret, error) {
	token, err := randomB64(16)
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %v", err)
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: clusterKey.Namespace,
			Name:      authTokenName(clusterKey.Name),
		},
		StringData: map[string]string{
			"token": token,
		},
	}

	if err := c.Create(ctx, secret); err != nil {
		return nil, fmt.Errorf("failed to create secret: %v", err)
	}

	return secret, nil
}

// randomB64 generates a random base64 string of n bytes.
func randomB64(n int) (string, error) {
	b := make([]byte, n)
	_, err := cryptorand.Read(b)
	if err != nil {
		return "", fmt.Errorf("failed to read random bytes: %v", err)
	}
	return base64.StdEncoding.EncodeToString(b), nil
}
