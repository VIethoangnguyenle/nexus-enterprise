package store

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Store handles database operations for drive_items, drive_shares, and drive_quotas.
type Store struct {
	db *pgxpool.Pool
}

// NewStore creates a new Store backed by the given connection pool.
func NewStore(db *pgxpool.Pool) *Store {
	return &Store{db: db}
}

// DriveItem represents a row in the drive_items table.
type DriveItem struct {
	ID             string
	WorkspaceID    string
	DriveContext   string
	DriveContextID string
	ParentID       *string
	ItemType       string
	Name           string
	MimeType       *string
	SizeBytes      *int64
	ObjectKey      *string
	StorageDocID   *string
	NGACNodeID     string
	ScopeOAID      string
	OwnerID        string
	Status         string
	TrashedAt      *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
	// IsRoot marks the drive root of a context. Top-level user folders are
	// parent-less too, so "no parent" does not identify the root; at most one
	// active root exists per (workspace, context, context id).
	IsRoot bool
}

// ErrRootExists is returned by InsertItem when an active root already exists
// for the item's drive context: another run created it first.
var ErrRootExists = errors.New("drive root already exists for this context")

// InsertItem creates a new drive item.
func (s *Store) InsertItem(ctx context.Context, item *DriveItem) error {
	if item.ID == "" {
		item.ID = uuid.New().String()
	}
	now := time.Now()
	if item.CreatedAt.IsZero() {
		item.CreatedAt = now
	}
	if item.UpdatedAt.IsZero() {
		item.UpdatedAt = now
	}
	_, err := s.db.Exec(ctx,
		`INSERT INTO drive_items (id, workspace_id, drive_context, drive_context_id, parent_id,
			item_type, name, mime_type, size_bytes, object_key, storage_doc_id, ngac_node_id,
			scope_oa_id, owner_id, status, is_root)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)`,
		item.ID, item.WorkspaceID, item.DriveContext, NilIfEmpty(item.DriveContextID),
		item.ParentID, item.ItemType, item.Name, item.MimeType, item.SizeBytes,
		item.ObjectKey, item.StorageDocID, item.NGACNodeID, NilIfEmpty(item.ScopeOAID),
		item.OwnerID, item.Status, item.IsRoot)
	var pgErr *pgconn.PgError
	if item.IsRoot && errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "drive_items_one_root_per_context" {
		return ErrRootExists
	}
	return err
}

