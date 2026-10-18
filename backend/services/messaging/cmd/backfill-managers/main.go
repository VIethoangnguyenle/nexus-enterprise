// Command backfill-managers gives every existing group an owner and a Managers
// UA: the creator stays owner while still assigned to the group's Members UA,
// otherwise the earliest current member takes over; the owner is put in the
// Managers UA, created beside Members when missing. Graph writes go through the
// policy writer. Failed groups are listed and make the command exit non-zero;
// rerunning is safe.
//
// Usage:
//
//	go run ./cmd/backfill-managers/ -dry-run         # print what would change
//	go run ./cmd/backfill-managers/                   # apply changes
//
// Environment:
//
//	DATABASE_URL   — Postgres DSN (default: dev)
//	POLICY_SERVICE_ADDR — policy gRPC address (default: localhost:50055)
//	INTERNAL_IDENTITY_SECRET — for gRPC auth
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"ngac-platform/pkg/bootstrap"
	"ngac-platform/pkg/grpcauth"
	policypb "ngac-platform/proto/policy"
)

func main() {
	bootstrap.InitLogger()
	if err := bootstrap.ConfigureInternalIdentity(); err != nil {
		slog.Error("refusing to start", "error", err)
		os.Exit(1)
	}

	dryRun := flag.Bool("dry-run", false, "print what would change without applying")
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	dbURL := bootstrap.Env("DATABASE_URL", "postgres://ngac:ngac_secret@localhost:5433/ngac?sslmode=disable")
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		slog.Error("database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	policyAddr := bootstrap.PolicyAddr()
	policyConn, err := grpcauth.Dial(policyAddr, "backfill-managers")
	if err != nil {
		slog.Error("policy service", "error", err)
		os.Exit(1)
	}
	defer policyConn.Close()
	b := &backfiller{db: pool, write: policypb.NewPolicyWriteServiceClient(policyConn), dryRun: *dryRun, out: os.Stdout}
	s, err := b.run(ctx)
	if err != nil {
		slog.Error("backfill", "error", err)
		os.Exit(1)
	}

	mode := "DRY RUN"
	if !*dryRun {
		mode = "APPLIED"
	}
	fmt.Printf("\n%s summary: %d groups, owners kept=%d moved=%d, managers UAs created=%d, failed=%d\n",
		mode, s.Groups, s.OwnersKept, s.OwnersMoved, s.ManagersUAsMade, s.Failed)
	if s.Failed > 0 {
		os.Exit(1)
	}
}
