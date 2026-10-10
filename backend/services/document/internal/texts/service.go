package texts

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"ngac-platform/ngac"
	"ngac-platform/pkg/httputil"
	policypb "ngac-platform/proto/policy"
)

// The sentinels are the shared ones, so httputil.MapDomainError understands
// them; a conflict is its own error because it carries the current document.
var (
	ErrAccessDenied = httputil.ErrAccessDenied
	ErrInvalidInput = httputil.ErrInvalidInput
)

// Limits of what a document holds.
const (
	MaxTitleRunes   = 200
	MaxContentBytes = 1 << 20
)

// DefaultTitle names a document created without one.
const DefaultTitle = "Văn bản chưa đặt tên"

// ConflictError is a save refused because the document moved on since the
// version it was based on. Current is the document as it now stands, when the
// caller may read it; a caller who may only write learns the version and nothing
// else (Readable is false and Current carries no title, content or owner).
type ConflictError struct {
	Current  *Doc
	Readable bool
}

func (e *ConflictError) Error() string { return ErrVersionConflict.Error() }
func (e *ConflictError) Unwrap() error { return ErrVersionConflict }

// Caller is the verified identity behind a request.
type Caller struct {
	UserID     string
	NGACNodeID string
}

// Service holds the rules: who may do what to which document, and what a
// document may contain. Every method fails closed: no caller, an OA that cannot
// be resolved or a policy answer that cannot be had all deny.
type Service struct {
	store  *Store
	policy policypb.PolicyReadServiceClient
}

// NewService returns a Service on store, deciding access through policy.
func NewService(store *Store, policy policypb.PolicyReadServiceClient) *Service {
	return &Service{store: store, policy: policy}
}

func denied(op string) error { return fmt.Errorf("%w: no %s access", ErrAccessDenied, op) }

// authorize requires op for the caller on one OA.
func (s *Service) authorize(ctx context.Context, who Caller, oaID, op string) error {
	if who.NGACNodeID == "" || who.UserID == "" || oaID == "" {
		return denied(op)
	}
	resp, err := s.policy.CheckAccess(ctx, &policypb.CheckAccessRequest{
		UserNodeId: who.NGACNodeID, ObjectNodeId: oaID, Operation: op,
	})
	if !ngac.Allowed(resp.GetDecision(), err) {
		return denied(op)
	}
	return nil
}

// load reads a document and requires op on its OA. A document that does not
// exist and one the caller may not touch answer alike: whether an ID exists is
// not the caller's to learn.
func (s *Service) load(ctx context.Context, who Caller, id, op string) (*Doc, error) {
	d, err := s.store.Get(ctx, id)
	if errors.Is(err, ErrNotFound) {
		return nil, denied(op)
	}
	if err != nil {
		return nil, err
	}
	if err := s.authorize(ctx, who, d.OAID, op); err != nil {
		return nil, err
	}
	return d, nil
}

func validTitle(title string) (string, error) {
	t := strings.TrimSpace(title)
	if t == "" {
		return "", fmt.Errorf("%w: title is required", ErrInvalidInput)
	}
	if utf8.RuneCountInString(t) > MaxTitleRunes || !utf8.ValidString(t) || strings.ContainsRune(t, 0) {
		return "", fmt.Errorf("%w: title is too long or malformed", ErrInvalidInput)
	}
	return t, nil
}

func validContent(content string) error {
	if len(content) > MaxContentBytes {
		return fmt.Errorf("%w: document is too large", ErrInvalidInput)
	}
	if !utf8.ValidString(content) || strings.ContainsRune(content, 0) {
		return fmt.Errorf("%w: document content is malformed", ErrInvalidInput)
	}
	return nil
}

func validStatus(status string) error {
	switch status {
	case StatusDraft, StatusActive, StatusArchived:
		return nil
	}
	return fmt.Errorf("%w: unknown status", ErrInvalidInput)
}

// Create writes a new draft into folderID (the top of the workspace's Documents
// when empty). It takes write on the OA of that place.
func (s *Service) Create(ctx context.Context, who Caller, workspaceID, folderID, title string) (*Doc, error) {
	if strings.TrimSpace(title) == "" {
		title = DefaultTitle
	}
	title, err := validTitle(title)
	if err != nil {
		return nil, err
	}
	oa, err := s.store.ParentOA(ctx, workspaceID, folderID)
	if errors.Is(err, ErrNotFound) {
		return nil, denied(ngac.OpWrite)
	}
	if err != nil {
		return nil, err
	}
	if err := s.authorize(ctx, who, oa, ngac.OpWrite); err != nil {
		return nil, err
	}
	return s.store.Insert(ctx, &Doc{
		WorkspaceID: workspaceID, FolderID: folderID, Title: title, Status: StatusDraft, OwnerID: who.UserID,
	})
}

