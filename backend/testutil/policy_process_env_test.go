//go:build realpolicy

package testutil

import (
	"strings"
	"testing"
)

// The environment is built from nothing: a REDIS_URL or KAFKA_BROKERS in the
// developer's shell must not reach the policy process.
func TestPolicyEnvIgnoresTheCallersEnvironment(t *testing.T) {
	t.Setenv("REDIS_URL", "redis://dev-redis:6379/0")
	t.Setenv("KAFKA_BROKERS", "dev-redpanda:19092")

	env := strings.Join(policyEnv("postgres://x", "1111", "2222"), "\n")
	for _, bad := range []string{"dev-redis", "dev-redpanda", "6379", "19092"} {
		if strings.Contains(env, bad) {
			t.Errorf("policy environment mentions %q:\n%s", bad, env)
		}
	}
	for _, want := range []string{"REDIS_URL=" + unreachableRedis, "KAFKA_BROKERS=" + unreachableKafka, "GRPC_PORT=1111", "METRICS_PORT=2222", "DATABASE_URL=postgres://x"} {
		if !strings.Contains(env, want) {
			t.Errorf("policy environment lacks %q:\n%s", want, env)
		}
	}
}
