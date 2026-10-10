// Package store provides PostgreSQL data access for the approval service.
// Each public method executes a single query — no business logic here.
// All queries run on the tenant's schema via TenantConn.
package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"ngac-platform/pkg/httputil"
	"ngac-platform/services/approval/internal/domain"
)

// Store provides data access methods for approval workflow tables.
type Store struct {
	db *pgxpool.Pool
}

// NewStore creates a store backed by the given connection pool.
func NewStore(db *pgxpool.Pool) *Store {
	return &Store{db: db}
}

// conn acquires a tenant-scoped connection using the schema from context.
// Caller MUST call conn.Release() when done.
func (s *Store) conn(ctx context.Context) (*pgxpool.Conn, error) {
	schema := httputil.TenantSchemaFromCtx(ctx)
	if schema == "" {
		return nil, fmt.Errorf("tenant schema not set in context")
	}
	return httputil.TenantConn(ctx, s.db, schema)
}

// ProvisionSchema creates an isolated PostgreSQL schema for a tenant
// by calling the provision_tenant_schema() function in the public schema.
// Returns the schema name. This runs on the pool directly (no tenant scoping).
func (s *Store) ProvisionSchema(ctx context.Context, tenantID string) (string, error) {
	var schema string
	err := s.db.QueryRow(ctx, `SELECT provision_tenant_schema($1)`, tenantID).Scan(&schema)
	if err != nil {
		return "", fmt.Errorf("provision schema: %w", err)
	}
	return schema, nil
}

