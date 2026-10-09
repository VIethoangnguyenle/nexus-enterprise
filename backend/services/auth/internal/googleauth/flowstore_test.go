package googleauth

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/redis/go-redis/v9"
)

// Runs only when REDIS_ADDR points at a disposable Redis.
func testFlowStore(t *testing.T) *RedisFlowStore {
	t.Helper()
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		t.Skip("REDIS_ADDR not set")
	}
	rdb := redis.NewClient(&redis.Options{Addr: addr})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		t.Skipf("redis not available: %v", err)
	}
	t.Cleanup(func() { rdb.Close() })
	return NewRedisFlowStore(rdb)
}

func TestRedisFlowStore_TakeIsOneTime(t *testing.T) {
	fs := testFlowStore(t)
	ctx := context.Background()
	secrets, err := NewFlowSecrets()
	if err != nil {
		t.Fatal(err)
	}

	if err := fs.Save(ctx, secrets.State, Flow{Nonce: secrets.Nonce, Verifier: secrets.Verifier}); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := fs.Take(ctx, secrets.State)
	if err != nil {
		t.Fatalf("take: %v", err)
	}
	if got.Nonce != secrets.Nonce || got.Verifier != secrets.Verifier {
		t.Errorf("flow = %+v", got)
	}

	// A replayed callback must find nothing.
	if _, err := fs.Take(ctx, secrets.State); !errors.Is(err, ErrFlowNotFound) {
		t.Fatalf("second take err = %v, want ErrFlowNotFound", err)
	}
}

func TestRedisFlowStore_UnknownState(t *testing.T) {
	fs := testFlowStore(t)
	if _, err := fs.Take(context.Background(), "never-issued"); !errors.Is(err, ErrFlowNotFound) {
		t.Fatalf("err = %v, want ErrFlowNotFound", err)
	}
}
