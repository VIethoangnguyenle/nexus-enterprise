package store

import (
	"context"
	"fmt"
	"strings"

	"ngac-platform/services/approval/internal/domain"
)

// SearchRequests finds requests matching a text query with access filtering.
// The text search matches case-insensitively on template_name and the
// requester's display name (joined from public.users). Access is filtered
// in SQL: the caller's own requests, requests they are assigned to (directly
// or through a group), and requests whose scope the caller can read.
func (s *Store) SearchRequests(ctx context.Context, p domain.SearchParams) ([]*domain.Request, int, error) {
	c, err := s.conn(ctx)
	if err != nil {
		return nil, 0, err
	}
	defer c.Release()

	// Build the WHERE clause dynamically.
	var conditions []string
	var args []any
	argIdx := 1

	// Text search: match on template_name or requester display name.
	// Uses lower() for case-insensitive matching. The unaccent extension
	// would be better but may not be available in all environments. % and _ in
	// the query are the characters themselves, not wildcards.
	textParam := fmt.Sprintf("$%d", argIdx)
	conditions = append(conditions, fmt.Sprintf(
		`(lower(ar.template_name) LIKE '%%' || lower(%s) || '%%' ESCAPE '\' OR lower(u.display_name) LIKE '%%' || lower(%s) || '%%' ESCAPE '\')`,
		textParam, textParam,
	))
	args = append(args, escapeLike(p.Query))
	argIdx++

	// Access control: the caller can see requests they created, are assigned
	// to (directly or through groups), or whose scope they can read.
	var accessParts []string
	accessParts = append(accessParts, fmt.Sprintf("ar.created_by = $%d", argIdx))
	args = append(args, p.UserNodeID)
	argIdx++

	allNodeIDs := append([]string{p.UserNodeID}, p.GroupNodeIDs...)
	accessParts = append(accessParts, fmt.Sprintf(
		"EXISTS (SELECT 1 FROM approval_assignments WHERE request_id = ar.id AND user_node_id = ANY($%d))",
		argIdx,
	))
	args = append(args, allNodeIDs)
	argIdx++

	if len(p.AccessibleScopes) > 0 {
		accessParts = append(accessParts, fmt.Sprintf("ar.scope_oa_id = ANY($%d)", argIdx))
		args = append(args, p.AccessibleScopes)
		argIdx++
	}

	conditions = append(conditions, "("+strings.Join(accessParts, " OR ")+")")

	// Optional filters.
	if p.Status != "" {
		conditions = append(conditions, fmt.Sprintf("ar.status = $%d", argIdx))
		args = append(args, p.Status)
		argIdx++
	}
	if p.TemplateID != "" {
		conditions = append(conditions, fmt.Sprintf("ar.template_id = $%d", argIdx))
		args = append(args, p.TemplateID)
		argIdx++
	}
	if p.CreatedBy != "" {
		// The filter names the requester by user ID; a request records their node.
		conditions = append(conditions, fmt.Sprintf("ar.created_by = (SELECT ngac_node FROM public.users WHERE id = $%d)", argIdx))
		args = append(args, p.CreatedBy)
		argIdx++
	}
	if p.From != "" {
		conditions = append(conditions, fmt.Sprintf("ar.created_at >= $%d::date", argIdx))
		args = append(args, p.From)
		argIdx++
	}
	if p.To != "" {
		conditions = append(conditions, fmt.Sprintf("ar.created_at < ($%d::date + interval '1 day')", argIdx))
		args = append(args, p.To)
		argIdx++
	}

	where := "WHERE " + strings.Join(conditions, " AND ")

	// Count total matching rows.
	countQuery := `SELECT count(*) FROM approval_requests ar
		LEFT JOIN public.users u ON u.ngac_node = ar.created_by ` + where

	var total int
	if err := c.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("search count: %w", err)
	}

	if total == 0 {
		return nil, 0, nil
	}

	// Fetch the limited result set.
	dataQuery := fmt.Sprintf(`SELECT ar.id, ar.entity_type, ar.entity_id, ar.template_id, ar.template_name,
		ar.template_snapshot, COALESCE(ar.form_data_json::text, ''),
		ar.current_step, ar.status, ar.scope_oa_id, ar.department_id, ar.created_by, ar.created_at, ar.completed_at
		FROM approval_requests ar
		LEFT JOIN public.users u ON u.ngac_node = ar.created_by
		%s
		ORDER BY ar.created_at DESC, ar.id DESC
		LIMIT $%d`, where, argIdx)
	args = append(args, p.Limit)

	rows, err := c.Query(ctx, dataQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("search requests: %w", err)
	}
	defer rows.Close()

	var results []*domain.Request
	for rows.Next() {
		r := &domain.Request{}
		if err := rows.Scan(
			&r.ID, &r.EntityType, &r.EntityID, &r.TemplateID, &r.TemplateName,
			&r.TemplateSnapshot, &r.FormDataJSON,
			&r.CurrentStep, &r.Status, &r.ScopeOAID, &r.DepartmentID, &r.CreatedBy, &r.CreatedAt, &r.CompletedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("scan search result: %w", err)
		}
		results = append(results, r)
	}
	return results, total, nil
}

// escapeLike makes LIKE's wildcards (% and _) and its escape character match
// themselves.
func escapeLike(s string) string {
	var out []byte
	for i := range len(s) {
		switch s[i] {
		case '%', '_', '\\':
			out = append(out, '\\')
		}
		out = append(out, s[i])
	}
	return string(out)
}
