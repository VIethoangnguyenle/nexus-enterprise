package main

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"ngac-platform/ngac"
	"ngac-platform/pkg/policyclient"
	policypb "ngac-platform/proto/policy"
)

// db is the part of a pgx pool the backfill reads and writes rows through.
type db interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// group is a private channel as the backfill sees it. InvitePolicy is its
// "Ai được thêm người" rule (ngac.GroupSettingEveryone or GroupSettingManagers).
type group struct {
	ID, CreatedBy, WorkspaceID, ContentOA, MembersUA, InvitePolicy string
}

// backfiller gives existing groups an owner and a Managers UA. Rows are read and
// the owner column written through db; every graph change goes through the
// policy writer, so decisions are invalidated.
type backfiller struct {
	db     db
	write  policypb.PolicyWriteServiceClient
	dryRun bool
	out    io.Writer
}

type summary struct {
	Groups, OwnersKept, OwnersMoved, ManagersUAsMade, Failed int
}

// groups lists every private channel, oldest first.
func (b *backfiller) groups(ctx context.Context) ([]group, error) {
	rows, err := b.db.Query(ctx, `
		SELECT id, COALESCE(created_by, ''), COALESCE(workspace_id, ''), COALESCE(ngac_oa_id, ''), COALESCE(ngac_ua_id, ''),
		       COALESCE(invite_policy, '')
		FROM channels WHERE channel_type = 'private' ORDER BY created_at ASC, id ASC`)
	if err != nil {
		return nil, fmt.Errorf("list groups: %w", err)
	}
	defer rows.Close()
	var out []group
	for rows.Next() {
		var g group
		if err := rows.Scan(&g.ID, &g.CreatedBy, &g.WorkspaceID, &g.ContentOA, &g.MembersUA, &g.InvitePolicy); err != nil {
			return nil, fmt.Errorf("scan group: %w", err)
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

// chooseOwner applies the owner rule: the creator stays owner only while they
// are directly assigned to the group's Members UA in ngac_assignments; otherwise
// the member who joined earliest among those currently assigned becomes owner.
// A group with nobody assigned keeps its creator column and gets no owner node.
func (b *backfiller) chooseOwner(ctx context.Context, g group) (userID, nodeID string, err error) {
	if g.MembersUA == "" {
		return g.CreatedBy, "", nil
	}
	if g.CreatedBy != "" {
		err := b.db.QueryRow(ctx, `
			SELECT u.ngac_node FROM users u
			JOIN ngac_assignments a ON a.child_id = u.ngac_node AND a.parent_id = $2
			WHERE u.id = $1`, g.CreatedBy, g.MembersUA).Scan(&nodeID)
		switch {
		case err == nil:
			return g.CreatedBy, nodeID, nil
		case !errors.Is(err, pgx.ErrNoRows):
			return "", "", fmt.Errorf("creator membership: %w", err)
		}
	}
	err = b.db.QueryRow(ctx, `
		SELECT u.id, u.ngac_node FROM ngac_assignments a
		JOIN users u ON u.ngac_node = a.child_id
		LEFT JOIN channel_members cm ON cm.channel_id = $1 AND cm.ngac_node_id = a.child_id
		WHERE a.parent_id = $2
		ORDER BY cm.joined_at ASC NULLS LAST, u.created_at ASC NULLS LAST, u.id ASC
		LIMIT 1`, g.ID, g.MembersUA).Scan(&userID, &nodeID)
	if errors.Is(err, pgx.ErrNoRows) {
		return g.CreatedBy, "", nil
	}
	if err != nil {
		return "", "", fmt.Errorf("earliest member: %w", err)
	}
	return userID, nodeID, nil
}

// run backfills every group. A group that fails is reported and counted, and
// the rest still run; rerunning is safe, since every step is idempotent.
func (b *backfiller) run(ctx context.Context) (summary, error) {
	groups, err := b.groups(ctx)
	if err != nil {
		return summary{}, err
	}
	s := summary{Groups: len(groups)}
	for _, g := range groups {
		if err := b.backfill(ctx, g, &s); err != nil {
			s.Failed++
			fmt.Fprintf(b.out, "  group %s: FAILED: %v\n", g.ID, err)
		}
	}
	return s, nil
}

func (b *backfiller) backfill(ctx context.Context, g group, s *summary) error {
	ownerID, ownerNode, err := b.chooseOwner(ctx, g)
	if err != nil {
		return err
	}
	if ownerID != g.CreatedBy {
		s.OwnersMoved++
		fmt.Fprintf(b.out, "  group %s: owner %s -> %s (creator is no longer a member)\n", g.ID, g.CreatedBy, ownerID)
		if !b.dryRun {
			if _, err := b.db.Exec(ctx, `UPDATE channels SET created_by = $2 WHERE id = $1`, g.ID, ownerID); err != nil {
				return fmt.Errorf("move owner: %w", err)
			}
		}
	} else {
		s.OwnersKept++
		fmt.Fprintf(b.out, "  group %s: owner %s kept\n", g.ID, ownerID)
	}
	if g.WorkspaceID == "" {
		return nil
	}
	return b.ensureManagers(ctx, g, ownerNode, s)
}

// ensureManagers finds the group's Managers UA by its name, creating it when it
// is missing, and then writes every edge the group's roles need: the UA under
// the workspace PC, the managers' grant on the content OA, the members' grant
// on it as the group's invite rule says, and the owner in the UA.
//
// The edges are written on every run, whether the UA was just made or found.
// A run that made the UA and then failed left it without some of them, and a
// rerun that skipped a found UA would leave it so for good. Each write is an
// upsert, so writing an edge that exists changes nothing.
func (b *backfiller) ensureManagers(ctx context.Context, g group, ownerNode string, s *summary) error {
	name := ngac.ChannelManagersUAName(ngac.ChannelID(g.ID))
	managersUA := ""
	n, err := b.write.FindNodeByName(ctx, &policypb.FindNodeByNameRequest{Name: name, NodeType: ngac.TypeUA})
	switch {
	case err == nil:
		managersUA = n.GetId()
	case !policyclient.IsNotFound(err):
		return fmt.Errorf("find managers UA: %w", err)
	}
	if managersUA == "" {
		s.ManagersUAsMade++
		fmt.Fprintf(b.out, "  group %s: creating managers UA %s\n", g.ID, name)
	}
	if b.dryRun {
		return nil
	}
	if g.ContentOA == "" {
		return fmt.Errorf("group has no content OA")
	}
	var pc string
	if err := b.db.QueryRow(ctx, `SELECT COALESCE(ngac_pc_id, '') FROM workspaces WHERE id = $1`, g.WorkspaceID).Scan(&pc); err != nil {
		return fmt.Errorf("workspace PC: %w", err)
	}
	if pc == "" {
		return fmt.Errorf("workspace has no PC")
	}
	if managersUA == "" {
		created, err := b.write.CreateNode(ctx, &policypb.CreateNodeRequest{Name: name, NodeType: ngac.TypeUA})
		if err != nil {
			return fmt.Errorf("create managers UA: %w", err)
		}
		managersUA = created.GetId()
	}
	if _, err := b.write.CreateAssignment(ctx, &policypb.CreateAssignmentRequest{ChildId: managersUA, ParentId: pc}); err != nil {
		return fmt.Errorf("assign managers UA under PC: %w", err)
	}
	if _, err := b.write.CreateAssociation(ctx, &policypb.CreateAssociationRequest{
		UaId: managersUA, OaId: g.ContentOA, Operations: ngac.ChannelManagerOps(),
	}); err != nil {
		return fmt.Errorf("grant managers on content: %w", err)
	}
	if g.MembersUA != "" {
		if _, err := b.write.CreateAssociation(ctx, &policypb.CreateAssociationRequest{
			UaId: g.MembersUA, OaId: g.ContentOA, Operations: memberOps(g.InvitePolicy),
		}); err != nil {
			return fmt.Errorf("grant members on content: %w", err)
		}
	}
	if ownerNode == "" {
		return nil
	}
	if _, err := b.write.CreateAssignment(ctx, &policypb.CreateAssignmentRequest{ChildId: ownerNode, ParentId: managersUA}); err != nil {
		return fmt.Errorf("add owner to managers: %w", err)
	}
	return nil
}

// memberOps is the members' grant on a group's content under its invite rule:
// without invite when only managers may add people, the full member
// operations otherwise (the rule's default).
func memberOps(invitePolicy string) []string {
	if invitePolicy == ngac.GroupSettingManagers {
		return ngac.ChannelMemberOpsRestricted()
	}
	return ngac.ChannelMemberOps()
}
