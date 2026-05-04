package store

import (
	"context"
	"fmt"
)

// InsertDepartment persists a new department row.
func (s *Store) InsertDepartment(ctx context.Context, d *Department) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO departments (id, workspace_id, name, parent_id, ngac_ua_id, sort_order)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		d.ID, d.WorkspaceID, d.Name, d.ParentID, d.NGACUaID, d.SortOrder,
	)
	if err != nil {
		return fmt.Errorf("insert department: %w", err)
	}
	return nil
}

// ListDepartmentsByWorkspace returns all departments for a workspace, ordered by sort_order.
func (s *Store) ListDepartmentsByWorkspace(ctx context.Context, wsID string) ([]*Department, error) {
	rows, err := s.db.Query(ctx,
		`SELECT id, workspace_id, name, parent_id, ngac_ua_id, sort_order, created_at, updated_at
		 FROM departments WHERE workspace_id = $1 ORDER BY sort_order, name`, wsID,
	)
	if err != nil {
		return nil, fmt.Errorf("list departments: %w", err)
	}
	defer rows.Close()

	var result []*Department
	for rows.Next() {
		var d Department
		if err := rows.Scan(&d.ID, &d.WorkspaceID, &d.Name, &d.ParentID, &d.NGACUaID, &d.SortOrder, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan department: %w", err)
		}
		result = append(result, &d)
	}
	return result, nil
}

// GetDepartment returns a single department by ID.
func (s *Store) GetDepartment(ctx context.Context, id string) (*Department, error) {
	var d Department
	err := s.db.QueryRow(ctx,
		`SELECT id, workspace_id, name, parent_id, ngac_ua_id, sort_order, created_at, updated_at
		 FROM departments WHERE id = $1`, id,
	).Scan(&d.ID, &d.WorkspaceID, &d.Name, &d.ParentID, &d.NGACUaID, &d.SortOrder, &d.CreatedAt, &d.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("get department %s: %w", id, err)
	}
	return &d, nil
}

// UpdateDepartmentName renames a department.
func (s *Store) UpdateDepartmentName(ctx context.Context, id, name string) error {
	_, err := s.db.Exec(ctx,
		`UPDATE departments SET name = $1, updated_at = NOW() WHERE id = $2`, name, id,
	)
	if err != nil {
		return fmt.Errorf("update department name: %w", err)
	}
	return nil
}

// MoveDepartment changes a department's parent.
func (s *Store) MoveDepartment(ctx context.Context, id string, newParentID *string) error {
	_, err := s.db.Exec(ctx,
		`UPDATE departments SET parent_id = $1, updated_at = NOW() WHERE id = $2`, newParentID, id,
	)
	if err != nil {
		return fmt.Errorf("move department: %w", err)
	}
	return nil
}

// DeleteDepartment removes a department row.
func (s *Store) DeleteDepartment(ctx context.Context, id string) error {
	_, err := s.db.Exec(ctx, `DELETE FROM departments WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete department: %w", err)
	}
	return nil
}

// UpdateUserDepartment sets a user's department.
func (s *Store) UpdateUserDepartment(ctx context.Context, userID string, deptID *string) error {
	_, err := s.db.Exec(ctx,
		`UPDATE tenant_users SET department_id = $1 WHERE user_id = $2`, deptID, userID,
	)
	if err != nil {
		return fmt.Errorf("update user department: %w", err)
	}
	return nil
}

// CountMembersByDepartment returns the number of users assigned to a department.
func (s *Store) CountMembersByDepartment(ctx context.Context, deptID string) (int, error) {
	var count int
	err := s.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM tenant_users WHERE department_id = $1`, deptID,
	).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count department members: %w", err)
	}
	return count, nil
}

// ReassignDepartmentChildren moves all child departments to a new parent.
func (s *Store) ReassignDepartmentChildren(ctx context.Context, oldParentID string, newParentID *string) error {
	_, err := s.db.Exec(ctx,
		`UPDATE departments SET parent_id = $1, updated_at = NOW() WHERE parent_id = $2`, newParentID, oldParentID,
	)
	if err != nil {
		return fmt.Errorf("reassign children: %w", err)
	}
	return nil
}

// ReassignDepartmentUsers moves all users from one department to another (or NULL).
func (s *Store) ReassignDepartmentUsers(ctx context.Context, oldDeptID string, newDeptID *string) error {
	_, err := s.db.Exec(ctx,
		`UPDATE tenant_users SET department_id = $1 WHERE department_id = $2`, newDeptID, oldDeptID,
	)
	if err != nil {
		return fmt.Errorf("reassign users: %w", err)
	}
	return nil
}
