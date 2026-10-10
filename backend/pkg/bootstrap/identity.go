package bootstrap

import (
	"fmt"
	"os"

	"ngac-platform/pkg/grpcauth"
)

// Environment variables for the secret that signs service-to-service identity
// tokens (see package grpcauth).
const (
	InternalIdentitySecretEnv         = "INTERNAL_IDENTITY_SECRET"
	InternalIdentitySecretPreviousEnv = "INTERNAL_IDENTITY_SECRET_PREVIOUS"
)

// DevInternalIdentitySecret is what a development process falls back to when
// INTERNAL_IDENTITY_SECRET is unset. It is committed to this repository, so it
// is public: it protects nothing and is refused outside development.
const DevInternalIdentitySecret = "ngac-dev-internal-identity-secret-not-for-production"

// devEnvironments are the APP_ENV values that permit the committed
// placeholder secrets.
var devEnvironments = map[string]bool{
	"dev": true, "development": true, "local": true, "test": true,
}

// IsDevEnvironment reports whether APP_ENV names a development environment.
func IsDevEnvironment() bool {
	return devEnvironments[os.Getenv("APP_ENV")]
}

// ConfigureInternalIdentity reads the identity-signing secret from the
// environment and installs it in grpcauth. Every service calls it first thing
// in main and exits on error.
//
// Outside development the secret is required, must not be the committed
// placeholder, and must differ from JWT_SECRET: a leaked user-token secret
// must not also let an attacker mint service identities. A development
// process with the variable unset uses the placeholder.
// INTERNAL_IDENTITY_SECRET_PREVIOUS, when set, is accepted for verification
// only, so the secret can be rotated service by service.
func ConfigureInternalIdentity() error {
	secret := os.Getenv(InternalIdentitySecretEnv)
	if secret == "" {
		if !IsDevEnvironment() {
			return fmt.Errorf("%s is not set (it is required unless APP_ENV is dev/development/local/test; APP_ENV=%q)",
				InternalIdentitySecretEnv, os.Getenv("APP_ENV"))
		}
		secret = DevInternalIdentitySecret
	}
	if secret == DevInternalIdentitySecret && !IsDevEnvironment() {
		return fmt.Errorf("refusing the placeholder %s committed to this repository outside development (APP_ENV=%q)",
			InternalIdentitySecretEnv, os.Getenv("APP_ENV"))
	}
	if jwt := os.Getenv("JWT_SECRET"); jwt != "" && jwt == secret {
		return fmt.Errorf("%s must differ from JWT_SECRET", InternalIdentitySecretEnv)
	}
	previous := os.Getenv(InternalIdentitySecretPreviousEnv)
	if jwt := os.Getenv("JWT_SECRET"); jwt != "" && jwt == previous {
		return fmt.Errorf("%s must differ from JWT_SECRET", InternalIdentitySecretPreviousEnv)
	}
	if previous == DevInternalIdentitySecret && !IsDevEnvironment() {
		return fmt.Errorf("refusing the placeholder %s outside development", InternalIdentitySecretPreviousEnv)
	}
	if err := grpcauth.Configure(grpcauth.Keys{Current: secret, Previous: previous}); err != nil {
		return fmt.Errorf("%s: %w", InternalIdentitySecretEnv, err)
	}
	return nil
}
