package httputil

import (
	"fmt"
	"os"
)

// DevJWTSecret is the placeholder every service falls back to when JWT_SECRET
// is unset. It is committed to this repository, so it is public: anyone can
// forge a token for any user of any tenant with it.
const DevJWTSecret = "ngac-super-secret-key-change-in-production"

// devEnvironments are the APP_ENV values that permit the placeholder secret.
var devEnvironments = map[string]bool{
	"dev": true, "development": true, "local": true, "test": true,
}

// RequireJWTSecret returns the signing secret, or an error explaining why the
// process must not start.
//
// Outside a development environment this refuses the placeholder rather than
// warning about it. A warning in a startup log is not a control: the service
// comes up, serves traffic, and accepts forged tokens for as long as nobody
// reads the log. Failing to boot converts a silent authentication bypass into
// an obvious deployment error.
func RequireJWTSecret(secret string) error {
	if secret == "" {
		return fmt.Errorf("JWT_SECRET is not set")
	}
	if secret != DevJWTSecret {
		return nil
	}
	if devEnvironments[os.Getenv("APP_ENV")] {
		return nil
	}
	return fmt.Errorf(
		"refusing to start with the placeholder JWT_SECRET committed to this repository: "+
			"set JWT_SECRET, or set APP_ENV to one of dev/development/local/test (APP_ENV=%q)",
		os.Getenv("APP_ENV"),
	)
}
