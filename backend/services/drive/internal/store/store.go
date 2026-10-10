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

// DriveShare represents a row in the drive_shares table.
type DriveShare struct {
	ID           string
	DriveItemID  string
	ShareType    string
	TargetNGACID *string
	TargetLabel  *string
	Operations   []string
	NGACShareOA  string
	CreatedBy    string
	CreatedAt    time.Time
}

// DriveQuota represents a row in the drive_quotas table.
type DriveQuota struct {
	WorkspaceID string
	MaxBytes    int64
	UsedBytes   int64
	MaxFiles    int32
	UsedFiles   int32
	UpdatedAt   time.Time
}

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
		item.ID, item.WorkspaceID, item.DriveContext, nilStr(item.DriveContextID),
		item.ParentID, item.ItemType, item.Name, item.MimeType, item.SizeBytes,
		item.ObjectKey, item.StorageDocID, item.NGACNodeID, nilStr(item.ScopeOAID),
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

// TrashChildren recursively trashes all children of a folder.
func (s *Store) TrashChildren(ctx context.Context, parentID string) error {
	_, err := s.db.Exec(ctx,
		`WITH RECURSIVE tree AS (
			SELECT id FROM drive_items WHERE parent_id = $1
			UNION ALL
			SELECT di.id FROM drive_items di JOIN tree t ON di.parent_id = t.id
		)
		UPDATE drive_items SET status = 'trashed', trashed_at = NOW(), updated_at = NOW()
		WHERE id IN (SELECT id FROM tree)`, parentID)
	return err
}

// RestoreChildren recursively restores all trashed children of a folder.
func (s *Store) RestoreChildren(ctx context.Context, parentID string) error {
	_, err := s.db.Exec(ctx,
		`WITH RECURSIVE tree AS (
			SELECT id FROM drive_items WHERE parent_id = $1
			UNION ALL
			SELECT di.id FROM drive_items di JOIN tree t ON di.parent_id = t.id
		)
		UPDATE drive_items SET status = 'active', trashed_at = NULL, updated_at = NOW()
		WHERE id IN (SELECT id FROM tree) AND status = 'trashed'`, parentID)
	return err
}

// ErrFolderHasDocuments is returned when a delete would remove a folder (or one
// beneath it) that still holds text documents. text_documents.folder_id is
// ON DELETE RESTRICT: the writing in it is never destroyed as a side effect.
var ErrFolderHasDocuments = errors.New("folder holds text documents")

// CountTextDocumentsUnder counts the text documents in a folder and in every
// folder beneath it, in any state.
func (s *Store) CountTextDocumentsUnder(ctx context.Context, folderID string) (int, error) {
	var n int
	err := s.db.QueryRow(ctx,
		`WITH RECURSIVE tree AS (
			SELECT id FROM drive_items WHERE id = $1
			UNION ALL
			SELECT di.id FROM drive_items di JOIN tree t ON di.parent_id = t.id
		)
		SELECT count(*) FROM text_documents WHERE folder_id IN (SELECT id FROM tree)`, folderID).Scan(&n)
	return n, err
}

// DeleteItem permanently removes a drive item. It returns ErrFolderHasDocuments
// when the database refuses because text documents still hang on the folder or
// one beneath it (a document saved after the caller last looked).
func (s *Store) DeleteItem(ctx context.Context, id string) error {
	_, err := s.db.Exec(ctx, `DELETE FROM drive_items WHERE id = $1`, id)
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23503" && pgErr.ConstraintName == "text_documents_folder_id_fkey" {
		return ErrFolderHasDocuments
	}
	return err
}

