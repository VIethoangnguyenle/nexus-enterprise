// Package texts is the text-document feature of the document service:
// documents written in the app, saved with optimistic concurrency and
// authorized on the OA of the folder they sit in.
package texts

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Status values of a text document.
const (
	StatusDraft    = "draft"
	StatusActive   = "active"
	StatusArchived = "archived"
)

// Sentinel errors of the store. The service turns them into the errors a
// caller sees.
var (
	ErrNotFound        = errors.New("text document not found")
	ErrVersionConflict = errors.New("text document changed since it was read")
)

// Doc is one text document. OAID is the attribute access is decided on: the
// OA of the folder it sits in, or the workspace's Documents OA at the top. It is
// empty when the document has no place a permission could come from (its folder
// is gone), and then nothing is allowed.
type Doc struct {
	ID           string
	WorkspaceID  string
	FolderID     string
	Title        string
	Content      string
	Version      int
	Status       string
	OwnerID      string
	OwnerName    string
	LastEditorID string
	CreatedAt    time.Time
	UpdatedAt    time.Time
	OAID         string
}

// Store reads and writes text_documents.
type Store struct{ db *pgxpool.Pool }

// NewStore returns a Store on db.
func NewStore(db *pgxpool.Pool) *Store { return &Store{db: db} }

// The OA a document is authorized on, worked out at read time from where the
// document sits now rather than copied into the row, so a folder that goes away
// takes the document's permissions with it instead of leaving a stale grant.
const oaExpr = `CASE
	WHEN d.folder_id IS NULL THEN COALESCE(
		w.documents_oa_id,
		(SELECT r.ngac_node_id FROM drive_items r
		  WHERE r.workspace_id = d.workspace_id AND r.drive_context = 'workspace'
		    AND r.is_root AND r.status = 'active' LIMIT 1),
		'')
	ELSE COALESCE(f.ngac_node_id, '')
END`

// Names come only from people who belong to the workspace: someone who has left
// is shown as nobody rather than by name.
const selectDoc = `SELECT d.id, d.workspace_id, COALESCE(d.folder_id, ''), d.title, %s, d.version, d.status,
		COALESCE(d.owner_id, ''), COALESCE(NULLIF(o.display_name, ''), o.username, ''), COALESCE(d.last_editor_id, ''),
		d.created_at, d.updated_at, ` + oaExpr + `
	FROM text_documents d
	JOIN workspaces w ON w.id = d.workspace_id
	LEFT JOIN drive_items f ON f.id = d.folder_id AND f.status = 'active' AND f.item_type = 'folder'
	LEFT JOIN tenant_users tu ON tu.tenant_id = d.workspace_id AND tu.user_id = d.owner_id
	LEFT JOIN users o ON o.id = tu.user_id
	`

