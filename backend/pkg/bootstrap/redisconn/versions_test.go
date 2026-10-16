package redisconn

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func TestVersions_BumpAndGet(t *testing.T) {
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		t.Skip("REDIS_ADDR not set")
	}
	rdb := redis.NewClient(&redis.Options{Addr: addr})
	ctx := context.Background()
	if err := rdb.Ping(ctx).Err(); err != nil {
		t.Skipf("redis unavailable: %v", err)
	}
	defer rdb.Close()

	stamp := time.Now().Format(time.RFC3339Nano)
	a, b := "ver:test:"+stamp+":a", "ver:test:"+stamp+":b"
	defer rdb.Del(ctx, a, b)
	v := NewVersions(rdb)

	got, err := v.Get(ctx, a, b)
	if err != nil || len(got) != 2 || got[0] != 0 || got[1] != 0 {
		t.Fatalf("fresh keys: got %v, %v", got, err)
	}
	if err := v.Bump(ctx, a); err != nil {
		t.Fatal(err)
	}
	if err := v.Bump(ctx, a, b); err != nil {
		t.Fatal(err)
	}
	got, err = v.Get(ctx, a, b)
	if err != nil || got[0] != 2 || got[1] != 1 {
		t.Fatalf("after bumps: got %v, %v", got, err)
	}
	if ttl := rdb.TTL(ctx, a).Val(); ttl <= 0 {
		t.Fatalf("a counter must expire eventually, ttl = %v", ttl)
	}
}
