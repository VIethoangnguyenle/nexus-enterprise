package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"

	"google.golang.org/grpc"

	"ngac-platform/pkg/bootstrap"
	"ngac-platform/pkg/grpcauth"
	pb "ngac-platform/proto/policy"
)

// invalidateService is the identity the operator tool presents. The policy
// service admits it on InvalidateCache and nothing else (see grpc.AuthPolicy).
const invalidateService = "policy-ctl"

// invalidateCallTimeout bounds one InvalidateCache call.
const invalidateCallTimeout = 30 * time.Second

type invalidateOptions struct {
	batch int
}

// parseInvalidateArgs reads `policy invalidate-cache` arguments. The only scope
// is every workspace; asking for nothing is an error rather than a silent no-op.
func parseInvalidateArgs(args []string) (invalidateOptions, error) {
	fs := flag.NewFlagSet("invalidate-cache", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	all := fs.Bool("all-workspaces", false, "drop cached decisions of every workspace")
	batch := fs.Int("batch", 50, "workspaces per InvalidateCache call")
	if err := fs.Parse(args); err != nil {
		return invalidateOptions{}, fmt.Errorf("usage: policy invalidate-cache --all-workspaces [--batch N]: %w", err)
	}
	if !*all {
		return invalidateOptions{}, errors.New("usage: policy invalidate-cache --all-workspaces [--batch N]")
	}
	if *batch < 1 {
		return invalidateOptions{}, errors.New("--batch must be at least 1")
	}
	return invalidateOptions{batch: *batch}, nil
}

// cacheInvalidator is the part of the policy write client the tool uses.
type cacheInvalidator interface {
	InvalidateCache(ctx context.Context, in *pb.InvalidateCacheRequest, opts ...grpc.CallOption) (*pb.InvalidateCacheResponse, error)
}

// invalidateInBatches asks the writer to drop the cached decisions around each
// node, batch ids at a time. It stops at the first failure: a batch that did not
// go through means decisions made before the migration may still be served.
func invalidateInBatches(ctx context.Context, c cacheInvalidator, nodeIDs []string, batch int, reason string) error {
	for start := 0; start < len(nodeIDs); start += batch {
		end := min(start+batch, len(nodeIDs))
		// WaitForReady rides out a writer that is still starting; the deadline stops
		// a writer that never comes from hanging the deploy.
		callCtx, cancel := context.WithTimeout(ctx, invalidateCallTimeout)
		_, err := c.InvalidateCache(callCtx, &pb.InvalidateCacheRequest{
			NodeIds: nodeIDs[start:end], Reason: reason,
		}, grpc.WaitForReady(true))
		cancel()
		if err != nil {
			return fmt.Errorf("invalidate nodes %d-%d of %d: %w", start+1, end, len(nodeIDs), err)
		}
	}
	return nil
}

// runInvalidateCache is `policy invalidate-cache --all-workspaces`. Migrations
// that rewrite ngac_* tables bypass EPP, so the running writer (and, through
// Redis, the read replica) still hold the old decisions until told otherwise.
// It reads every workspace's policy-class node id from the database and has the
// running writer at POLICY_SERVICE_ADDR invalidate them. It returns the process
// exit code.
func runInvalidateCache(args []string) int {
	opts, err := parseInvalidateArgs(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	if err := bootstrap.ConfigureInternalIdentity(); err != nil {
		slog.Error("refusing to run", "error", err)
		return 1
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	pool, err := bootstrap.ConnectDB(ctx, bootstrap.Env("DATABASE_URL", "postgres://ngac:ngac_secret@localhost:5433/ngac?sslmode=disable"))
	if err != nil {
		slog.Error("connect database", "error", err)
		return 1
	}
	defer pool.Close()

	rows, err := pool.Query(ctx, `SELECT ngac_pc_id FROM workspaces WHERE ngac_pc_id IS NOT NULL ORDER BY id`)
	if err != nil {
		slog.Error("list workspaces", "error", err)
		return 1
	}
	var pcIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			slog.Error("scan workspace", "error", err)
			return 1
		}
		pcIDs = append(pcIDs, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		slog.Error("list workspaces", "error", err)
		return 1
	}

	addr := bootstrap.PolicyAddr()
	conn, err := grpcauth.Dial(addr, invalidateService)
	if err != nil {
		slog.Error("dial policy", "addr", addr, "error", err)
		return 1
	}
	defer conn.Close()

	if err := invalidateInBatches(ctx, pb.NewPolicyWriteServiceClient(conn), pcIDs, opts.batch, "policy invalidate-cache --all-workspaces"); err != nil {
		slog.Error("invalidate cache", "addr", addr, "error", err)
		return 1
	}
	slog.Info("invalidated cached decisions", "workspaces", len(pcIDs), "addr", addr)
	return 0
}
