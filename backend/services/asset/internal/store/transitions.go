package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// TransitionRecord records a lifecycle state change.
type TransitionRecord struct {
	ID        string
	AssetID   string
	FromState string
	ToState   string
	Action    string
	ActorID   string
	ActorName string
	// SubjectUserID is the person the step concerned: the new holder on a
	// hand-over, the previous holder on a return. Empty for steps about no one.
	SubjectUserID string
	SubjectName   string
	Comment       string
	CreatedAt     time.Time
	// ExpectHolder, when set, is who the caller read as the holder; the step is
	// refused if somebody else holds the asset by the time it is locked.
	ExpectHolder string
}

// UpdateAssetState changes the state of an asset and optionally the assigned_to field.
func (s *Store) UpdateAssetState(ctx context.Context, assetID, newState string, assignedTo *string) error {
	var err error
	if assignedTo != nil {
		_, err = s.pool.Exec(ctx,
			`UPDATE assets SET state = $1, assigned_to = $2, updated_at = NOW() WHERE id = $3`,
			newState, *assignedTo, assetID,
		)
	} else {
		_, err = s.pool.Exec(ctx,
			`UPDATE assets SET state = $1, updated_at = NOW() WHERE id = $2`,
			newState, assetID,
		)
	}
	if err != nil {
		return fmt.Errorf("updating asset state: %w", err)
	}
	return nil
}

// ClearAssignment removes the assigned_to from an asset.
func (s *Store) ClearAssignment(ctx context.Context, assetID string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE assets SET assigned_to = NULL, updated_at = NOW() WHERE id = $1`, assetID,
	)
	if err != nil {
		return fmt.Errorf("clearing asset assignment: %w", err)
	}
	return nil
}

// clearsHolder reports whether an asset that moves into this state is no
// longer held by anyone. Maintenance and the like leave the holder in place —
// it is still their laptop — while stock, retired and disposed assets have none.
func clearsHolder(toState string) bool {
	switch toState {
	case "available", "requested", "retired", "disposed":
		return true
	}
	return false
}

// ApplyTransition changes an asset's state and records the transition in its
// history, in one database transaction. If either write fails neither takes
// effect: a state change with no history row would be an unaudited change, and
// the history is how the UI says who did what and when.
//
// assignedTo, when non-nil, also sets the asset's holder (and is the step's
// subject). A move into a state that holds no one clears the holder, and the
// step then names the person it was taken from.
func (s *Store) ApplyTransition(ctx context.Context, tr *TransitionRecord, assignedTo *string) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transition: %w", err)
	}
	defer tx.Rollback(ctx) // no-op after Commit

	var holder *string
	var state string
	var deleted bool
	if err := tx.QueryRow(ctx, `SELECT state, deleted, assigned_to FROM assets WHERE id = $1 FOR UPDATE`, tr.AssetID).Scan(&state, &deleted, &holder); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("locking asset: %w", err)
	}
	// The step was chosen from what the caller read; under the lock, it must
	// still be true.
	if deleted {
		return ErrNotFound
	}
	if state != tr.FromState {
		return ErrStateChanged
	}
	if tr.ExpectHolder != "" && (holder == nil || *holder != tr.ExpectHolder) {
		return ErrStateChanged
	}
	subject := tr.SubjectUserID
	switch {
	case assignedTo != nil:
		holder = assignedTo
		if subject == "" {
			subject = *assignedTo
		}
	case clearsHolder(tr.ToState):
		if holder != nil && subject == "" {
			subject = *holder
		}
		holder = nil
	}
	tr.SubjectUserID = subject

	if _, err := tx.Exec(ctx,
		`UPDATE assets SET state = $1, assigned_to = $2, updated_at = NOW() WHERE id = $3`,
		tr.ToState, holder, tr.AssetID); err != nil {
		return fmt.Errorf("updating asset state: %w", err)
	}
	if err := insertTransition(ctx, tx, tr); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transition: %w", err)
	}
	return nil
}

type execer interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

func insertTransition(ctx context.Context, q execer, tr *TransitionRecord) error {
	tr.ID = uuid.New().String()
	tr.CreatedAt = time.Now()
	if _, err := q.Exec(ctx,
		`INSERT INTO asset_transitions (id, asset_id, from_state, to_state, action, actor_id, subject_user_id, comment, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, NULLIF($7, ''), $8, $9)`,
		tr.ID, tr.AssetID, tr.FromState, tr.ToState, tr.Action, tr.ActorID, tr.SubjectUserID, tr.Comment, tr.CreatedAt,
	); err != nil {
		return fmt.Errorf("inserting transition: %w", err)
	}
	return nil
}

// InsertTransition records a lifecycle step without changing the asset.
func (s *Store) InsertTransition(ctx context.Context, tr *TransitionRecord) error {
	return insertTransition(ctx, s.pool, tr)
}

// GetAssetHistory returns all transitions for an asset ordered chronologically.
// The people in it are named only if they belong to the asset's workspace.
func (s *Store) GetAssetHistory(ctx context.Context, assetID string) ([]*TransitionRecord, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT t.id, t.asset_id, t.from_state, t.to_state, t.action,
		        t.actor_id, `+personName("au", "atu")+`, COALESCE(t.subject_user_id, ''), `+personName("su", "stu")+`,
		        t.comment, t.created_at
		 FROM asset_transitions t
		 JOIN assets a ON a.id = t.asset_id`+
			memberJoin("t.actor_id", "a.workspace_id", "au", "atu")+
			memberJoin("t.subject_user_id", "a.workspace_id", "su", "stu")+`
		 WHERE t.asset_id = $1 ORDER BY t.created_at, t.id`, assetID,
	)
	if err != nil {
		return nil, fmt.Errorf("getting asset history: %w", err)
	}
	defer rows.Close()

	var records []*TransitionRecord
	for rows.Next() {
		tr := &TransitionRecord{}
		if err := rows.Scan(
			&tr.ID, &tr.AssetID, &tr.FromState, &tr.ToState, &tr.Action,
			&tr.ActorID, &tr.ActorName, &tr.SubjectUserID, &tr.SubjectName, &tr.Comment, &tr.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scanning transition row: %w", err)
		}
		records = append(records, tr)
	}
	return records, rows.Err()
}