// InsertTemplate persists a new approval template with its conditions and steps.
func (s *Store) InsertTemplate(ctx context.Context, t *domain.Template) error {
	c, err := s.conn(ctx)
	if err != nil {
		return err
	}
	defer c.Release()

	tx, err := c.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// Serialize form fields to JSONB.
	var formFieldsJSON interface{}
	if len(t.FormFields) > 0 {
		b, err := json.Marshal(t.FormFields)
		if err != nil {
			return fmt.Errorf("marshal form fields: %w", err)
		}
		formFieldsJSON = string(b)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO approval_templates (id, name, entity_type, is_active, priority, form_fields, created_by, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		t.ID, t.Name, t.EntityType, t.IsActive, t.Priority, formFieldsJSON, t.CreatedBy, t.CreatedAt, t.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert template: %w", err)
	}

	for _, cond := range t.Conditions {
		_, err = tx.Exec(ctx, `
			INSERT INTO approval_conditions (id, template_id, field, operator, value)
			VALUES ($1, $2, $3, $4, $5)`,
			cond.ID, t.ID, cond.Field, cond.Operator, cond.Value,
		)
		if err != nil {
			return fmt.Errorf("insert condition: %w", err)
		}
	}

	for _, step := range t.Steps {
		_, err = tx.Exec(ctx, `
			INSERT INTO approval_steps (id, template_id, step_order, name, approver_type, approver_value, required_count, timeout_hours)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
			step.ID, t.ID, step.StepOrder, step.Name, step.ApproverType, step.ApproverValue, step.RequiredCount, step.TimeoutHours,
		)
		if err != nil {
			return fmt.Errorf("insert step: %w", err)
		}
	}

	return tx.Commit(ctx)
}

// GetTemplate retrieves a template by ID with its conditions and steps. The
// four reads share one REPEATABLE READ snapshot, so a template being edited
// at the same moment is read whole, never with the old steps and the new name.
func (s *Store) GetTemplate(ctx context.Context, id string) (*domain.Template, error) {
	c, err := s.conn(ctx)
	if err != nil {
		return nil, err
	}
	defer c.Release()

	tx, err := c.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, fmt.Errorf("begin read tx: %w", err)
	}
	defer tx.Rollback(ctx)

	t := &domain.Template{}
	var formFieldsJSON *string
	err = tx.QueryRow(ctx, `
		SELECT id, name, entity_type, is_active, priority, form_fields, created_by, created_at, updated_at
		FROM approval_templates WHERE id = $1`, id,
	).Scan(&t.ID, &t.Name, &t.EntityType, &t.IsActive, &t.Priority, &formFieldsJSON, &t.CreatedBy, &t.CreatedAt, &t.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get template: %w", err)
	}
	if formFieldsJSON != nil {
		if err := json.Unmarshal([]byte(*formFieldsJSON), &t.FormFields); err != nil {
			return nil, fmt.Errorf("unmarshal form fields: %w", err)
		}
	}

	if err := loadConditions(ctx, tx, []string{id}, func(_ string, cond *domain.Condition) { t.Conditions = append(t.Conditions, cond) }); err != nil {
		return nil, err
	}

	stepRows, err := tx.Query(ctx, `
		SELECT id, step_order, name, approver_type, COALESCE(approver_value, ''), COALESCE(required_count, 1), COALESCE(timeout_hours, 0)
		FROM approval_steps WHERE template_id = $1 ORDER BY step_order`, id)
	if err != nil {
		return nil, fmt.Errorf("list steps: %w", err)
	}
	defer stepRows.Close()

	for stepRows.Next() {
		step := &domain.Step{}
		if err := stepRows.Scan(&step.ID, &step.StepOrder, &step.Name, &step.ApproverType, &step.ApproverValue, &step.RequiredCount, &step.TimeoutHours); err != nil {
			return nil, fmt.Errorf("scan step: %w", err)
		}
		t.Steps = append(t.Steps, step)
	}
	if err := stepRows.Err(); err != nil {
		return nil, err
	}
	return t, nil
}

// loadConditions reads the conditions of the given templates and hands each to
// add with its template id.
func loadConditions(ctx context.Context, q interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}, templateIDs []string, add func(templateID string, c *domain.Condition)) error {
	rows, err := q.Query(ctx, `
		SELECT template_id::text, id::text, field, operator, value::text
		FROM approval_conditions WHERE template_id::text = ANY($1) ORDER BY id`, templateIDs)
	if err != nil {
		return fmt.Errorf("list conditions: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var tid string
		cond := &domain.Condition{}
		if err := rows.Scan(&tid, &cond.ID, &cond.Field, &cond.Operator, &cond.Value); err != nil {
			return fmt.Errorf("scan condition: %w", err)
		}
		add(tid, cond)
	}
	return rows.Err()
}

// ListTemplates retrieves templates filtered by entity type and active status.
func (s *Store) ListTemplates(ctx context.Context, entityType string, activeOnly bool) ([]*domain.Template, error) {
	c, err := s.conn(ctx)
	if err != nil {
		return nil, err
	}
	defer c.Release()

	query := `SELECT t.id, t.name, t.entity_type, t.is_active, t.priority, t.form_fields, t.created_by, t.created_at, t.updated_at,
		(SELECT COUNT(*) FROM approval_steps s WHERE s.template_id = t.id) AS step_count,
		(SELECT COUNT(*) FROM approval_conditions c WHERE c.template_id = t.id) AS condition_count
		FROM approval_templates t WHERE 1=1`
	args := []any{}
	argIdx := 1

	if entityType != "" {
		query += fmt.Sprintf(" AND t.entity_type = $%d", argIdx)
		args = append(args, entityType)
		argIdx++
	}
	if activeOnly {
		query += fmt.Sprintf(" AND t.is_active = $%d", argIdx)
		args = append(args, true)
	}
	query += " ORDER BY t.priority DESC, t.created_at DESC"

	rows, err := c.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list templates: %w", err)
	}
	defer rows.Close()

	var templates []*domain.Template
	for rows.Next() {
		t := &domain.Template{}
		var formFieldsJSON *string
		var stepCount, condCount int
		if err := rows.Scan(&t.ID, &t.Name, &t.EntityType, &t.IsActive, &t.Priority, &formFieldsJSON, &t.CreatedBy, &t.CreatedAt, &t.UpdatedAt, &stepCount, &condCount); err != nil {
			return nil, fmt.Errorf("scan template: %w", err)
		}
		if formFieldsJSON != nil {
			json.Unmarshal([]byte(*formFieldsJSON), &t.FormFields)
		}
		t.StepCount = stepCount
		t.ConditionCount = condCount
		templates = append(templates, t)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()

	// Conditions come with the list: choosing a template for a form means
	// judging them, and a list without them would let every template match.
	byID := make(map[string]*domain.Template, len(templates))
	ids := make([]string, 0, len(templates))
	for _, t := range templates {
		byID[t.ID] = t
		ids = append(ids, t.ID)
	}
	if err := loadConditions(ctx, c, ids, func(tid string, cond *domain.Condition) {
		byID[tid].Conditions = append(byID[tid].Conditions, cond)
	}); err != nil {
		return nil, err
	}
	return templates, nil
}

// UpdateTemplate saves a template's metadata, form fields, steps and
// conditions as one change. Steps and conditions are replaced wholesale by the
// template as given (the domain loads them first, so an edit that does not
// touch them hands the same ones back); requests already made keep their own
// frozen snapshot and are unaffected.
func (s *Store) UpdateTemplate(ctx context.Context, t *domain.Template, expected time.Time) (time.Time, error) {
	c, err := s.conn(ctx)
	if err != nil {
		return time.Time{}, err
	}
	defer c.Release()

	var formFieldsJSON interface{}
	if len(t.FormFields) > 0 {
		b, err := json.Marshal(t.FormFields)
		if err != nil {
			return time.Time{}, fmt.Errorf("marshal form fields: %w", err)
		}
		formFieldsJSON = string(b)
	}

	tx, err := c.Begin(ctx)
	if err != nil {
		return time.Time{}, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// The precondition and the write are one statement: of two edits made from
	// the same read, the second finds updated_at moved and is refused.
	var updatedAt time.Time
	err = tx.QueryRow(ctx, `
		UPDATE approval_templates SET name = $2, is_active = $3, priority = $4, form_fields = $5, updated_at = NOW()
		WHERE id = $1 AND updated_at = $6
		RETURNING updated_at`,
		t.ID, t.Name, t.IsActive, t.Priority, formFieldsJSON, expected,
	).Scan(&updatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		var exists bool
		if qerr := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM approval_templates WHERE id = $1)`, t.ID).Scan(&exists); qerr == nil && !exists {
			return time.Time{}, domain.ErrNotFound
		}
		return time.Time{}, domain.ErrStale
	}
	if err != nil {
		return time.Time{}, fmt.Errorf("update template: %w", err)
	}

	if _, err := tx.Exec(ctx, `DELETE FROM approval_steps WHERE template_id = $1`, t.ID); err != nil {
		return time.Time{}, fmt.Errorf("clear steps: %w", err)
	}
	if _, err := tx.Exec(ctx, `DELETE FROM approval_conditions WHERE template_id = $1`, t.ID); err != nil {
		return time.Time{}, fmt.Errorf("clear conditions: %w", err)
	}
	for _, cond := range t.Conditions {
		if _, err := tx.Exec(ctx, `
			INSERT INTO approval_conditions (id, template_id, field, operator, value)
			VALUES ($1, $2, $3, $4, $5)`,
			cond.ID, t.ID, cond.Field, cond.Operator, cond.Value,
		); err != nil {
			return time.Time{}, fmt.Errorf("insert condition: %w", err)
		}
	}
	for _, step := range t.Steps {
		if _, err := tx.Exec(ctx, `
			INSERT INTO approval_steps (id, template_id, step_order, name, approver_type, approver_value, required_count, timeout_hours)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
			step.ID, t.ID, step.StepOrder, step.Name, step.ApproverType, step.ApproverValue, step.RequiredCount, step.TimeoutHours,
		); err != nil {
			return time.Time{}, fmt.Errorf("insert step: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return time.Time{}, fmt.Errorf("commit: %w", err)
	}
	return updatedAt, nil
}

// InsertRequest persists a new approval request.
func (s *Store) InsertRequest(ctx context.Context, r *domain.Request) error {
	c, err := s.conn(ctx)
	if err != nil {
		return err
	}
	defer c.Release()

	var formData interface{}
	if r.FormDataJSON != "" {
		formData = r.FormDataJSON
	}

	_, err = c.Exec(ctx, `
		INSERT INTO approval_requests (id, entity_type, entity_id, template_id, template_name, template_snapshot, form_data_json, current_step, status, scope_oa_id, department_id, created_by, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`,
		r.ID, r.EntityType, r.EntityID, r.TemplateID, r.TemplateName, r.TemplateSnapshot, formData, r.CurrentStep, r.Status, r.ScopeOAID, r.DepartmentID, r.CreatedBy, r.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert request: %w", err)
	}
	return nil
}

// InsertRequestWithAssignments stores a request and its first assignments in
// one transaction: a request never exists with nobody to decide it.
func (s *Store) InsertRequestWithAssignments(ctx context.Context, r *domain.Request, assignments []*domain.AssignmentRecord) error {
	c, err := s.conn(ctx)
	if err != nil {
		return err
	}
	defer c.Release()

	tx, err := c.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	var formData interface{}
	if r.FormDataJSON != "" {
		formData = r.FormDataJSON
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO approval_requests (id, entity_type, entity_id, template_id, template_name, template_snapshot, form_data_json, current_step, status, scope_oa_id, department_id, created_by, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)`,
		r.ID, r.EntityType, r.EntityID, r.TemplateID, r.TemplateName, r.TemplateSnapshot, formData, r.CurrentStep, r.Status, r.ScopeOAID, r.DepartmentID, r.CreatedBy, r.CreatedAt,
	); err != nil {
		return fmt.Errorf("insert request: %w", err)
	}
	for _, a := range assignments {
		if _, err := tx.Exec(ctx, `
			INSERT INTO approval_assignments (id, request_id, step_order, user_node_id, grant_source, status)
			VALUES ($1, $2, $3, $4, $5, $6)`,
			a.ID, a.RequestID, a.StepOrder, a.UserNodeID, a.GrantSource, a.Status,
		); err != nil {
			return fmt.Errorf("insert assignment: %w", err)
		}
	}
	return tx.Commit(ctx)
}

// GetRequest retrieves an approval request by ID.
func (s *Store) GetRequest(ctx context.Context, id string) (*domain.Request, error) {
	c, err := s.conn(ctx)
	if err != nil {
		return nil, err
	}
	defer c.Release()

	r := &domain.Request{}
	var formDataJSON *string
	err = c.QueryRow(ctx, `
		SELECT id, entity_type, entity_id, template_id, template_name, template_snapshot, form_data_json, current_step, status,
		       scope_oa_id, department_id, created_by, created_at, completed_at
		FROM approval_requests WHERE id = $1`, id,
	).Scan(&r.ID, &r.EntityType, &r.EntityID, &r.TemplateID, &r.TemplateName, &r.TemplateSnapshot, &formDataJSON, &r.CurrentStep, &r.Status,
		&r.ScopeOAID, &r.DepartmentID, &r.CreatedBy, &r.CreatedAt, &r.CompletedAt)
	if formDataJSON != nil {
		r.FormDataJSON = *formDataJSON
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get request: %w", err)
	}
	return r, nil
}

// InsertAssignments bulk-inserts approval assignments for a request step.
func (s *Store) InsertAssignments(ctx context.Context, assignments []*domain.AssignmentRecord) error {
	c, err := s.conn(ctx)
	if err != nil {
		return err
	}
	defer c.Release()

	for _, a := range assignments {
		_, err := c.Exec(ctx, `
			INSERT INTO approval_assignments (id, request_id, step_order, user_node_id, grant_source, status)
			VALUES ($1, $2, $3, $4, $5, $6)`,
			a.ID, a.RequestID, a.StepOrder, a.UserNodeID, a.GrantSource, a.Status,
		)
		if err != nil {
			return fmt.Errorf("insert assignment: %w", err)
		}
	}
	return nil
}

// GetAssignment finds a specific user's pending assignment for a request.
func (s *Store) GetAssignment(ctx context.Context, requestID, userNodeID string) (*domain.AssignmentRecord, error) {
	c, err := s.conn(ctx)
	if err != nil {
		return nil, err
	}
	defer c.Release()

	a := &domain.AssignmentRecord{}
	var comment *string
	err = c.QueryRow(ctx, `
		SELECT id, request_id, step_order, user_node_id, grant_source, status, acted_at, comment
		FROM approval_assignments
		WHERE request_id = $1 AND user_node_id = $2 AND status = 'pending'`, requestID, userNodeID,
	).Scan(&a.ID, &a.RequestID, &a.StepOrder, &a.UserNodeID, &a.GrantSource, &a.Status, &a.ActedAt, &comment)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get assignment: %w", err)
	}
	if comment != nil {
		a.Comment = *comment
	}
	return a, nil
}

// HasAssignment reports whether any of the nodes has an assignment of any
// status on any step of the request.
func (s *Store) HasAssignment(ctx context.Context, requestID string, nodeIDs []string) (bool, error) {
	c, err := s.conn(ctx)
	if err != nil {
		return false, err
	}
	defer c.Release()

	var found bool
	err = c.QueryRow(ctx, `
		SELECT EXISTS (SELECT 1 FROM approval_assignments WHERE request_id = $1 AND user_node_id = ANY($2))`,
		requestID, nodeIDs).Scan(&found)
	if err != nil {
		return false, fmt.Errorf("has assignment: %w", err)
	}
	return found, nil
}

// FindGroupAssignment returns the pending role or department row of a step
// whose group is among groupNodeIDs. A group row is the one whose grant source
// names its own user_node_id (role:<ua> / department:<ua>), which a person's
// row never does.
func (s *Store) FindGroupAssignment(ctx context.Context, requestID string, stepOrder int, groupNodeIDs []string) (*domain.AssignmentRecord, error) {
	c, err := s.conn(ctx)
	if err != nil {
		return nil, err
	}
	defer c.Release()

	a := &domain.AssignmentRecord{}
	err = c.QueryRow(ctx, `
		SELECT id, request_id, step_order, user_node_id, grant_source, status
		FROM approval_assignments
		WHERE request_id = $1 AND step_order = $2 AND status = 'pending'
		  AND user_node_id = ANY($3)
		  AND grant_source IN ('role:' || user_node_id, 'department:' || user_node_id)
		ORDER BY id LIMIT 1`, requestID, stepOrder, groupNodeIDs,
	).Scan(&a.ID, &a.RequestID, &a.StepOrder, &a.UserNodeID, &a.GrantSource, &a.Status)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find group assignment: %w", err)
	}
	return a, nil
}

// InsertActedAssignment records a person's own decision on a step assigned to
// a group. The unique (request, step, person) index makes a second decision by
// the same person ErrAlreadyExists.
func (s *Store) InsertActedAssignment(ctx context.Context, a *domain.AssignmentRecord) error {
	c, err := s.conn(ctx)
	if err != nil {
		return err
	}
	defer c.Release()

	_, err = c.Exec(ctx, `
		INSERT INTO approval_assignments (id, request_id, step_order, user_node_id, grant_source, status, acted_at, comment)
		VALUES ($1, $2, $3, $4, $5, $6, COALESCE($7, NOW()), $8)`,
		a.ID, a.RequestID, a.StepOrder, a.UserNodeID, a.GrantSource, a.Status, a.ActedAt, a.Comment)
	var pg *pgconn.PgError
	if errors.As(err, &pg) && pg.Code == "23505" {
		return domain.ErrAlreadyExists
	}
	if err != nil {
		return fmt.Errorf("insert acted assignment: %w", err)
	}
	return nil
}

// ListAssignments returns every assignment of a request, step by step.
func (s *Store) ListAssignments(ctx context.Context, requestID string) ([]*domain.AssignmentRecord, error) {
	c, err := s.conn(ctx)
	if err != nil {
		return nil, err
	}
	defer c.Release()

	rows, err := c.Query(ctx, `
		SELECT id, request_id, step_order, user_node_id, grant_source, status, acted_at, COALESCE(comment, '')
		FROM approval_assignments
		WHERE request_id = $1
		ORDER BY step_order, acted_at NULLS LAST, id`, requestID)
	if err != nil {
		return nil, fmt.Errorf("list assignments: %w", err)
	}
	defer rows.Close()

	var out []*domain.AssignmentRecord
	for rows.Next() {
		a := &domain.AssignmentRecord{}
		if err := rows.Scan(&a.ID, &a.RequestID, &a.StepOrder, &a.UserNodeID, &a.GrantSource, &a.Status, &a.ActedAt, &a.Comment); err != nil {
			return nil, fmt.Errorf("scan assignment: %w", err)
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// UpdateAssignmentStatus updates an assignment's status and sets acted_at.
func (s *Store) UpdateAssignmentStatus(ctx context.Context, id, status, comment string) error {
	c, err := s.conn(ctx)
	if err != nil {
		return err
	}
	defer c.Release()

	_, err = c.Exec(ctx, `
		UPDATE approval_assignments SET status = $2, acted_at = NOW(), comment = $3
		WHERE id = $1`, id, status, comment,
	)
	if err != nil {
		return fmt.Errorf("update assignment: %w", err)
	}
	return nil
}

// CountApprovedForStep counts approved assignments for a specific step.
func (s *Store) CountApprovedForStep(ctx context.Context, requestID string, stepOrder int) (int, error) {
	c, err := s.conn(ctx)
	if err != nil {
		return 0, err
	}
	defer c.Release()

	var count int
	err = c.QueryRow(ctx, `
		SELECT COUNT(*) FROM approval_assignments
		WHERE request_id = $1 AND step_order = $2 AND status = 'approved'`,
		requestID, stepOrder,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count approved: %w", err)
	}
	return count, nil
}

// ListPendingAssignees returns the user nodes whose assignment on a step is
// still pending.
func (s *Store) ListPendingAssignees(ctx context.Context, requestID string, stepOrder int) ([]string, error) {
	c, err := s.conn(ctx)
	if err != nil {
		return nil, err
	}
	defer c.Release()

	rows, err := c.Query(ctx, `
		SELECT user_node_id FROM approval_assignments
		WHERE request_id = $1 AND step_order = $2 AND status = 'pending'
		ORDER BY user_node_id`,
		requestID, stepOrder,
	)
	if err != nil {
		return nil, fmt.Errorf("list pending assignees: %w", err)
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan pending assignee: %w", err)
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// SkipRemainingAssignments marks all pending assignments for a step as skipped.
func (s *Store) SkipRemainingAssignments(ctx context.Context, requestID string, stepOrder int) error {
	c, err := s.conn(ctx)
	if err != nil {
		return err
	}
	defer c.Release()

	_, err = c.Exec(ctx, `
		UPDATE approval_assignments SET status = 'skipped'
		WHERE request_id = $1 AND step_order = $2 AND status = 'pending'`,
		requestID, stepOrder,
	)
	if err != nil {
		return fmt.Errorf("skip remaining: %w", err)
	}
	return nil
}

// SkipAllPendingAssignments marks ALL pending assignments across all steps as skipped.
func (s *Store) SkipAllPendingAssignments(ctx context.Context, requestID string) error {
	c, err := s.conn(ctx)
	if err != nil {
		return err
	}
	defer c.Release()

	_, err = c.Exec(ctx, `
		UPDATE approval_assignments SET status = 'skipped'
		WHERE request_id = $1 AND status = 'pending'`, requestID,
	)
	if err != nil {
		return fmt.Errorf("skip all pending: %w", err)
	}
	return nil
}

// AdvanceStep increments the current_step on an approval request.
func (s *Store) AdvanceStep(ctx context.Context, requestID string, fromStep, nextStep int) (bool, error) {
	c, err := s.conn(ctx)
	if err != nil {
		return false, err
	}
	defer c.Release()

	// Compare-and-swap on current_step. Two approvals that satisfy the same
	// quorum concurrently both observe the count as met and both try to
	// advance; without the guard both succeed and each goes on to create a
	// full set of assignments for the next step. Only the caller that moves
	// the step away from fromStep may proceed.
	tag, err := c.Exec(ctx, `
		UPDATE approval_requests
		SET current_step = $3
		WHERE id = $1 AND current_step = $2 AND status = 'pending'`,
		requestID, fromStep, nextStep,
	)
	if err != nil {
		return false, fmt.Errorf("advance step: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

// CompleteRequest marks a request as completed with the given terminal status.
func (s *Store) CompleteRequest(ctx context.Context, requestID, status string) (bool, error) {
	c, err := s.conn(ctx)
	if err != nil {
		return false, err
	}
	defer c.Release()

	// Only a still-pending request may reach a terminal state, so a late
	// approval cannot overwrite a rejection (or vice versa) and the completion
	// audit entry is written exactly once.
	tag, err := c.Exec(ctx, `
		UPDATE approval_requests
		SET status = $2, completed_at = NOW()
		WHERE id = $1 AND status = 'pending'`,
		requestID, status,
	)
	if err != nil {
		return false, fmt.Errorf("complete request: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

// ListPending returns the user's pending assignments on their requests' current
// steps: rows of their own, and the group rows of the roles and departments in
// groupNodeIDs. A step the user has already acted on drops out even though its
// group row stays pending for the other members.
func (s *Store) ListPending(ctx context.Context, userNodeID string, groupNodeIDs []string) ([]*domain.RequestWithAssignment, error) {
	c, err := s.conn(ctx)
	if err != nil {
		return nil, err
	}
	defer c.Release()

	rows, err := c.Query(ctx, `
		SELECT ar.id, ar.entity_type, ar.entity_id, ar.status, ar.template_id, COALESCE(ar.template_name,''), ar.template_snapshot, COALESCE(ar.form_data_json::text,''),
		       ar.current_step, ar.scope_oa_id, ar.department_id, ar.created_by, ar.created_at, ar.completed_at,
		       aa.id, aa.step_order, aa.user_node_id, aa.grant_source, aa.status
		FROM approval_assignments aa
		JOIN approval_requests ar ON aa.request_id = ar.id
		WHERE aa.status = 'pending'
		  AND ar.status = 'pending'
		  AND ar.current_step = aa.step_order
		  AND (aa.user_node_id = $1
		       OR (aa.user_node_id = ANY($2)
		           AND aa.grant_source IN ('role:' || aa.user_node_id, 'department:' || aa.user_node_id)))
		  AND NOT EXISTS (
		        SELECT 1 FROM approval_assignments mine
		        WHERE mine.request_id = aa.request_id AND mine.step_order = aa.step_order
		          AND mine.user_node_id = $1 AND mine.id <> aa.id)
		ORDER BY ar.created_at ASC`, userNodeID, groupNodeIDs,
	)
	if err != nil {
		return nil, fmt.Errorf("list pending: %w", err)
	}
	defer rows.Close()

	return scanRequestsWithAssignment(rows)
}

// ListHistory returns acted-upon assignments with cursor-based paging.
func (s *Store) ListHistory(ctx context.Context, userNodeID, cursor string, limit int) ([]*domain.RequestWithAssignment, string, error) {
	c, err := s.conn(ctx)
	if err != nil {
		return nil, "", err
	}
	defer c.Release()

	query := `
		SELECT ar.id, ar.entity_type, ar.entity_id, ar.status, ar.template_id, COALESCE(ar.template_name,''), ar.template_snapshot, COALESCE(ar.form_data_json::text,''),
		       ar.current_step, ar.scope_oa_id, ar.department_id, ar.created_by, ar.created_at, ar.completed_at,
		       aa.id, aa.step_order, aa.user_node_id, aa.grant_source, aa.status, aa.acted_at, aa.comment
		FROM approval_assignments aa
		JOIN approval_requests ar ON aa.request_id = ar.id
		WHERE aa.user_node_id = $1
		  AND aa.status IN ('approved', 'rejected')`
	args := []any{userNodeID}
	argIdx := 2

	if cursor != "" {
		query += fmt.Sprintf(" AND aa.acted_at < $%d", argIdx)
		args = append(args, cursor)
		argIdx++
	}
	query += fmt.Sprintf(" ORDER BY aa.acted_at DESC LIMIT $%d", argIdx)
	args = append(args, limit+1)

	rows, err := c.Query(ctx, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("list history: %w", err)
	}
	defer rows.Close()

	var results []*domain.RequestWithAssignment
	for rows.Next() {
		r := &domain.Request{}
		a := &domain.AssignmentRecord{}
		if err := rows.Scan(
			&r.ID, &r.EntityType, &r.EntityID, &r.Status, &r.TemplateID, &r.TemplateName, &r.TemplateSnapshot, &r.FormDataJSON,
			&r.CurrentStep, &r.ScopeOAID, &r.DepartmentID, &r.CreatedBy, &r.CreatedAt, &r.CompletedAt,
			&a.ID, &a.StepOrder, &a.UserNodeID, &a.GrantSource, &a.Status, &a.ActedAt, &a.Comment,
		); err != nil {
			return nil, "", fmt.Errorf("scan history: %w", err)
		}
		a.RequestID = r.ID
		results = append(results, &domain.RequestWithAssignment{Request: r, Assignment: a})
	}

	var nextCursor string
	if len(results) > limit {
		last := results[limit]
		nextCursor = last.Assignment.ActedAt.Format("2006-01-02T15:04:05.999999Z07:00")
		results = results[:limit]
	}
	return results, nextCursor, nil
}

// ListMyRequests returns requests created by the user with cursor paging.
func (s *Store) ListMyRequests(ctx context.Context, userNodeID, cursor string, limit int) ([]*domain.Request, string, error) {
	c, err := s.conn(ctx)
	if err != nil {
		return nil, "", err
	}
	defer c.Release()

	query := `SELECT id, entity_type, entity_id, template_id, template_name, template_snapshot, COALESCE(form_data_json::text, ''),
		current_step, status, scope_oa_id, department_id, created_by, created_at, completed_at
		FROM approval_requests WHERE created_by = $1`
	args := []any{userNodeID}
	argIdx := 2

	if cursor != "" {
		query += fmt.Sprintf(" AND created_at < $%d", argIdx)
		args = append(args, cursor)
		argIdx++
	}
	query += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d", argIdx)
	args = append(args, limit+1)

	rows, err := c.Query(ctx, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("list my requests: %w", err)
	}
	defer rows.Close()

	results, nextCursor, err := scanRequests(rows, limit)
	if err != nil {
		return nil, "", err
	}
	return results, nextCursor, nil
}

// ListByScopes returns requests within the given scope OA IDs with cursor paging.
func (s *Store) ListByScopes(ctx context.Context, scopeOAIDs []string, cursor string, limit int) ([]*domain.Request, string, error) {
	c, err := s.conn(ctx)
	if err != nil {
		return nil, "", err
	}
	defer c.Release()

	query := `SELECT id, entity_type, entity_id, template_id, template_name, template_snapshot, COALESCE(form_data_json::text, ''),
		current_step, status, scope_oa_id, department_id, created_by, created_at, completed_at
		FROM approval_requests WHERE scope_oa_id = ANY($1)`
	args := []any{scopeOAIDs}
	argIdx := 2

	if cursor != "" {
		query += fmt.Sprintf(" AND created_at < $%d", argIdx)
		args = append(args, cursor)
		argIdx++
	}
	query += fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d", argIdx)
	args = append(args, limit+1)

	rows, err := c.Query(ctx, query, args...)
	if err != nil {
		return nil, "", fmt.Errorf("list by scopes: %w", err)
	}
	defer rows.Close()

	results, nextCursor, err := scanRequests(rows, limit)
	if err != nil {
		return nil, "", err
	}
	return results, nextCursor, nil
}

// InsertAuditEntry appends an audit record to the audit log.
func (s *Store) InsertAuditEntry(ctx context.Context, entry *domain.AuditEntry) error {
	c, err := s.conn(ctx)
	if err != nil {
		return err
	}
	defer c.Release()

	// INET and JSONB columns reject empty strings — send nil for NULL
	var ipAddr interface{}
	if entry.IPAddress != "" {
		ipAddr = entry.IPAddress
	}
	var detail interface{}
	if entry.DetailJSON != "" {
		detail = entry.DetailJSON
	}

	_, err = c.Exec(ctx, `
		INSERT INTO approval_audit_log (id, request_id, action, actor_node_id, step_order, detail, ip_address, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		entry.ID, entry.RequestID, entry.Action, entry.ActorNodeID, entry.StepOrder, detail, ipAddr, entry.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("insert audit: %w", err)
	}
	return nil
}

// ListAuditEntries retrieves all audit records for a request, ordered chronologically.
func (s *Store) ListAuditEntries(ctx context.Context, requestID string) ([]*domain.AuditEntry, error) {
	c, err := s.conn(ctx)
	if err != nil {
		return nil, err
	}
	defer c.Release()

	rows, err := c.Query(ctx, `
		SELECT id, request_id, action, actor_node_id, step_order, detail, ip_address, created_at
		FROM approval_audit_log WHERE request_id = $1 ORDER BY created_at ASC`, requestID,
	)
	if err != nil {
		return nil, fmt.Errorf("list audit: %w", err)
	}
	defer rows.Close()

	var entries []*domain.AuditEntry
	for rows.Next() {
		e := &domain.AuditEntry{}
		var detailJSON, ipAddress *string
		if err := rows.Scan(&e.ID, &e.RequestID, &e.Action, &e.ActorNodeID, &e.StepOrder, &detailJSON, &ipAddress, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan audit: %w", err)
		}
		if detailJSON != nil {
			e.DetailJSON = *detailJSON
		}
		if ipAddress != nil {
			e.IPAddress = *ipAddress
		}
		entries = append(entries, e)
	}
	return entries, nil
}

// --- reconciliation queries ---

// FindPendingByGrantSource finds all pending assignments whose grant_source
// matches the given pattern (e.g., "role:KeToan_Chief"). Used by the
// reconciliation consumer to find assignments affected by role changes.
func (s *Store) FindPendingByGrantSource(ctx context.Context, grantSourcePattern string) ([]*domain.AssignmentRecord, error) {
	c, err := s.conn(ctx)
	if err != nil {
		return nil, err
	}
	defer c.Release()

	rows, err := c.Query(ctx, `
		SELECT id, request_id, step_order, user_node_id, grant_source, status
		FROM approval_assignments
		WHERE status = 'pending' AND grant_source = $1`, grantSourcePattern,
	)
	if err != nil {
		return nil, fmt.Errorf("find pending by grant source: %w", err)
	}
	defer rows.Close()

	var results []*domain.AssignmentRecord
	for rows.Next() {
		a := &domain.AssignmentRecord{}
		if err := rows.Scan(&a.ID, &a.RequestID, &a.StepOrder, &a.UserNodeID, &a.GrantSource, &a.Status); err != nil {
			return nil, fmt.Errorf("scan assignment: %w", err)
		}
		results = append(results, a)
	}
	return results, nil
}

// FindPendingByUserAndSource finds a user's pending assignments matching
// a specific request ID or grant source pattern. Used to check duplicates
// during reconciliation and to find revocable assignments.
func (s *Store) FindPendingByUserAndSource(ctx context.Context, userNodeID, grantSourcePattern string) ([]*domain.AssignmentRecord, error) {
	c, err := s.conn(ctx)
	if err != nil {
		return nil, err
	}
	defer c.Release()

	rows, err := c.Query(ctx, `
		SELECT id, request_id, step_order, user_node_id, grant_source, status
		FROM approval_assignments
		WHERE user_node_id = $1 AND status = 'pending' AND grant_source = $2`,
		userNodeID, grantSourcePattern,
	)
	if err != nil {
		return nil, fmt.Errorf("find pending by user+source: %w", err)
	}
	defer rows.Close()

	var results []*domain.AssignmentRecord
	for rows.Next() {
		a := &domain.AssignmentRecord{}
		if err := rows.Scan(&a.ID, &a.RequestID, &a.StepOrder, &a.UserNodeID, &a.GrantSource, &a.Status); err != nil {
			return nil, fmt.Errorf("scan assignment: %w", err)
		}
		results = append(results, a)
	}
	return results, nil
}

// --- scan helpers ---

// scanRequestsWithAssignment scans rows from pending/history queries.
func scanRequestsWithAssignment(rows interface {
	Next() bool
	Scan(dest ...any) error
}) ([]*domain.RequestWithAssignment, error) {
	var results []*domain.RequestWithAssignment
	for rows.Next() {
		r := &domain.Request{}
		a := &domain.AssignmentRecord{}
		if err := rows.Scan(
			&r.ID, &r.EntityType, &r.EntityID, &r.Status, &r.TemplateID, &r.TemplateName, &r.TemplateSnapshot, &r.FormDataJSON,
			&r.CurrentStep, &r.ScopeOAID, &r.DepartmentID, &r.CreatedBy, &r.CreatedAt, &r.CompletedAt,
			&a.ID, &a.StepOrder, &a.UserNodeID, &a.GrantSource, &a.Status,
		); err != nil {
			return nil, fmt.Errorf("scan request+assignment: %w", err)
		}
		a.RequestID = r.ID
		results = append(results, &domain.RequestWithAssignment{Request: r, Assignment: a})
	}
	return results, nil
}

// scanRequests scans rows into Request slice with cursor extraction.
func scanRequests(rows interface {
	Next() bool
	Scan(dest ...any) error
}, limit int) ([]*domain.Request, string, error) {
	var results []*domain.Request
	for rows.Next() {
		r := &domain.Request{}
		if err := rows.Scan(&r.ID, &r.EntityType, &r.EntityID, &r.TemplateID, &r.TemplateName, &r.TemplateSnapshot, &r.FormDataJSON,
			&r.CurrentStep, &r.Status, &r.ScopeOAID, &r.DepartmentID, &r.CreatedBy, &r.CreatedAt, &r.CompletedAt,
		); err != nil {
			return nil, "", fmt.Errorf("scan request: %w", err)
		}
		results = append(results, r)
	}

	var nextCursor string
	if len(results) > limit {
		nextCursor = results[limit].CreatedAt.Format("2006-01-02T15:04:05.999999Z07:00")
		results = results[:limit]
	}
	return results, nextCursor, nil
}