// GetItem retrieves a drive item by ID.
func (s *Store) GetItem(ctx context.Context, id string) (*DriveItem, error) {
	item := &DriveItem{}
	err := s.db.QueryRow(ctx,
		`SELECT id, workspace_id, drive_context, COALESCE(drive_context_id,''), parent_id,
			item_type, name, mime_type, size_bytes, object_key, storage_doc_id, ngac_node_id,
			COALESCE(scope_oa_id,''), owner_id, status, trashed_at, created_at, updated_at
		 FROM drive_items WHERE id = $1`, id).
		Scan(&item.ID, &item.WorkspaceID, &item.DriveContext, &item.DriveContextID,
			&item.ParentID, &item.ItemType, &item.Name, &item.MimeType, &item.SizeBytes,
			&item.ObjectKey, &item.StorageDocID, &item.NGACNodeID, &item.ScopeOAID,
			&item.OwnerID, &item.Status, &item.TrashedAt, &item.CreatedAt, &item.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return item, err
}

// ListChildren returns active drive items in a folder.
func (s *Store) ListChildren(ctx context.Context, parentID *string, workspaceID, driveContext, driveContextID string) ([]*DriveItem, error) {
	var rows pgx.Rows
	var err error

	if parentID == nil {
		rows, err = s.db.Query(ctx,
			`SELECT id, workspace_id, drive_context, COALESCE(drive_context_id,''), parent_id,
				item_type, name, mime_type, size_bytes, object_key, storage_doc_id, ngac_node_id,
				COALESCE(scope_oa_id,''), owner_id, status, trashed_at, created_at, updated_at
			 FROM drive_items
			 WHERE parent_id IS NULL AND workspace_id = $1
			   AND drive_context = $2 AND COALESCE(drive_context_id,'') = $3
			   AND status = 'active'
			 ORDER BY item_type DESC, name ASC`, workspaceID, driveContext, driveContextID)
	} else {
		rows, err = s.db.Query(ctx,
			`SELECT id, workspace_id, drive_context, COALESCE(drive_context_id,''), parent_id,
				item_type, name, mime_type, size_bytes, object_key, storage_doc_id, ngac_node_id,
				COALESCE(scope_oa_id,''), owner_id, status, trashed_at, created_at, updated_at
			 FROM drive_items
			 WHERE parent_id = $1 AND status = 'active'
			 ORDER BY item_type DESC, name ASC`, *parentID)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []*DriveItem
	for rows.Next() {
		item := &DriveItem{}
		if err := rows.Scan(&item.ID, &item.WorkspaceID, &item.DriveContext, &item.DriveContextID,
			&item.ParentID, &item.ItemType, &item.Name, &item.MimeType, &item.SizeBytes,
			&item.ObjectKey, &item.StorageDocID, &item.NGACNodeID, &item.ScopeOAID,
			&item.OwnerID, &item.Status, &item.TrashedAt, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, nil
}

// ListByScopes returns all active items whose scope_oa_id matches any of the
// given scope IDs. Used for NGAC scope-based listing where the caller has
// pre-resolved their accessible scopes via ResolveAccessibleScopes.
// This replaces per-item CheckAccess loops with a single indexed SQL query.
func (s *Store) ListByScopes(ctx context.Context, scopeOAIDs []string, cursor string, limit int) ([]*DriveItem, string, error) {
	if len(scopeOAIDs) == 0 {
		return nil, "", nil
	}
	if limit <= 0 {
		limit = 50
	}

	var rows pgx.Rows
	var err error
	if cursor == "" {
		rows, err = s.db.Query(ctx,
			`SELECT id, workspace_id, drive_context, COALESCE(drive_context_id,''), parent_id,
				item_type, name, mime_type, size_bytes, object_key, storage_doc_id, ngac_node_id,
				COALESCE(scope_oa_id,''), owner_id, status, trashed_at, created_at, updated_at
			 FROM drive_items
			 WHERE scope_oa_id = ANY($1) AND status = 'active'
			 ORDER BY created_at DESC
			 LIMIT $2`, scopeOAIDs, limit+1)
	} else {
		rows, err = s.db.Query(ctx,
			`SELECT id, workspace_id, drive_context, COALESCE(drive_context_id,''), parent_id,
				item_type, name, mime_type, size_bytes, object_key, storage_doc_id, ngac_node_id,
				COALESCE(scope_oa_id,''), owner_id, status, trashed_at, created_at, updated_at
			 FROM drive_items
			 WHERE scope_oa_id = ANY($1) AND status = 'active'
			   AND created_at < $2
			 ORDER BY created_at DESC
			 LIMIT $3`, scopeOAIDs, cursor, limit+1)
	}
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	var items []*DriveItem
	for rows.Next() {
		item := &DriveItem{}
		if err := rows.Scan(&item.ID, &item.WorkspaceID, &item.DriveContext, &item.DriveContextID,
			&item.ParentID, &item.ItemType, &item.Name, &item.MimeType, &item.SizeBytes,
			&item.ObjectKey, &item.StorageDocID, &item.NGACNodeID, &item.ScopeOAID,
			&item.OwnerID, &item.Status, &item.TrashedAt, &item.CreatedAt, &item.UpdatedAt); err != nil {
			return nil, "", err
		}
		items = append(items, item)
	}

	var nextCursor string
	if len(items) > limit {
		nextCursor = items[limit-1].CreatedAt.Format("2006-01-02T15:04:05.999999Z")
		items = items[:limit]
	}
	return items, nextCursor, nil
}

// UpdateParent changes the parent of a drive item (move operation).
func (s *Store) UpdateParent(ctx context.Context, id string, newParentID *string) error {
	_, err := s.db.Exec(ctx,
		`UPDATE drive_items SET parent_id = $1, updated_at = NOW() WHERE id = $2`,
		newParentID, id)
	return err
}

// UpdateParentAndNode moves an item in one statement: its parent row and the
// OA it is authorized against change together or not at all. The update applies
// only if the item is still under expectedParent and not trashed; it reports
// whether it did, so a caller that lost a race to another move can undo its
// policy edges instead of recording a parent that disagrees with the graph.
func (s *Store) UpdateParentAndNode(ctx context.Context, id string, expectedParent, newParentID *string, ngacNodeID string) (bool, error) {
	tag, err := s.db.Exec(ctx,
		`UPDATE drive_items SET parent_id = $1, ngac_node_id = $2, updated_at = NOW()
		 WHERE id = $3 AND parent_id IS NOT DISTINCT FROM $4 AND status <> 'trashed'`,
		newParentID, ngacNodeID, id, expectedParent)
	if err != nil {
		return false, err
	}
	return tag.RowsAffected() == 1, nil
}

// LockItem serialises work on one item across every drive instance with a
// session-level Postgres advisory lock, and returns the function that releases
// it. Moves hold it from reading the item's parent to writing the new one, so
// two moves of the same item cannot interleave their policy edges. The lock
// lives on its own pooled connection; if that connection cannot be unlocked it
// is closed, which drops the lock.
func (s *Store) LockItem(ctx context.Context, id string) (func(), error) {
	conn, err := s.db.Acquire(ctx)
	if err != nil {
		return nil, err
	}
	key := "drive_item:" + id
	if _, err := conn.Exec(ctx, `SELECT pg_advisory_lock(hashtextextended($1, 0))`, key); err != nil {
		_ = conn.Hijack().Close(context.Background())
		return nil, err
	}
	return func() {
		uctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, err := conn.Exec(uctx, `SELECT pg_advisory_unlock(hashtextextended($1, 0))`, key); err != nil {
			_ = conn.Hijack().Close(uctx)
			return
		}
		conn.Release()
	}, nil
}

// MaxTreeDepth bounds every walk up or down the folder tree. It is far deeper
// than any real hierarchy and exists so that a parent_id cycle ends a query
// instead of running it until the statement times out.
const MaxTreeDepth = 64

// IsAncestorOrSelf reports whether ancestorID is id itself or one of the
// folders above it, following parent_id. Moving a folder under anything for
// which this is true would make it its own ancestor.
func (s *Store) IsAncestorOrSelf(ctx context.Context, ancestorID, id string) (bool, error) {
	var found bool
	err := s.db.QueryRow(ctx,
		`WITH RECURSIVE up AS (
			SELECT id, parent_id, 1 AS depth FROM drive_items WHERE id = $1
			UNION ALL
			SELECT di.id, di.parent_id, up.depth + 1 FROM drive_items di JOIN up ON di.id = up.parent_id
			WHERE up.depth < $3
		)
		SELECT EXISTS (SELECT 1 FROM up WHERE id = $2)`, id, ancestorID, MaxTreeDepth).Scan(&found)
	return found, err
}

// UpdateName renames a drive item.
func (s *Store) UpdateName(ctx context.Context, id, newName string) error {
	_, err := s.db.Exec(ctx,
		`UPDATE drive_items SET name = $1, updated_at = NOW() WHERE id = $2`,
		newName, id)
	return err
}

// UpdateNGACNodeID updates the NGAC node ID for a drive item.
// Used when moving files to a new parent folder — files inherit the parent's OA.
func (s *Store) UpdateNGACNodeID(ctx context.Context, id, ngacNodeID string) error {
	_, err := s.db.Exec(ctx,
		`UPDATE drive_items SET ngac_node_id = $1, updated_at = NOW() WHERE id = $2`,
		ngacNodeID, id)
	return err
}

// UpdateStatus sets the status of a drive item.
func (s *Store) UpdateStatus(ctx context.Context, id, status string) error {
	if status == "trashed" {
		_, err := s.db.Exec(ctx,
			`UPDATE drive_items SET status = $1, trashed_at = NOW(), updated_at = NOW() WHERE id = $2`,
			status, id)
		return err
	}
	_, err := s.db.Exec(ctx,
		`UPDATE drive_items SET status = $1, trashed_at = NULL, updated_at = NOW() WHERE id = $2`,
		status, id)
	return err
}

// GetBreadcrumb returns the path from a folder to the root.
func (s *Store) GetBreadcrumb(ctx context.Context, folderID string) ([]struct{ ID, Name string }, error) {
	// Depth counts up from the folder asked about, so the deepest row is the
	// top of the tree. Ordering by id would give an arbitrary path.
	//
	// The walk is bounded: a parent_id cycle (which no code path should
	// produce, but a bad write or manual edit can) would otherwise recurse
	// until the statement times out, on every list request for that folder.
	rows, err := s.db.Query(ctx,
		`WITH RECURSIVE path AS (
			SELECT id, name, parent_id, 0 AS depth FROM drive_items WHERE id = $1
			UNION ALL
			SELECT di.id, di.name, di.parent_id, p.depth + 1 FROM drive_items di JOIN path p ON di.id = p.parent_id
			WHERE p.depth < $2
		)
		SELECT id, name FROM path ORDER BY depth DESC`, folderID, MaxTreeDepth)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var crumbs []struct{ ID, Name string }
	for rows.Next() {
		var c struct{ ID, Name string }
		if err := rows.Scan(&c.ID, &c.Name); err != nil {
			return nil, err
		}
		crumbs = append(crumbs, c)
	}
	return crumbs, rows.Err()
}

// --- Shares ---

// --- Quotas ---

// NilIfEmpty is the nullable column value for s: nil for "", else &s.
func NilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// SetTrashed trashes (or restores) an item and, for a folder, everything
// beneath it as one unit: either the whole subtree changes or none of it does,
// so a failure cannot leave a trashed folder over live children.
func (s *Store) SetTrashed(ctx context.Context, id string, isFolder, trashed bool) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // a no-op after Commit

	status, parentSQL, childSQL := "active", `trashed_at = NULL`, `trashed_at = NULL`
	childWhere := ` AND status = 'trashed'`
	if trashed {
		status, parentSQL, childSQL, childWhere = "trashed", `trashed_at = NOW()`, `trashed_at = NOW()`, ""
	}
	if _, err := tx.Exec(ctx,
		`UPDATE drive_items SET status = $1, `+parentSQL+`, updated_at = NOW() WHERE id = $2`, status, id); err != nil {
		return err
	}
	if isFolder {
		if _, err := tx.Exec(ctx,
			`WITH RECURSIVE tree AS (
				SELECT id FROM drive_items WHERE parent_id = $2
				UNION ALL
				SELECT di.id FROM drive_items di JOIN tree t ON di.parent_id = t.id
			)
			UPDATE drive_items SET status = $1, `+childSQL+`, updated_at = NOW()
			WHERE id IN (SELECT id FROM tree)`+childWhere, status, id); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
