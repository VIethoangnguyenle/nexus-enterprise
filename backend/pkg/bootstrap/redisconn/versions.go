package redisconn

import (
	"context"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// versionLifetime lets a counter nobody read for a month go: the next read sees
// 0 and the ETag derived from it differs anyway.
const versionLifetime = 30 * 24 * time.Hour

// Versions keeps the change counters behind httputil.ConditionalGET in Redis, so
// every instance of a service sees the same ones. It satisfies httputil.Versions.
//
// A service with no Redis wires no Versions at all (it must not wrap a nil
// client): with no counters there are no ETags and every response is a full 200.
type Versions struct{ rdb *redis.Client }

// NewVersions returns the counters stored in rdb, which must not be nil.
func NewVersions(rdb *redis.Client) *Versions { return &Versions{rdb: rdb} }

// Get returns the current value of each key, in order; a key never bumped is 0.
func (v *Versions) Get(ctx context.Context, keys ...string) ([]int64, error) {
	vals, err := v.rdb.MGet(ctx, keys...).Result()
	if err != nil {
		return nil, err
	}
	out := make([]int64, len(vals))
	for i, raw := range vals {
		s, ok := raw.(string)
		if !ok {
			continue // never bumped
		}
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return nil, err
		}
		out[i] = n
	}
	return out, nil
}

// Bump advances every key.
func (v *Versions) Bump(ctx context.Context, keys ...string) error {
	pipe := v.rdb.Pipeline()
	for _, k := range keys {
		pipe.Incr(ctx, k)
		pipe.Expire(ctx, k, versionLifetime)
	}
	_, err := pipe.Exec(ctx)
	return err
}