// Opened is a document with what the caller may do to it. CanRead says whether
// the caller may be shown the document's text and title: only then does a
// response carry them.
type Opened struct {
	*Doc
	CanRead  bool
	CanWrite bool
}

// Get opens a document; it takes read, and reports whether write is held too.
func (s *Service) Get(ctx context.Context, who Caller, id string) (*Opened, error) {
	d, err := s.load(ctx, who, id, ngac.OpRead)
	if err != nil {
		return nil, err
	}
	return &Opened{Doc: d, CanRead: true, CanWrite: s.authorize(ctx, who, d.OAID, ngac.OpWrite) == nil}, nil
}

// Page limits. A listing is bounded: a workspace can hold far more documents
// than a screen shows, and the caller pages through them.
const (
	DefaultPageSize = 50
	MaxPageSize     = 200
	// maxScan bounds how many rows one call reads while looking for readable
	// ones, so a caller who may read almost nothing cannot make it walk a table.
	maxScan = 2000
)

// Page is one page of a listing. Next is empty when there is nothing after it.
type Page struct {
	Docs []*Opened
	Next string
}

func encodeCursor(d *Doc) string {
	raw := fmt.Sprintf("%d|%s", d.UpdatedAt.UnixMicro(), d.ID)
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

func decodeCursor(c string) (*Cursor, error) {
	if c == "" {
		return nil, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(c)
	if err != nil {
		return nil, fmt.Errorf("%w: bad cursor", ErrInvalidInput)
	}
	micro, id, ok := strings.Cut(string(raw), "|")
	n, perr := strconv.ParseInt(micro, 10, 64)
	if !ok || perr != nil || id == "" {
		return nil, fmt.Errorf("%w: bad cursor", ErrInvalidInput)
	}
	return &Cursor{UpdatedAt: time.UnixMicro(n).UTC(), ID: id}, nil
}

func checkScope(scope string) error {
	switch scope {
	case "", "mine", "drafts", "shared":
		return nil
	}
	return fmt.Errorf("%w: unknown scope", ErrInvalidInput)
}

// readable asks the policy service, in one call, which of the OAs the caller may
// read and which of those it may also write. An unreadable answer is an error,
// never "nothing allowed" and never "everything allowed".
func (s *Service) readable(ctx context.Context, who Caller, oas []string) (map[string]*policypb.ObjectPermissions, error) {
	if len(oas) == 0 {
		return nil, nil
	}
	batch, err := s.policy.BatchCheckAccess(ctx, &policypb.BatchCheckAccessRequest{
		UserNodeId: who.NGACNodeID, ObjectIds: oas, Operations: []string{ngac.OpRead, ngac.OpWrite},
	})
	if err != nil {
		return nil, fmt.Errorf("batch access check: %w", err)
	}
	return batch.GetResults(), nil
}

// List returns one page of the workspace's documents the caller may read,
// without their content, newest edit first, each with whether the caller may
// also write. scope is "", "mine", "drafts" or "shared"; cursor is a previous
// page's Next; limit defaults to 50 and is capped at 200.
func (s *Service) List(ctx context.Context, who Caller, workspaceID, scope, cursor string, limit int) (*Page, error) {
	if who.NGACNodeID == "" || who.UserID == "" {
		return nil, denied(ngac.OpRead)
	}
	if err := checkScope(scope); err != nil {
		return nil, err
	}
	after, err := decodeCursor(cursor)
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = DefaultPageSize
	}
	if limit > MaxPageSize {
		limit = MaxPageSize
	}
	filter := ListFilter{WorkspaceID: workspaceID, Viewer: who.UserID, Scope: scope}
	page := &Page{Docs: []*Opened{}}
	scanned := 0
	for len(page.Docs) < limit && scanned < maxScan {
		rows, err := s.store.List(ctx, filter, after, limit)
		if err != nil {
			return nil, err
		}
		if len(rows) == 0 {
			return page, nil
		}
		seen := map[string]bool{}
		var oas []string
		for _, d := range rows {
			if d.OAID != "" && !seen[d.OAID] {
				seen[d.OAID] = true
				oas = append(oas, d.OAID)
			}
		}
		perms, err := s.readable(ctx, who, oas)
		if err != nil {
			// An unreadable answer must not list everything.
			return nil, err
		}
		for i, d := range rows {
			scanned++
			after = &Cursor{UpdatedAt: d.UpdatedAt, ID: d.ID}
			if d.OAID != "" && perms[d.OAID].GetPermissions()[ngac.OpRead] {
				page.Docs = append(page.Docs, &Opened{Doc: d, CanRead: true, CanWrite: perms[d.OAID].GetPermissions()[ngac.OpWrite]})
				if len(page.Docs) == limit {
					if i < len(rows)-1 || len(rows) == limit {
						page.Next = encodeCursor(d)
					}
					return page, nil
				}
			}
		}
		if len(rows) < limit {
			return page, nil // the table is exhausted
		}
	}
	if after != nil && scanned >= maxScan {
		page.Next = base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf("%d|%s", after.UpdatedAt.UnixMicro(), after.ID)))
	}
	return page, nil
}

