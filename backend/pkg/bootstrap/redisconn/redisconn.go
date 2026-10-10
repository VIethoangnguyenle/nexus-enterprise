// Package redisconn opens the Redis client the services use. It is separate
// from bootstrap so a service that has no Redis does not link the client.
package redisconn

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/redis/go-redis/v9"
)

// Connect creates a client from a redis:// URL and verifies it can reach the
// server. On error no client is returned.
func Connect(ctx context.Context, redisURL string) (*redis.Client, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("parsing redis url: %w", err)
	}
	rdb := redis.NewClient(opts)
	if err := rdb.Ping(ctx).Err(); err != nil {
		_ = rdb.Close()
		return nil, fmt.Errorf("pinging redis: %w", err)
	}
	slog.Info("redis connected", "addr", opts.Addr, "db", opts.DB)
	return rdb, nil
}
