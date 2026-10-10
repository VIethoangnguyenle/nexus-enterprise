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

	"ngac-platform/services/approval/internal/domain"
)

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
