package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// Workspace is what the calendar needs to know about a workspace.
type Workspace struct {
	ID   string
	Name string
	// Type is "personal" or "organization".
	Type string
	// PCNodeID is the workspace's policy class: a person belongs to the workspace
	// exactly when it is among their ancestors.
	PCNodeID string
}

// Workspace reads one workspace. A workspace that does not exist is ErrNotFound.
func (s *Store) Workspace(ctx context.Context, id string) (*Workspace, error) {
	w := &Workspace{}
	err := s.pool.QueryRow(ctx,
		`SELECT id, name, type, COALESCE(ngac_pc_id, '') FROM workspaces WHERE id = $1`, id,
	).Scan(&w.ID, &w.Name, &w.Type, &w.PCNodeID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("workspace: %w", ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("workspace: %w", err)
	}
	return w, nil
}

// Person is someone of a workspace, as a screen shows them. NodeID is the key a
// picker sends back; no screen displays it.
type Person struct {
	NodeID      string `json:"nodeId"`
	DisplayName string `json:"displayName"`
	AvatarURL   string `json:"avatarUrl,omitempty"`
}

// personColumns reads a person from tu (tenant_users) joined to u (users): the
// display name, else the login, so a name is never empty.
const personColumns = `tu.ngac_node_id, COALESCE(NULLIF(u.display_name, ''), u.username), COALESCE(u.avatar_url, '')`

func scanPerson(row interface{ Scan(...any) error }) (*Person, error) {
	p := &Person{}
	if err := row.Scan(&p.NodeID, &p.DisplayName, &p.AvatarURL); err != nil {
		return nil, err
	}
	return p, nil
}

// PersonByNode reads an active member of the workspace by their NGAC user node.
// Someone who is not a member, or not active, is ErrNotFound: the name of an
// account is read only through its membership of the workspace asked about.
func (s *Store) PersonByNode(ctx context.Context, workspaceID, nodeID string) (*Person, error) {
	p, err := scanPerson(s.pool.QueryRow(ctx,
		`SELECT `+personColumns+`
		   FROM tenant_users tu JOIN users u ON u.id = tu.user_id
		  WHERE tu.tenant_id = $1 AND tu.ngac_node_id = $2 AND tu.status = 'active'`,
		workspaceID, nodeID))
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("person: %w", ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("person: %w", err)
	}
	return p, nil
}

// People reads the active members of the workspace among the given nodes, keyed
// by node. A node that is not a member is absent from the result.
func (s *Store) People(ctx context.Context, workspaceID string, nodeIDs []string) (map[string]*Person, error) {
	out := map[string]*Person{}
	if len(nodeIDs) == 0 {
		return out, nil
	}
	rows, err := s.pool.Query(ctx,
		`SELECT `+personColumns+`
		   FROM tenant_users tu JOIN users u ON u.id = tu.user_id
		  WHERE tu.tenant_id = $1 AND tu.ngac_node_id = ANY($2) AND tu.status = 'active'`,
		workspaceID, nodeIDs)
	if err != nil {
		return nil, fmt.Errorf("people: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		p, err := scanPerson(rows)
		if err != nil {
			return nil, fmt.Errorf("people: %w", err)
		}
		out[p.NodeID] = p
	}
	return out, rows.Err()
}

// Channel is a conversation, as far as the calendar cares.
type Channel struct {
	ID          string
	WorkspaceID string
	// Type is "workspace", "private" or "dm".
	Type string
	Name string
	// ContentOAID is the OA the conversation's content is authorized on; a group
	// calendar's OA is assigned beneath it.
	ContentOAID string
}

// Channel reads one conversation. One that does not exist is ErrNotFound.
func (s *Store) Channel(ctx context.Context, id string) (*Channel, error) {
	c := &Channel{}
	err := s.pool.QueryRow(ctx,
		`SELECT id, COALESCE(workspace_id, ''), channel_type, name, COALESCE(ngac_oa_id, '')
		   FROM channels WHERE id = $1`, id,
	).Scan(&c.ID, &c.WorkspaceID, &c.Type, &c.Name, &c.ContentOAID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("channel: %w", ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("channel: %w", err)
	}
	return c, nil
}