// Count says how many documents of a listing the caller may read, without
// loading any of them: the rows are counted per OA, and the OAs asked about in
// one policy call.
func (s *Service) Count(ctx context.Context, who Caller, workspaceID, scope string) (int, error) {
	if who.NGACNodeID == "" || who.UserID == "" {
		return 0, denied(ngac.OpRead)
	}
	if err := checkScope(scope); err != nil {
		return 0, err
	}
	counts, err := s.store.CountByOA(ctx, ListFilter{WorkspaceID: workspaceID, Viewer: who.UserID, Scope: scope})
	if err != nil {
		return 0, err
	}
	oas := make([]string, 0, len(counts))
	for oa := range counts {
		if oa != "" {
			oas = append(oas, oa)
		}
	}
	perms, err := s.readable(ctx, who, oas)
	if err != nil {
		return 0, err
	}
	total := 0
	for _, oa := range oas {
		if perms[oa].GetPermissions()[ngac.OpRead] {
			total += counts[oa]
		}
	}
	return total, nil
}

// Change is one save: the version it was based on and what to change.
type Change struct {
	BaseVersion int
	Title       *string
	Content     *string
	Status      *string
}

// Update saves a change. It takes write on the document's OA, validates what is
// to be stored, and applies it only if the document is still at BaseVersion;
// otherwise it returns a *ConflictError holding the current document, and
// nothing is written.
func (s *Service) Update(ctx context.Context, who Caller, id string, ch Change) (*Opened, error) {
	d, err := s.load(ctx, who, id, ngac.OpWrite)
	if err != nil {
		return nil, err
	}
	if ch.BaseVersion < 1 {
		return nil, fmt.Errorf("%w: base_version is required", ErrInvalidInput)
	}
	if ch.Title == nil && ch.Content == nil && ch.Status == nil {
		return nil, fmt.Errorf("%w: nothing to change", ErrInvalidInput)
	}
	p := Patch{Content: ch.Content, Status: ch.Status}
	if ch.Title != nil {
		t, err := validTitle(*ch.Title)
		if err != nil {
			return nil, err
		}
		p.Title = &t
	}
	if ch.Content != nil {
		if err := validContent(*ch.Content); err != nil {
			return nil, err
		}
		clean := Sanitize(*ch.Content)
		p.Content = &clean
	}
	if ch.Status != nil {
		if err := validStatus(*ch.Status); err != nil {
			return nil, err
		}
	}
	// Writing does not carry the right to read. What comes back (the saved text,
	// or the other version in a conflict) is shown only to someone who may read it.
	canRead := s.authorize(ctx, who, d.OAID, ngac.OpRead) == nil
	cur, err := s.store.Update(ctx, d.ID, ch.BaseVersion, p, who.UserID)
	switch {
	case errors.Is(err, ErrVersionConflict):
		if !canRead {
			return nil, &ConflictError{Current: &Doc{ID: cur.ID, WorkspaceID: cur.WorkspaceID, Version: cur.Version}}
		}
		return nil, &ConflictError{Current: cur, Readable: true}
	case errors.Is(err, ErrNotFound):
		// Deleted while the save was in flight: the same answer as for any
		// document the caller can no longer touch.
		return nil, denied(ngac.OpWrite)
	case err != nil:
		return nil, err
	}
	if !canRead {
		cur = &Doc{ID: cur.ID, WorkspaceID: cur.WorkspaceID, Version: cur.Version, Status: cur.Status, UpdatedAt: cur.UpdatedAt}
	}
	return &Opened{Doc: cur, CanRead: canRead, CanWrite: true}, nil
}

// Delete removes a document; it takes write on the document's OA.
func (s *Service) Delete(ctx context.Context, who Caller, id string) error {
	d, err := s.load(ctx, who, id, ngac.OpWrite)
	if err != nil {
		return err
	}
	if err := s.store.Delete(ctx, d.ID); err != nil && !errors.Is(err, ErrNotFound) {
		return err
	}
	return nil
}
