package domain_test

import (
	"context"
	"fmt"
	"os"
	"sort"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"ngac-platform/services/auth/internal/auth"
	"ngac-platform/services/auth/internal/domain"
	"ngac-platform/services/auth/internal/store"
)

// provingSender is a sender that, like a mail provider, reaches the owner.
type provingSender struct{ recordingSender }

func (*provingSender) DeliversToOwner() bool { return true }

type stepTimes map[string][]time.Duration

func (s stepTimes) time(name string, f func()) {
	start := time.Now()
	f()
	s[name] = append(s[name], time.Since(start))
}

func (s stepTimes) report(t *testing.T) {
	names := make([]string, 0, len(s))
	for n := range s {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		d := append([]time.Duration(nil), s[n]...)
		sort.Slice(d, func(i, j int) bool { return d[i] < d[j] })
		t.Logf("%-28s n=%-3d median=%-10v p95=%-10v max=%v", n, len(d), d[len(d)/2], d[len(d)*95/100], d[len(d)-1])
	}
}

// TestOTPVerify_StepTimings times each step of POST /otp/verify against the
// development Postgres and Redis. It is a measurement, not a check, so it only
// runs when OTP_TIMING=1:
//
//	OTP_TIMING=1 TEST_DATABASE_URL=... REDIS_ADDR=... go test ./internal/domain -run StepTimings -v
func TestOTPVerify_StepTimings(t *testing.T) {
	if os.Getenv("OTP_TIMING") != "1" {
		t.Skip("set OTP_TIMING=1 to measure")
	}
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := pool.Ping(ctx); err != nil {
		t.Skipf("database not available: %v", err)
	}
	rdb := testRedisClient(t)

	var email, userID, username, node string
	err = pool.QueryRow(ctx, `
		SELECT u.email, u.id, u.username, COALESCE(u.ngac_node,'')
		  FROM users u JOIN tenant_users tu ON tu.user_id = u.id AND tu.status = 'active'
		 WHERE u.email IS NOT NULL AND u.email <> ''
		 ORDER BY u.created_at LIMIT 1`).Scan(&email, &userID, &username, &node)
	if err != nil {
		t.Skipf("no user with an email and a workspace in this database: %v", err)
	}

	auth.SetJWTSecret("test-secret-key-for-testing-only")
	st := store.New(pool)
	sender := &provingSender{}
	svc := domain.NewService(st, rdb, nil, nil, nil, nil)
	svc.ConfigureOTP(domain.OTPOptions{FixedCode: "", Sender: sender})

	const rounds = 30
	times := stepTimes{}
	for i := 0; i < rounds; i++ {
		// A distinct address per round would need a user each; instead clear the
		// per-address request counter so the limit does not interfere.
		for _, k := range rdb.Keys(ctx, "otp_rl:*").Val() {
			rdb.Del(ctx, k)
		}
		var sid string
		times.time("1 request otp (queue-less)", func() {
			if sid, err = svc.RequestOTP(ctx, email, "email"); err != nil {
				t.Fatal(err)
			}
		})
		code := sender.last()
		times.time("2 VerifyOTP total", func() {
			if _, err = svc.VerifyOTP(ctx, sid, code); err != nil {
				t.Fatal(err)
			}
		})
		times.time("2a redis GET session", func() { rdb.Get(ctx, "otp:none") })
		times.time("2b redis INCR+EXPIRE+DEL", func() { timeRedisAttempt(ctx, rdb, fmt.Sprintf("otp_attempts:timing-%d", i)) })
		times.time("2c db GetUserByEmail", func() { _, _ = st.GetUserByEmail(ctx, email) })
		times.time("2d db MarkEmailVerified", func() { _, _ = st.MarkEmailVerified(ctx, userID) })
		times.time("2e db ListTenantsByUser", func() { _, _ = st.ListTenantsByUser(ctx, userID) })
		times.time("2f sign access token", func() { _, _, _ = auth.GenerateToken(userID, username, node, "tenant") })
		times.time("3 AttachRefreshToken", func() {
			if _, err = svc.AttachRefreshToken(ctx, domain.RefreshIdentity{
				UserID: userID, Username: username, NGACNodeID: node, TenantID: "t", SessionID: fmt.Sprintf("s-%d", i),
			}); err != nil {
				t.Fatal(err)
			}
		})
	}
	times.report(t)
}

func timeRedisAttempt(ctx context.Context, rdb *redis.Client, key string) {
	rdb.Incr(ctx, key)
	rdb.Expire(ctx, key, time.Minute)
	rdb.Del(ctx, key)
}
