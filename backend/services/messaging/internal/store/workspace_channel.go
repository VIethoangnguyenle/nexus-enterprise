package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// ErrWorkspaceChannelExists: the workspace already has its workspace-wide
// channel. The unique index channels_one_workspace_wide raises it for a second
// one, whatever races got the two inserts there.
var ErrWorkspaceChannelExists = errors.New("the workspace already has a workspace-wide channel")

const workspaceChannelIndex = "channels_one_workspace_wide"

// asWorkspaceChannelExists maps the unique violation of a second workspace-wide
// channel to ErrWorkspaceChannelExists and passes any other error through.
func asWorkspaceChannelExists(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == workspaceChannelIndex {
		return fmt.Errorf("%w: %v", ErrWorkspaceChannelExists, err)
	}
	return err
}

const (
	wideIndexUnknown int32 = iota // not seen yet: ask the database
	wideIndexPresent              // seen; an index is never dropped but by a rollback
	wideIndexAbsent               // tests only: behave as before migration 060
)

// workspaceChannelsIndexed reports whether migration 060 has enforced one
// workspace-wide channel per workspace (a valid channels_one_workspace_wide). It is
// asked lazily and a positive answer is kept, so nothing needs the migration at
// start-up and the same binary works on data from either side of it.
func (s *Store) workspaceChannelsIndexed(ctx context.Context) (bool, error) {
	switch s.wideIndex.Load() {
	case wideIndexPresent:
		return true, nil
	case wideIndexAbsent:
		return false, nil
	}
	var ok bool
	if err := s.db.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM pg_index i JOIN pg_class c ON c.oid = i.indexrelid
		                WHERE c.relname = $1 AND i.indisvalid)`, workspaceChannelIndex).Scan(&ok); err != nil {
		return false, fmt.Errorf("workspace channel index: %w", err)
	}
	if ok {
		s.wideIndex.Store(wideIndexPresent)
	}
	return ok, nil
}

// WorkspaceWideChannel returns the workspace's one channel that every member is
// in (#general), or nil when it has none.
//
// After migration 060 a workspace has at most one channel of type 'workspace', so
// the type alone names it, whatever it has been renamed to. Before it, every group
// a user made was typed 'workspace', so the type names nothing; the oldest one
// called "general" that the workspace's owner created does, which is the rule the
// migration applies on its first run. Which of the two holds is read from the
// database, so one build serves data from before and after the migration.
func (s *Store) WorkspaceWideChannel(ctx context.Context, workspaceID string) (*Channel, error) {
	indexed, err := s.workspaceChannelsIndexed(ctx)
	if err != nil {
		return nil, err
	}
	q := `SELECT ` + channelCols + `
		 FROM channels c JOIN workspaces w ON w.id = c.workspace_id
		 WHERE c.workspace_id = $1 AND c.channel_type = 'workspace'`
	if !indexed {
		q += ` AND c.name = 'general' AND c.created_by = w.owner_id`
	}
	q += ` ORDER BY c.created_at, c.id LIMIT 1`
	ch, err := scanChannel(s.db.QueryRow(ctx, q, workspaceID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("workspace-wide channel: %w", err)
	}
	return ch, nil
}

// HasWorkspaceTypeChannel reports whether any channel of the workspace is typed
// 'workspace'. A workspace may have only one such channel.
func (s *Store) HasWorkspaceTypeChannel(ctx context.Context, workspaceID string) (bool, error) {
	var exists bool
	if err := s.db.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM channels WHERE workspace_id = $1 AND channel_type = 'workspace')`,
		workspaceID).Scan(&exists); err != nil {
		return false, fmt.Errorf("workspace channel lookup: %w", err)
	}
	return exists, nil
}

// DeleteChannelMembersInWorkspace drops the member-cache rows of one person for
// every channel of the workspace, and returns how many there were. A person who
// leaves a workspace is stripped from every channel's Members UA in the graph;
// this keeps the cache (member_count, push recipients) in step. DMs belong to no
// workspace and are untouched.
func (s *Store) DeleteChannelMembersInWorkspace(ctx context.Context, workspaceID, ngacNodeID string) (int64, error) {
	tag, err := s.db.Exec(ctx,
		`DELETE FROM channel_members
		 WHERE ngac_node_id = $2
		   AND channel_id IN (SELECT id FROM channels WHERE workspace_id = $1)`,
		workspaceID, ngacNodeID)
	if err != nil {
		return 0, fmt.Errorf("forget workspace member: %w", err)
	}
	return tag.RowsAffected(), nil
}