// GetChildFiles returns all file items under a folder recursively (for permanent delete + quota).
func (s *Store) GetChildFiles(ctx context.Context, parentID string) ([]*DriveItem, error) {
	rows, err := s.db.Query(ctx,
		`WITH RECURSIVE tree AS (
			SELECT id FROM drive_items WHERE parent_id = $1
			UNION ALL
			SELECT di.id FROM drive_items di JOIN tree t ON di.parent_id = t.id
		)
		SELECT id, workspace_id, drive_context, COALESCE(drive_context_id,''), parent_id,
			item_type, name, mime_type, size_bytes, object_key, storage_doc_id, ngac_node_id,
			COALESCE(scope_oa_id,''), owner_id, status, trashed_at, created_at, updated_at
		FROM drive_items WHERE id IN (SELECT id FROM tree) AND item_type = 'file'`, parentID)
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

// FindRootByContext finds the root folder for a workspace or channel drive.
func (s *Store) FindRootByContext(ctx context.Context, workspaceID, driveContext, driveContextID string) (*DriveItem, error) {
	item := &DriveItem{}
	err := s.db.QueryRow(ctx,
		`SELECT id, workspace_id, drive_context, COALESCE(drive_context_id,''), parent_id,
			item_type, name, mime_type, size_bytes, object_key, storage_doc_id, ngac_node_id,
			COALESCE(scope_oa_id,''), owner_id, status, trashed_at, created_at, updated_at
		 FROM drive_items
		 WHERE workspace_id = $1 AND drive_context = $2
		   AND COALESCE(drive_context_id,'') = $3
		   AND parent_id IS NULL AND item_type = 'folder' AND is_root AND status = 'active'
		 LIMIT 1`, workspaceID, driveContext, driveContextID).
		Scan(&item.ID, &item.WorkspaceID, &item.DriveContext, &item.DriveContextID,
			&item.ParentID, &item.ItemType, &item.Name, &item.MimeType, &item.SizeBytes,
			&item.ObjectKey, &item.StorageDocID, &item.NGACNodeID, &item.ScopeOAID,
			&item.OwnerID, &item.Status, &item.TrashedAt, &item.CreatedAt, &item.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return item, err
}

// RootWorkspaceByNode returns the workspace whose drive root hangs on the given
// OA, or "" when no root does. A drive OA that is the root of one workspace must
// never be adopted by another.
func (s *Store) RootWorkspaceByNode(ctx context.Context, nodeID string) (string, error) {
	var ws string
	err := s.db.QueryRow(ctx,
		`SELECT workspace_id FROM drive_items WHERE ngac_node_id = $1 AND is_root LIMIT 1`, nodeID).Scan(&ws)
	if err == pgx.ErrNoRows {
		return "", nil
	}
	return ws, err
}

// --- Shares ---

// InsertShare creates a drive share record.
func (s *Store) InsertShare(ctx context.Context, share *DriveShare) error {
	if share.ID == "" {
		share.ID = uuid.New().String()
	}
	_, err := s.db.Exec(ctx,
		`INSERT INTO drive_shares (id, drive_item_id, share_type, target_ngac_id, target_label,
			operations, ngac_share_oa, created_by)
		 VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		share.ID, share.DriveItemID, share.ShareType, share.TargetNGACID, share.TargetLabel,
		share.Operations, share.NGACShareOA, share.CreatedBy)
	return err
}

// GetShare retrieves a share by ID.
func (s *Store) GetShare(ctx context.Context, id string) (*DriveShare, error) {
	share := &DriveShare{}
	err := s.db.QueryRow(ctx,
		`SELECT id, drive_item_id, share_type, target_ngac_id, target_label,
			operations, ngac_share_oa, created_by, created_at
		 FROM drive_shares WHERE id = $1`, id).
		Scan(&share.ID, &share.DriveItemID, &share.ShareType, &share.TargetNGACID,
			&share.TargetLabel, &share.Operations, &share.NGACShareOA, &share.CreatedBy,
			&share.CreatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	return share, err
}

// ListSharesByItem returns all shares for a drive item.
func (s *Store) ListSharesByItem(ctx context.Context, itemID string) ([]*DriveShare, error) {
	rows, err := s.db.Query(ctx,
		`SELECT id, drive_item_id, share_type, target_ngac_id, target_label,
			operations, ngac_share_oa, created_by, created_at
		 FROM drive_shares WHERE drive_item_id = $1 ORDER BY created_at`, itemID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var shares []*DriveShare
	for rows.Next() {
		share := &DriveShare{}
		if err := rows.Scan(&share.ID, &share.DriveItemID, &share.ShareType, &share.TargetNGACID,
			&share.TargetLabel, &share.Operations, &share.NGACShareOA, &share.CreatedBy,
			&share.CreatedAt); err != nil {
			return nil, err
		}
		shares = append(shares, share)
	}
	return shares, nil
}

// ListSharesByTarget returns shares targeting a specific NGAC node.
func (s *Store) ListSharesByTarget(ctx context.Context, targetNGACIDs []string) ([]*DriveShare, error) {
	rows, err := s.db.Query(ctx,
		`SELECT ds.id, ds.drive_item_id, ds.share_type, ds.target_ngac_id, ds.target_label,
			ds.operations, ds.ngac_share_oa, ds.created_by, ds.created_at
		 FROM drive_shares ds
		 JOIN drive_items di ON ds.drive_item_id = di.id
		 WHERE (ds.target_ngac_id = ANY($1) OR ds.share_type = 'public')
		   AND di.status = 'active'
		 ORDER BY ds.created_at DESC`, targetNGACIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var shares []*DriveShare
	for rows.Next() {
		share := &DriveShare{}
		if err := rows.Scan(&share.ID, &share.DriveItemID, &share.ShareType, &share.TargetNGACID,
			&share.TargetLabel, &share.Operations, &share.NGACShareOA, &share.CreatedBy,
			&share.CreatedAt); err != nil {
			return nil, err
		}
		shares = append(shares, share)
	}
	return shares, nil
}

// DeleteShare removes a share record.
func (s *Store) DeleteShare(ctx context.Context, id string) error {
	_, err := s.db.Exec(ctx, `DELETE FROM drive_shares WHERE id = $1`, id)
	return err
}

// --- Quotas ---

// GetOrCreateQuota returns the quota for a workspace, creating a default if needed.
func (s *Store) GetOrCreateQuota(ctx context.Context, workspaceID string) (*DriveQuota, error) {
	q := &DriveQuota{}
	err := s.db.QueryRow(ctx,
		`INSERT INTO drive_quotas (workspace_id) VALUES ($1)
		 ON CONFLICT (workspace_id) DO NOTHING;
		 SELECT workspace_id, max_bytes, used_bytes, max_files, used_files, updated_at
		 FROM drive_quotas WHERE workspace_id = $1`, workspaceID).
		Scan(&q.WorkspaceID, &q.MaxBytes, &q.UsedBytes, &q.MaxFiles, &q.UsedFiles, &q.UpdatedAt)
	if err != nil {
		// Try separate query (some drivers don't support multi-statement)
		s.db.Exec(ctx, `INSERT INTO drive_quotas (workspace_id) VALUES ($1) ON CONFLICT DO NOTHING`, workspaceID)
		err = s.db.QueryRow(ctx,
			`SELECT workspace_id, max_bytes, used_bytes, max_files, used_files, updated_at
			 FROM drive_quotas WHERE workspace_id = $1`, workspaceID).
			Scan(&q.WorkspaceID, &q.MaxBytes, &q.UsedBytes, &q.MaxFiles, &q.UsedFiles, &q.UpdatedAt)
	}
	return q, err
}

// UpdateQuotaLimits sets the max_bytes and max_files for a workspace.
func (s *Store) UpdateQuotaLimits(ctx context.Context, workspaceID string, maxBytes int64, maxFiles int32) error {
	_, err := s.db.Exec(ctx,
		`UPDATE drive_quotas SET max_bytes = $1, max_files = $2, updated_at = NOW()
		 WHERE workspace_id = $3`, maxBytes, maxFiles, workspaceID)
	return err
}

// IncrementQuota adds to used_bytes and used_files atomically.
func (s *Store) IncrementQuota(ctx context.Context, workspaceID string, bytes int64, files int32) error {
	_, err := s.db.Exec(ctx,
		`UPDATE drive_quotas SET used_bytes = used_bytes + $1, used_files = used_files + $2,
			updated_at = NOW()
		 WHERE workspace_id = $3`, bytes, files, workspaceID)
	return err
}

// DecrementQuota subtracts from used_bytes and used_files atomically.
func (s *Store) DecrementQuota(ctx context.Context, workspaceID string, bytes int64, files int32) error {
	_, err := s.db.Exec(ctx,
		`UPDATE drive_quotas SET used_bytes = GREATEST(0, used_bytes - $1),
			used_files = GREATEST(0, used_files - $2), updated_at = NOW()
		 WHERE workspace_id = $3`, bytes, files, workspaceID)
	return err
}

// CheckQuota returns true if the workspace has room for the given size.
func (s *Store) CheckQuota(ctx context.Context, workspaceID string, additionalBytes int64) (bool, error) {
	q, err := s.GetOrCreateQuota(ctx, workspaceID)
	if err != nil {
		return false, err
	}
	if q.MaxBytes >= 0 && q.UsedBytes+additionalBytes > q.MaxBytes {
		return false, nil
	}
	if q.MaxFiles >= 0 && q.UsedFiles+1 > q.MaxFiles {
		return false, nil
	}
	return true, nil
}

func nilStr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// UpdateFileSize updates the size_bytes column for a drive item.
func (s *Store) UpdateFileSize(ctx context.Context, id string, sizeBytes int64) {
	s.db.Exec(ctx, `UPDATE drive_items SET size_bytes = $1 WHERE id = $2`, sizeBytes, id)
}

// GetWorkspacePCID returns the NGAC PC node ID for a workspace.
func (s *Store) GetWorkspacePCID(ctx context.Context, workspaceID string) (string, error) {
	var pcID string
	err := s.db.QueryRow(ctx,
		`SELECT ngac_pc_id FROM workspaces WHERE id = $1`, workspaceID).Scan(&pcID)
	if err == pgx.ErrNoRows {
		return "", nil
	}
	return pcID, err
}

// GetWorkspaceDocumentsOAID returns the Documents OA recorded for a workspace,
// or "" when the workspace has none recorded (or does not exist). The drive roots
// itself there; it never works the OA out from node names.
func (s *Store) GetWorkspaceDocumentsOAID(ctx context.Context, workspaceID string) (string, error) {
	var oaID string
	err := s.db.QueryRow(ctx,
		`SELECT COALESCE(documents_oa_id, '') FROM workspaces WHERE id = $1`, workspaceID).Scan(&oaID)
	if err == pgx.ErrNoRows {
		return "", nil
	}
	return oaID, err
}

// GetChannelWorkspaceID returns the workspace_id for a channel.
func (s *Store) GetChannelWorkspaceID(ctx context.Context, channelID string) (string, error) {
	var wsID string
	err := s.db.QueryRow(ctx,
		`SELECT COALESCE(workspace_id, '') FROM channels WHERE id = $1`, channelID).Scan(&wsID)
	if err == pgx.ErrNoRows {
		return "", nil
	}
	return wsID, err
}
