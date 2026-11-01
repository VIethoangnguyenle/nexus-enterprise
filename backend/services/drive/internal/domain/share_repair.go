package domain

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"ngac-platform/pkg/grpcauth"
	"ngac-platform/pkg/policyclient"
	policypb "ngac-platform/proto/policy"
)

// shareRepairBatch bounds one repair pass; the next pass continues.
const shareRepairBatch = 1000

// DefaultShareRepairRetries are the waits between share repair runs until one
// completes clean: about an hour and a half in all, enough for a policy
// service restarted in the same deploy to come back.
var DefaultShareRepairRetries = []time.Duration{
	10 * time.Second, 30 * time.Second, time.Minute, 2 * time.Minute, 5 * time.Minute,
	10 * time.Minute, 15 * time.Minute, 15 * time.Minute, 15 * time.Minute, 15 * time.Minute,
}

// ShareRepairResult counts what one repair run did.
type ShareRepairResult struct {
	FilesRepaired     int // files whose shares were narrowed to the file
	RootSharesRevoked int // shares of a drive root that were revoked
	Failed            int // files or root shares left as they were
}

// RepairShares brings shares made before the current rules up to them, through
// the policy writer so every change takes the EPP invalidation path:
//
//   - a share of a file whose share OA holds the file's folder (or the Documents
//     OA) is narrowed to the file (see RepairFileShares);
//   - a share of a drive root, which hands over the whole drive, is revoked.
//
// Nothing is ever widened: a share is narrowed or removed, or left as it was
// when a step fails. A run that finds nothing writes nothing, so running it
// again is safe. An empty workspaceID covers every workspace.
func (s *Service) RepairShares(ctx context.Context, workspaceID string) (ShareRepairResult, error) {
	var res ShareRepairResult
	var errs []error
	repaired, failed, err := s.repairFiles(ctx, workspaceID)
	res.FilesRepaired, res.Failed = repaired, failed
	if err != nil {
		errs = append(errs, err)
	}
	revoked, failed, err := s.revokeRootShares(ctx, workspaceID)
	res.RootSharesRevoked, res.Failed = revoked, res.Failed+failed
	if err != nil {
		errs = append(errs, err)
	}
	if res.Failed > 0 {
		errs = append(errs, fmt.Errorf("%d shares could not be repaired", res.Failed))
	}
	return res, errors.Join(errs...)
}

// RepairSharesUntilClean runs RepairShares, and again after each wait in
// retries for as long as a run does not complete clean (the policy service may
// be restarting in the same deploy). Each run logs one summary line,
// "drive share repair run", with its counts and whether it was clean. It
// returns nil once a run is clean, the last run's error when the retries are
// spent, and the context's error when it is cancelled (shutdown).
func (s *Service) RepairSharesUntilClean(ctx context.Context, workspaceID string, retries []time.Duration) error {
	for attempt := 0; ; attempt++ {
		res, err := s.RepairShares(ctx, workspaceID)
		clean := err == nil
		attrs := []any{
			"attempt", attempt + 1, "clean", clean, "files_repaired", res.FilesRepaired,
			"root_shares_revoked", res.RootSharesRevoked, "failed", res.Failed,
		}
		if clean {
			slog.Info("drive share repair run", attrs...)
			return nil
		}
		slog.Warn("drive share repair run", append(attrs, "error", err)...)
		if attempt >= len(retries) {
			slog.Error("drive share repair gave up; shares listed above are left as they were until the next start",
				"attempts", attempt+1)
			return err
		}
		t := time.NewTimer(retries[attempt])
		select {
		case <-ctx.Done():
			t.Stop()
			return ctx.Err()
		case <-t.C:
		}
	}
}

// RepairFileShares narrows the shares made before a shared file had an OA of
// its own. Such a share OA holds the file's folder OA (or the Documents OA for a
// top-level file) and so grants everything in it. Each such file gets its own
// OA and its shares are narrowed to it (ensureFileOA).
//
// It finds its work with one query on the drive's own tables and does nothing
// more when that finds nothing; it is safe to run again after a partial
// failure. It never widens a share (see narrowShare), and a folder share is
// never touched.
//
// The policy writer admits drive only on behalf of a user, so each file is
// repaired as the person who made its earliest share, or else its owner. A file
// with neither is logged and left for an operator. It returns how many files it
// repaired, and an error naming how many it could not.
func (s *Service) RepairFileShares(ctx context.Context, workspaceID string) (int, error) {
	repaired, failed, err := s.repairFiles(ctx, workspaceID)
	if err == nil && failed > 0 {
		err = fmt.Errorf("%d shared files could not be repaired", failed)
	}
	return repaired, err
}

func (s *Service) repairFiles(ctx context.Context, workspaceID string) (repaired, failed int, err error) {
	files, err := s.store.SharedFilesWithoutTheirOwnOA(ctx, workspaceID, shareRepairBatch)
	if err != nil {
		return 0, 0, err
	}
	for _, f := range files {
		if f.ActorUserID == "" || f.ActorNode == "" {
			slog.Error("shared file not repaired: neither its sharer nor its owner has an identity",
				"item", f.Item.ID)
			failed++
			continue
		}
		as := grpcauth.WithCaller(ctx, grpcauth.Caller{UserID: f.ActorUserID, NGACNodeID: f.ActorNode})
		if _, err := s.ensureFileOA(as, f.Item); err != nil {
			slog.Error("shared file not repaired", "item", f.Item.ID, "err", err)
			failed++
			continue
		}
		slog.Info("narrowed the shares of a file shared before it had its own OA", "item", f.Item.ID)
		repaired++
	}
	if len(files) == shareRepairBatch {
		slog.Warn("more shared files may need repair; the next run continues", "batch", shareRepairBatch)
	}
	return repaired, failed, nil
}

// revokeRootShares revokes every share of a drive root, as the person who made
// it: its share OA is deleted through the policy writer (which removes the
// association and every edge on it), then the row. A share OA that is already
// gone counts as revoked. A share whose maker has no identity, or whose
// revocation is refused, is logged and left for the next run.
func (s *Service) revokeRootShares(ctx context.Context, workspaceID string) (revoked, failed int, err error) {
	shares, err := s.store.SharesOfDriveRoots(ctx, workspaceID, shareRepairBatch)
	if err != nil {
		return 0, 0, err
	}
	for _, sh := range shares {
		if sh.ActorUserID == "" || sh.ActorNode == "" {
			slog.Error("share of a drive root not revoked: its maker has no identity",
				"share_id", sh.ShareID, "item", sh.ItemID, "share_oa", sh.ShareOA)
			failed++
			continue
		}
		as := grpcauth.WithCaller(ctx, grpcauth.Caller{UserID: sh.ActorUserID, NGACNodeID: sh.ActorNode})
		if _, err := s.policyWrite.DeleteNode(as, &policypb.DeleteNodeRequest{NodeId: sh.ShareOA}); err != nil && !policyclient.IsNotFound(err) {
			slog.Error("share of a drive root not revoked", "share_id", sh.ShareID, "item", sh.ItemID, "err", err)
			failed++
			continue
		}
		if err := s.store.DeleteShare(ctx, sh.ShareID); err != nil {
			slog.Error("share of a drive root revoked but its row not removed", "share_id", sh.ShareID, "err", err)
			failed++
			continue
		}
		slog.Warn("revoked a share of a drive root", "share_id", sh.ShareID, "item", sh.ItemID, "share_oa", sh.ShareOA)
		revoked++
	}
	return revoked, failed, nil
}