func scanDoc(row pgx.Row) (*Doc, error) {
	d := &Doc{}
	err := row.Scan(&d.ID, &d.WorkspaceID, &d.FolderID, &d.Title, &d.Content, &d.Version, &d.Status,
		&d.OwnerID, &d.OwnerName, &d.LastEditorID, &d.CreatedAt, &d.UpdatedAt, &d.OAID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	return d, err
}

// ParentOA returns the OA new documents in folderID (or at the top of the
// workspace, when empty) are authorized on. ErrNotFound when the folder is not
// an active folder of workspaceID, or the workspace has no Documents OA.
func (s *Store) ParentOA(ctx context.Context, workspaceID, folderID string) (string, error) {
	var oa string
	var err error
	if folderID != "" {
		err = s.db.QueryRow(ctx,
			`SELECT ngac_node_id FROM drive_items
			  WHERE id = $1 AND workspace_id = $2 AND item_type = 'folder' AND status = 'active'`,
			folderID, workspaceID).Scan(&oa)
	} else {
		err = s.db.QueryRow(ctx,
			`SELECT COALESCE(w.documents_oa_id,
				(SELECT r.ngac_node_id FROM drive_items r
				  WHERE r.workspace_id = w.id AND r.drive_context = 'workspace' AND r.is_root AND r.status = 'active' LIMIT 1),
				'')
			   FROM workspaces w WHERE w.id = $1`, workspaceID).Scan(&oa)
	}
	if errors.Is(err, pgx.ErrNoRows) || (err == nil && oa == "") {
		return "", ErrNotFound
	}
	return oa, err
}

// Insert stores a new document at version 1.
func (s *Store) Insert(ctx context.Context, d *Doc) (*Doc, error) {
	var folder *string
	if d.FolderID != "" {
		folder = &d.FolderID
	}
	var id string
	err := s.db.QueryRow(ctx,
		`INSERT INTO text_documents (workspace_id, folder_id, title, content, status, owner_id, last_editor_id)
		 VALUES ($1, $2, $3, $4, $5, $6, $6) RETURNING id`,
		d.WorkspaceID, folder, d.Title, d.Content, d.Status, d.OwnerID).Scan(&id)
	if err != nil {
		return nil, fmt.Errorf("insert text document: %w", err)
	}
	return s.Get(ctx, id)
}

// Get returns one document with its content.
func (s *Store) Get(ctx context.Context, id string) (*Doc, error) {
	return scanDoc(s.db.QueryRow(ctx, fmt.Sprintf(selectDoc, "d.content")+`WHERE d.id = $1`, id))
}

// ListFilter narrows List. Viewer is the calling user.
type ListFilter struct {
	WorkspaceID string
	Viewer      string
	// Scope is "", "mine" (written by the viewer), "drafts" (the viewer's
	// drafts) or "shared" (written by someone else).
	Scope string
}

func (f ListFilter) where() (string, []any) {
	where := `WHERE d.workspace_id = $1`
	args := []any{f.WorkspaceID}
	switch f.Scope {
	case "mine":
		where += ` AND d.owner_id = $2`
		args = append(args, f.Viewer)
	case "drafts":
		where += ` AND d.owner_id = $2 AND d.status = 'draft'`
		args = append(args, f.Viewer)
	case "shared":
		// A document whose author is gone belongs to someone else.
		where += ` AND d.owner_id IS DISTINCT FROM $2`
		args = append(args, f.Viewer)
	}
	return where, args
}

// Cursor marks where the next page of a listing starts: the listing is ordered
// by last edit, newest first, then by id.
type Cursor struct {
	UpdatedAt time.Time
	ID        string
}

// List returns up to limit documents after the cursor (nil: from the start),
// without their content, newest edit first. It does not decide who may see
// them; the caller filters by access.
func (s *Store) List(ctx context.Context, f ListFilter, after *Cursor, limit int) ([]*Doc, error) {
	where, args := f.where()
	if after != nil {
		args = append(args, after.UpdatedAt, after.ID)
		where += fmt.Sprintf(` AND (d.updated_at, d.id) < ($%d, $%d)`, len(args)-1, len(args))
	}
	args = append(args, limit)
	rows, err := s.db.Query(ctx,
		fmt.Sprintf(selectDoc, "''")+where+fmt.Sprintf(` ORDER BY d.updated_at DESC, d.id DESC LIMIT $%d`, len(args)), args...)
	if err != nil {
		return nil, fmt.Errorf("list text documents: %w", err)
	}
	defer rows.Close()
	var out []*Doc
	for rows.Next() {
		d, err := scanDoc(rows)
		if err != nil {
			return nil, fmt.Errorf("scan text document: %w", err)
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// CountByOA counts the documents of a listing per OA, so a caller can count
// what it may read with one policy call and without loading a single document.
func (s *Store) CountByOA(ctx context.Context, f ListFilter) (map[string]int, error) {
	where, args := f.where()
	rows, err := s.db.Query(ctx,
		`SELECT `+oaExpr+`, count(*)
		   FROM text_documents d
		   JOIN workspaces w ON w.id = d.workspace_id
		   LEFT JOIN drive_items f ON f.id = d.folder_id AND f.status = 'active' AND f.item_type = 'folder' `+where+`
		  GROUP BY 1`, args...)
	if err != nil {
		return nil, fmt.Errorf("count text documents: %w", err)
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var oa string
		var n int
		if err := rows.Scan(&oa, &n); err != nil {
			return nil, fmt.Errorf("scan count: %w", err)
		}
		out[oa] = n
	}
	return out, rows.Err()
}

// Patch is the set of fields a save may change; nil leaves a field alone.
type Patch struct {
	Title   *string
	Content *string
	Status  *string
}

// Update applies patch if the row is still at baseVersion, in one statement, so
// two saves racing from the same version cannot both win. When the row has moved
// on it returns ErrVersionConflict together with the current document.
//
// The document handed back is read in the same transaction as the write: our
// UPDATE holds the row until commit, so it is the version this call produced and
// not a later writer's. A client that adopted the later version as its base would
// overwrite that writer's change without ever seeing a conflict.
func (s *Store) Update(ctx context.Context, id string, baseVersion int, p Patch, editor string) (*Doc, error) {
	var cur *Doc
	var conflict bool
	err := pgx.BeginFunc(ctx, s.db, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx,
			`UPDATE text_documents
			    SET title = COALESCE($3, title), content = COALESCE($4, content), status = COALESCE($5, status),
			        version = version + 1, last_editor_id = $6, updated_at = NOW()
			  WHERE id = $1 AND version = $2`,
			id, baseVersion, p.Title, p.Content, p.Status, editor)
		if err != nil {
			return fmt.Errorf("update text document: %w", err)
		}
		conflict = tag.RowsAffected() == 0
		cur, err = scanDoc(tx.QueryRow(ctx, fmt.Sprintf(selectDoc, "d.content")+`WHERE d.id = $1`, id))
		return err
	})
	if err != nil {
		return nil, err
	}
	if conflict {
		return cur, ErrVersionConflict
	}
	return cur, nil
}

// Delete removes a document.
func (s *Store) Delete(ctx context.Context, id string) error {
	tag, err := s.db.Exec(ctx, `DELETE FROM text_documents WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete text document: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
