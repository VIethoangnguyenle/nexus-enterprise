package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"ngac-platform/ngac"
	"ngac-platform/services/approval/internal/domain"
)

// DisplayNames maps keys that appear on approval records to what a person
// would call them, within one tenant:
//
//   - a user's NGAC node id (requester, approver, actor) or user id → display
//     name, falling back to the username — only for people who belong to the
//     tenant (tenant_users), matched by tenant_users.ngac_node_id or users.ngac_node;
//   - a role's or department's UA node id → the UA's display_name property,
//     falling back to its name — only a UA that sits under the tenant's PC or is
//     a department of the tenant's workspace;
//   - a department's id → the department's name, only in this workspace.
//
// A key that is none of these, or belongs to another tenant, is left out, so
// the caller shows a neutral word rather than an id and one tenant never reads
// another's names. It runs on the shared schema, not a tenant's: users, roles
// and departments live there.
func (s *Store) DisplayNames(ctx context.Context, tenantID string, keys []string) (map[string]string, error) {
	names := make(map[string]string)
	if len(keys) == 0 || tenantID == "" {
		return names, nil
	}
	pcName := ngac.PCName(ngac.WorkspaceID(tenantID))
	// Ordered by precedence: a person wins over a role or department that
	// happens to share a key, and the first row seen for a key is kept.
	rows, err := s.db.Query(ctx, `
		WITH RECURSIVE
		cand AS (
			SELECT id FROM ngac_nodes WHERE node_type = 'UA' AND id = ANY($3)
		),
		up(root, id) AS (
			SELECT id, id FROM cand
			UNION
			SELECT up.root, a.parent_id FROM ngac_assignments a JOIN up ON a.child_id = up.id
		),
		in_tenant AS (
			SELECT DISTINCT up.root AS id
			FROM up JOIN ngac_nodes p ON p.id = up.id
			WHERE p.node_type = 'PC' AND p.name = $2
		)
		SELECT key, name FROM (
			SELECT tu.ngac_node_id AS key, COALESCE(NULLIF(u.display_name, ''), u.username) AS name, 1 AS rank
			  FROM tenant_users tu JOIN users u ON u.id = tu.user_id
			 WHERE tu.tenant_id = $1 AND tu.ngac_node_id = ANY($3)
			UNION ALL
			SELECT u.ngac_node, COALESCE(NULLIF(u.display_name, ''), u.username), 1
			  FROM tenant_users tu JOIN users u ON u.id = tu.user_id
			 WHERE tu.tenant_id = $1 AND u.ngac_node = ANY($3)
			UNION ALL
			SELECT u.id, COALESCE(NULLIF(u.display_name, ''), u.username), 2
			  FROM tenant_users tu JOIN users u ON u.id = tu.user_id
			 WHERE tu.tenant_id = $1 AND u.id = ANY($3)
			UNION ALL
			SELECT id, name, 3 FROM departments WHERE workspace_id = $1 AND id = ANY($3)
			UNION ALL
			SELECT ngac_ua_id, name, 3 FROM departments WHERE workspace_id = $1 AND ngac_ua_id = ANY($3)
			UNION ALL
			SELECT n.id, COALESCE(NULLIF(n.properties->>$4, ''), n.name), 4
			  FROM ngac_nodes n JOIN in_tenant t ON t.id = n.id
		) n ORDER BY rank`, tenantID, pcName, keys, ngac.PropDisplayName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var key, name string
		if err := rows.Scan(&key, &name); err != nil {
			return nil, err
		}
		if _, seen := names[key]; !seen {
			names[key] = name
		}
	}
	return names, rows.Err()
}

// FindNodeID returns the id of the node with this exact name and type.
func (s *Store) FindNodeID(ctx context.Context, name, nodeType string) (string, error) {
	var id string
	err := s.db.QueryRow(ctx, `SELECT id FROM ngac_nodes WHERE name = $1 AND node_type = $2 LIMIT 1`, name, nodeType).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", domain.ErrNotFound
	}
	if err != nil {
		return "", err
	}
	return id, nil
}

// CanonicalApprover checks that the approver a template step names exists in
// this tenant and returns the key the step stores: a person's user node, a
// role's UA, or a department's UA (a department id becomes its UA). Anything
// else — an id of another tenant, a node of the wrong type — is ErrInvalidInput.
func (s *Store) CanonicalApprover(ctx context.Context, tenantID, approverType, value string) (string, error) {
	invalid := fmt.Errorf("%q is not a %s of this tenant: %w", "approver", approverType, domain.ErrInvalidInput)
	if value == "" {
		return "", invalid
	}
	switch approverType {
	case "specific_user":
		var ok bool
		err := s.db.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM tenant_users tu JOIN users u ON u.id = tu.user_id
				WHERE tu.tenant_id = $1 AND (tu.ngac_node_id = $2 OR u.ngac_node = $2))`,
			tenantID, value).Scan(&ok)
		if err != nil {
			return "", err
		}
		if !ok {
			return "", invalid
		}
		return value, nil
	case "role_in_dept", "department":
		// A department may be named by its id (what the departments list gives).
		var ua string
		err := s.db.QueryRow(ctx, `SELECT ngac_ua_id FROM departments WHERE workspace_id = $1 AND id = $2`, tenantID, value).Scan(&ua)
		if err == nil {
			value = ua
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return "", err
		}
		var ok bool
		err = s.db.QueryRow(ctx, `
			WITH RECURSIVE up(id) AS (
				SELECT id FROM ngac_nodes WHERE id = $2 AND node_type = 'UA'
				UNION
				SELECT a.parent_id FROM ngac_assignments a JOIN up ON a.child_id = up.id
			)
			SELECT EXISTS (SELECT 1 FROM up JOIN ngac_nodes p ON p.id = up.id WHERE p.node_type = 'PC' AND p.name = $3)
			    OR EXISTS (SELECT 1 FROM departments WHERE workspace_id = $1 AND ngac_ua_id = $2)`,
			tenantID, value, ngac.PCName(ngac.WorkspaceID(tenantID))).Scan(&ok)
		if err != nil {
			return "", err
		}
		if !ok {
			return "", invalid
		}
		return value, nil
	}
	return "", invalid
}
