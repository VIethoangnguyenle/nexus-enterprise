package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// lockedAsset is an asset row read FOR UPDATE inside a transaction.
type lockedAsset struct {
	typeID, workspaceID, state string
	deleted                    bool
	holder                     *string
}

func lockAsset(ctx context.Context, tx pgx.Tx, assetID string) (*lockedAsset, error) {
	a := &lockedAsset{}
	err := tx.QueryRow(ctx,
		`SELECT type_id, workspace_id, state, deleted, assigned_to FROM assets WHERE id = $1 FOR UPDATE`, assetID,
	).Scan(&a.typeID, &a.workspaceID, &a.state, &a.deleted, &a.holder)
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && a.deleted) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("locking asset: %w", err)
	}
	return a, nil
}

// isActiveMember reports whether the user is an active member of the workspace.
func isActiveMember(ctx context.Context, tx pgx.Tx, workspaceID, userID string) (bool, error) {
	var ok bool
	err := tx.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM tenant_users WHERE tenant_id = $1 AND user_id = $2 AND status = 'active')`,
		workspaceID, userID).Scan(&ok)
	if err != nil {
		return false, fmt.Errorf("checking membership: %w", err)
	}
	return ok, nil
}

// HandOver gives an available asset, or moves an assigned one, to a member of
// its workspace, and records the step. The checks and the writes share one
// transaction on the locked asset row, so two hand-overs of the same asset
// cannot both see it free.
func (s *Store) HandOver(ctx context.Context, assetID, assigneeID, actorID, comment string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin hand-over: %w", err)
	}
	defer tx.Rollback(ctx)

	a, err := lockAsset(ctx, tx, assetID)
	if err != nil {
		return err
	}
	if a.state != "available" && a.state != "assigned" {
		return ErrAssetUnavailable
	}
	ok, err := isActiveMember(ctx, tx, a.workspaceID, assigneeID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrNotAMember
	}
	if a.holder != nil && *a.holder == assigneeID {
		return ErrSameHolder
	}
	if _, err := tx.Exec(ctx,
		`UPDATE assets SET state = 'assigned', assigned_to = $1, updated_at = NOW() WHERE id = $2`,
		assigneeID, assetID); err != nil {
		return fmt.Errorf("assigning asset: %w", err)
	}
	if err := insertTransition(ctx, tx, &TransitionRecord{
		AssetID: assetID, FromState: a.state, ToState: "assigned", Action: "assign",
		ActorID: actorID, SubjectUserID: assigneeID, Comment: comment,
	}); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit hand-over: %w", err)
	}
	return nil
}

// FulfilParams names a request, the asset that answers it, and who is acting.
type FulfilParams struct {
	RequestID string
	AssetID   string
	ActorID   string
	// Comment is the approver's note (ApproveAndAssign) or the hand-over note.
	Comment string
}

// ApproveAndAssign approves a pending request and gives it the named asset as
// one unit: the asset becomes the requester's, the step is recorded in the
// asset's history, and the request is marked fulfilled with the actor as its
// approver. Any refusal, or any failed write, leaves request and asset exactly
// as they were.
func (s *Store) ApproveAndAssign(ctx context.Context, p FulfilParams) error {
	return s.fulfil(ctx, p, "pending", true)
}

// AssignApproved gives an asset to a request that was already approved.
func (s *Store) AssignApproved(ctx context.Context, p FulfilParams) error {
	return s.fulfil(ctx, p, "approved", false)
}

func (s *Store) fulfil(ctx context.Context, p FulfilParams, wantStatus string, recordApprover bool) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin fulfil: %w", err)
	}
	defer tx.Rollback(ctx)

	var status, reqType, reqWS, requester string
	err = tx.QueryRow(ctx,
		`SELECT status, type_id, workspace_id, requester_id FROM asset_requests WHERE id = $1 FOR UPDATE`, p.RequestID,
	).Scan(&status, &reqType, &reqWS, &requester)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("locking request: %w", err)
	}
	if status != wantStatus {
		return ErrRequestNotOpen
	}

	a, err := lockAsset(ctx, tx, p.AssetID)
	if err != nil {
		return err
	}
	if a.typeID != reqType || a.workspaceID != reqWS {
		return ErrWrongType
	}
	if a.state != "available" || a.holder != nil {
		return ErrAssetUnavailable
	}
	ok, err := isActiveMember(ctx, tx, reqWS, requester)
	if err != nil {
		return err
	}
	if !ok {
		return ErrNotAMember
	}

	if _, err := tx.Exec(ctx,
		`UPDATE assets SET state = 'assigned', assigned_to = $1, updated_at = NOW() WHERE id = $2`,
		requester, p.AssetID); err != nil {
		return fmt.Errorf("assigning asset: %w", err)
	}
	if err := insertTransition(ctx, tx, &TransitionRecord{
		AssetID: p.AssetID, FromState: a.state, ToState: "assigned", Action: "assign",
		ActorID: p.ActorID, SubjectUserID: requester, Comment: p.Comment,
	}); err != nil {
		return err
	}
	if recordApprover {
		_, err = tx.Exec(ctx,
			`UPDATE asset_requests SET status = 'fulfilled', assigned_asset_id = $1, approver_id = $2,
			        approver_comment = $3, updated_at = NOW() WHERE id = $4`,
			p.AssetID, p.ActorID, p.Comment, p.RequestID)
	} else {
		_, err = tx.Exec(ctx,
			`UPDATE asset_requests SET status = 'fulfilled', assigned_asset_id = $1, updated_at = NOW() WHERE id = $2`,
			p.AssetID, p.RequestID)
	}
	if err != nil {
		return fmt.Errorf("fulfilling request: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit fulfil: %w", err)
	}
	return nil
}

// AllActiveMembers reports whether every user ID is an active member of the
// workspace. An empty list is trivially true.
func (s *Store) AllActiveMembers(ctx context.Context, workspaceID string, userIDs []string) (bool, error) {
	if len(userIDs) == 0 {
		return true, nil
	}
	var n int
	err := s.pool.QueryRow(ctx,
		`SELECT COUNT(DISTINCT user_id) FROM tenant_users WHERE tenant_id = $1 AND status = 'active' AND user_id = ANY($2)`,
		workspaceID, userIDs).Scan(&n)
	if err != nil {
		return false, fmt.Errorf("checking members: %w", err)
	}
	distinct := map[string]bool{}
	for _, id := range userIDs {
		distinct[id] = true
	}
	return n == len(distinct), nil
}
