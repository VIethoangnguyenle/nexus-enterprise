package httputil_test

import (
	"testing"

	"ngac-platform/pkg/httputil"
)

func TestRequireJWTSecret(t *testing.T) {
	const realSecret = "a-real-deployment-secret"

	cases := []struct {
		name      string
		appEnv    string
		secret    string
		wantError bool
	}{
		{"real secret in production", "production", realSecret, false},
		{"real secret with no APP_ENV", "", realSecret, false},
		{"placeholder in dev", "dev", httputil.DevJWTSecret, false},
		{"placeholder in local", "local", httputil.DevJWTSecret, false},
		{"placeholder in test", "test", httputil.DevJWTSecret, false},

		// The failures that matter: shipping the repo's own secret.
		{"placeholder in production", "production", httputil.DevJWTSecret, true},
		{"placeholder in staging", "staging", httputil.DevJWTSecret, true},
		{"placeholder with APP_ENV unset", "", httputil.DevJWTSecret, true},
		{"placeholder with a typo'd APP_ENV", "prod", httputil.DevJWTSecret, true},

		{"empty secret", "dev", "", true},
		{"empty secret in production", "production", "", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("APP_ENV", tc.appEnv)
			err := httputil.RequireJWTSecret(tc.secret)
			if tc.wantError && err == nil {
				t.Error("expected the service to refuse to start, got no error")
			}
			if !tc.wantError && err != nil {
				t.Errorf("expected the service to start, got: %v", err)
			}
		})
	}
}
