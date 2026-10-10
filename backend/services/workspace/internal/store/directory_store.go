package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// Profile is what a screen shows of a person: names and a picture, never an
// identifier to read. UserID is a stable colour key for the avatar; the node ID
// is how the graph knows the person.
type Profile struct {
	UserID      string
	NodeID      string
	Username    string
	DisplayName string
	Email       string
	AvatarURL   string
	Title       string
	// Status is the person's standing in this workspace: active, invited or
	// disabled. A person the graph admits but tenant_users does not list is active.
	Status string
}

const profileColumns = `u.id, COALESCE(u.ngac_node,''), u.username, COALESCE(u.display_name,''),
	COALESCE(u.email,''), COALESCE(u.avatar_url,''), COALESCE(u.title,'')`

func scanProfile(row pgx.Row, withStatus bool) (*Profile, error) {
	var p Profile
	dest := []any{&p.UserID, &p.NodeID, &p.Username, &p.DisplayName, &p.Email, &p.AvatarURL, &p.Title}
	if withStatus {
		dest = append(dest, &p.Status)
	}
	if err := row.Scan(dest...); err != nil {
		return nil, err
	}
	if p.Status == "" {
		p.Status = "active"
	}
	return &p, nil
}

// ProfilesByNodeIDs returns the profile of every given user node that has a
// users row, keyed by node ID. A node with no row is simply absent; the caller
// falls back to the name on the node.
func (s *Store) ProfilesByNodeIDs(ctx context.Context, tenantID string, nodeIDs []string) (map[string]*Profile, error) {
	out := make(map[string]*Profile, len(nodeIDs))
	if len(nodeIDs) == 0 {
		return out, nil
	}
	rows, err := s.db.Query(ctx,
		`SELECT `+profileColumns+`, COALESCE(tu.status, 'active')
		   FROM users u
		   LEFT JOIN tenant_users tu ON tu.user_id = u.id AND tu.tenant_id = $1
		  WHERE u.ngac_node = ANY($2)`, tenantID, nodeIDs)
	if err != nil {
		return nil, fmt.Errorf("profiles by node: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		p, err := scanProfile(rows, true)
		if err != nil {
			return nil, fmt.Errorf("scan profile: %w", err)
		}
		out[p.NodeID] = p
	}
	return out, rows.Err()
}

// EnsureTenantUser lists a person as a member of the workspace, so contacts and
// names elsewhere (approval, chat) can find them. An existing row is left as it
// is: a person an administrator disabled does not come back by being invited.
func (s *Store) EnsureTenantUser(ctx context.Context, tenantID, userID, nodeID string) error {
	_, err := s.db.Exec(ctx,
		`INSERT INTO tenant_users (tenant_id, user_id, role, status, ngac_node_id)
		 VALUES ($1, $2, 'member', 'active', $3)
		 ON CONFLICT (tenant_id, user_id)
		 DO UPDATE SET ngac_node_id = COALESCE(tenant_users.ngac_node_id, EXCLUDED.ngac_node_id)`,
		tenantID, userID, nodeID)
	if err != nil {
		return fmt.Errorf("ensure tenant user: %w", err)
	}
	return nil
}

// RemoveTenantUser drops a person's listing in the workspace.
func (s *Store) RemoveTenantUser(ctx context.Context, tenantID, nodeID string) error {
	_, err := s.db.Exec(ctx,
		`DELETE FROM tenant_users tu USING users u
		  WHERE tu.user_id = u.id AND tu.tenant_id = $1
		    AND (tu.ngac_node_id = $2 OR u.ngac_node = $2)`, tenantID, nodeID)
	if err != nil {
		return fmt.Errorf("remove tenant user: %w", err)
	}
	return nil
}
