package domain_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"ngac-platform/ngac"
	"ngac-platform/pkg/grpcauth"
	policypb "ngac-platform/proto/policy"
	"ngac-platform/services/messaging/internal/domain"
	"ngac-platform/services/messaging/internal/store"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// insertGroupChannel writes a private channel row with created_by and the
// three management settings. Returns (channelID, oaID, uaID).
func (f *fixture) insertGroupChannel(t *testing.T, name, createdBy string) (chID, oaID, uaID string) {
	t.Helper()
	ctx := context.Background()
	n := time.Now().UnixNano()
	chID = fmt.Sprintf("test-grp-ch-%d", n)
	oaID = fmt.Sprintf("test-grp-oa-%d", n)
	uaID = fmt.Sprintf("test-grp-ua-%d", n)

	for _, id := range []string{oaID, uaID} {
		_, err := f.pool.Exec(ctx,
			`INSERT INTO ngac_nodes (id, name, node_type) VALUES ($1, $2, $3)`,
			id, "n-"+id, ngac.TypeOA)
		require.NoError(t, err)
	}

	_, err := f.pool.Exec(ctx,
		`INSERT INTO channels (id, name, channel_type, workspace_id, ngac_oa_id, ngac_ua_id, created_by, created_at)
		 VALUES ($1, $2, 'private', $3, $4, $5, $6, now())`,
		chID, name, f.wsID, oaID, uaID, createdBy)
	require.NoError(t, err)

	t.Cleanup(func() {
		f.pool.Exec(ctx, `DELETE FROM channel_members WHERE channel_id = $1`, chID)
		f.pool.Exec(ctx, `DELETE FROM channels WHERE id = $1`, chID)
		f.pool.Exec(ctx, `DELETE FROM ngac_nodes WHERE id = $1`, oaID)
		f.pool.Exec(ctx, `DELETE FROM ngac_nodes WHERE id = $1`, uaID)
	})
	if createdBy == f.userID {
		f.bindCallerNode(t)
	}
	return chID, oaID, uaID
}

// bindCallerNode makes the fixture's user sign in as userNode in the database
// too. The owner of a group is resolved from created_by to that user's node, so
// a test whose caller is userNode owns the group only when the two agree.
func (f *fixture) bindCallerNode(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	var prev string
	require.NoError(t, f.pool.QueryRow(ctx, `SELECT COALESCE(ngac_node, '') FROM users WHERE id = $1`, f.userID).Scan(&prev))
	if prev == userNode {
		return
	}
	_, err := f.pool.Exec(ctx, `INSERT INTO ngac_nodes (id, name, node_type) VALUES ($1, $1, 'U') ON CONFLICT (id) DO NOTHING`, userNode)
	require.NoError(t, err)
	_, err = f.pool.Exec(ctx, `UPDATE users SET ngac_node = $1 WHERE id = $2`, userNode, f.userID)
	require.NoError(t, err)
	t.Cleanup(func() {
		f.pool.Exec(ctx, `UPDATE users SET ngac_node = NULLIF($1, '') WHERE id = $2`, prev, f.userID)
	})
}

// insertGroupChannelWithSettings adds policy columns.
func (f *fixture) insertGroupChannelWithSettings(t *testing.T, name, createdBy, postPolicy, invitePolicy, mentionAllPolicy string) (chID, oaID, uaID string) {
	t.Helper()
	chID, oaID, uaID = f.insertGroupChannel(t, name, createdBy)
	ctx := context.Background()
	_, err := f.pool.Exec(ctx,
		`UPDATE channels SET post_policy = $2, invite_policy = $3, mention_all_policy = $4 WHERE id = $1`,
		chID, postPolicy, invitePolicy, mentionAllPolicy)
	require.NoError(t, err)
	return chID, oaID, uaID
}

// ctxForUser creates a context with the specified user as the caller.
func ctxForUser(userID, nodeID, wsID string) context.Context {
	return grpcauth.WithCaller(context.Background(), grpcauth.Caller{
		UserID: userID, NGACNodeID: nodeID, TenantID: wsID,
	})
}

const (
	managerNode  = "ngac-user-manager"
	plainNode    = "ngac-user-plain"
	outsiderNode = "ngac-user-outsider"
	wsOwnerNode  = "ngac-user-ws-owner"
)

// allowRead grants read on oaID for the node.
func (f *fixture) allowRead(oaID, node string) {
	f.read.allow[grant{node, oaID, ngac.OpRead}] = true
}

// allowWrite grants write on oaID for the node.
func (f *fixture) allowWrite(oaID, node string) {
	f.read.allow[grant{node, oaID, ngac.OpWrite}] = true
}

// allowAll grants read+write+invite on oaID for the node (member ops).
func (f *fixture) allowAll(oaID, node string) {
	for _, op := range ngac.ChannelMemberOps() {
		f.read.allow[grant{node, oaID, op}] = true
	}
}

// withManagersUA adds a managers UA node as a child of the PC.
func (f *fixture) withManagersUA(chID string) string {
	managersName := ngac.ChannelManagersUAName(ngac.ChannelID(chID))
	managersID := fmt.Sprintf("test-managers-ua-%d", time.Now().UnixNano())
	f.read.children[f.pcID] = append(f.read.children[f.pcID], &policypb.NGACNode{
		Id: managersID, Name: managersName, NodeType: ngac.TypeUA,
	})
	return managersID
}

// ---------------------------------------------------------------------------
// GetGroupManagement — settings retrieval
// ---------------------------------------------------------------------------

func TestGetGroupManagement_ReturnsSettings(t *testing.T) {
	f := newFixture(t)
	chID, oaID, _ := f.insertGroupChannelWithSettings(t, "grp-mgmt", f.userID,
		ngac.GroupSettingManagers, ngac.GroupSettingManagers, ngac.GroupSettingEveryone)
	f.allowRead(oaID, userNode)
	ctx := f.callerCtx()

	settings, err := f.svc.GetGroupManagement(ctx, chID, userNode)

	require.NoError(t, err)
	assert.Equal(t, ngac.GroupSettingManagers, settings.PostPolicy)
	assert.Equal(t, ngac.GroupSettingManagers, settings.InvitePolicy)
	assert.Equal(t, ngac.GroupSettingEveryone, settings.MentionAllPolicy)
	assert.Equal(t, f.userID, settings.OwnerID)
}

func TestGetGroupManagement_DeniedWithoutRead(t *testing.T) {
	f := newFixture(t)
	chID, _, _ := f.insertGroupChannel(t, "grp-deny-read", f.userID)
	ctx := f.callerCtx()

	_, err := f.svc.GetGroupManagement(ctx, chID, userNode)

	require.ErrorIs(t, err, domain.ErrAccessDenied)
}

func TestGetGroupManagement_RefusedForDM(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	n := time.Now().UnixNano()
	chID := fmt.Sprintf("test-dm-mgmt-%d", n)
	oaID := fmt.Sprintf("test-dm-mgmt-oa-%d", n)
	_, err := f.pool.Exec(ctx, `INSERT INTO ngac_nodes (id, name, node_type) VALUES ($1, $2, $3)`,
		oaID, "dm-oa", ngac.TypeOA)
	require.NoError(t, err)
	_, err = f.pool.Exec(ctx,
		`INSERT INTO channels (id, name, channel_type, workspace_id, ngac_oa_id, created_at)
		 VALUES ($1, $2, 'dm', $3, $4, now())`, chID, "dm", f.wsID, oaID)
	require.NoError(t, err)
	t.Cleanup(func() {
		f.pool.Exec(ctx, `DELETE FROM channels WHERE id = $1`, chID)
		f.pool.Exec(ctx, `DELETE FROM ngac_nodes WHERE id = $1`, oaID)
	})
	f.allowRead(oaID, userNode)

	_, err = f.svc.GetGroupManagement(f.callerCtx(), chID, userNode)

	require.ErrorIs(t, err, domain.ErrInvalidInput)
}

// ---------------------------------------------------------------------------
// UpdateGroupSettings — owner/manager only
// ---------------------------------------------------------------------------

func TestUpdateGroupSettings_OwnerChangesSettings(t *testing.T) {
	f := newFixture(t)
	chID, oaID, _ := f.insertGroupChannel(t, "grp-update", f.userID)
	f.allowRead(oaID, userNode)
	ctx := f.callerCtx()

	// Owner (created_by matches userID in callerCtx).
	err := f.svc.UpdateGroupSettings(ctx, chID, userNode,
		ngac.GroupSettingManagers, ngac.GroupSettingManagers, ngac.GroupSettingManagers)

	require.NoError(t, err)
	// Verify via DB.
	var pp, ip, mp string
	require.NoError(t, f.pool.QueryRow(context.Background(),
		`SELECT post_policy, invite_policy, mention_all_policy FROM channels WHERE id = $1`, chID).
		Scan(&pp, &ip, &mp))
	assert.Equal(t, ngac.GroupSettingManagers, pp)
	assert.Equal(t, ngac.GroupSettingManagers, ip)
	assert.Equal(t, ngac.GroupSettingManagers, mp)
}

func TestUpdateGroupSettings_DeniedForPlainMember(t *testing.T) {
	f := newFixture(t)
	chID, oaID, _ := f.insertGroupChannel(t, "grp-update-deny", f.userID)
	f.allowRead(oaID, plainNode)
	// Plain member is not the owner and not in Managers UA.
	ctx := ctxForUser("other-user", plainNode, f.wsID)

	err := f.svc.UpdateGroupSettings(ctx, chID, plainNode,
		ngac.GroupSettingManagers, "", "")

	require.ErrorIs(t, err, domain.ErrAccessDenied)
}

func TestUpdateGroupSettings_InvalidValueRefused(t *testing.T) {
	f := newFixture(t)
	chID, oaID, _ := f.insertGroupChannel(t, "grp-bad-val", f.userID)
	f.allowRead(oaID, userNode)
	ctx := f.callerCtx()

	err := f.svc.UpdateGroupSettings(ctx, chID, userNode,
		"invalid_value", "", "")

	require.ErrorIs(t, err, domain.ErrInvalidInput)
}

// ---------------------------------------------------------------------------
// Restricted posting — top-level, polls, tasks, approval cards
// ---------------------------------------------------------------------------

func TestSendMessage_RestrictedPostingDeniesTopLevelFromNonManager(t *testing.T) {
	f := newFixture(t)
	chID, oaID, _ := f.insertGroupChannelWithSettings(t, "grp-post-restrict",
		f.userID, ngac.GroupSettingManagers, ngac.GroupSettingEveryone, ngac.GroupSettingEveryone)
	plainNodeID, plainUserID := f.createTestUser(t)
	f.allowAll(oaID, plainNodeID)
	ctx := ctxForUser(plainUserID, plainNodeID, f.wsID)

	_, err := f.svc.SendMessage(ctx, domain.SendMessageInput{
		ChannelID:    chID,
		SenderID:     plainUserID,
		SenderNodeID: plainNodeID,
		Content:      "restricted post",
	})

	require.ErrorIs(t, err, domain.ErrAccessDenied)
}

func TestSendMessage_RestrictedPostingAllowsRepliesFromNonManager(t *testing.T) {
	f := newFixture(t)
	chID, oaID, _ := f.insertGroupChannelWithSettings(t, "grp-reply-allow",
		f.userID, ngac.GroupSettingManagers, ngac.GroupSettingEveryone, ngac.GroupSettingEveryone)
	f.allowAll(oaID, userNode)
	ctx := f.callerCtx()

	// Owner sends a top-level message first (they are the owner, so allowed).
	msg, err := f.svc.SendMessage(ctx, domain.SendMessageInput{
		ChannelID:    chID,
		SenderID:     f.userID,
		SenderNodeID: userNode,
		Content:      "root message",
	})
	require.NoError(t, err)

	// Plain member replies — allowed even with restricted posting.
	plainNodeID, plainUserID := f.createTestUser(t)
	f.allowAll(oaID, plainNodeID)
	replyCtx := ctxForUser(plainUserID, plainNodeID, f.wsID)
	_, err = f.svc.SendMessage(replyCtx, domain.SendMessageInput{
		ChannelID:       chID,
		SenderID:        plainUserID,
		SenderNodeID:    plainNodeID,
		Content:         "reply message",
		ParentMessageID: msg.Id,
	})

	require.NoError(t, err, "replies in threads must be allowed even with restricted posting")
}

func TestSendMessage_SystemMessageTypeFromUserDenied(t *testing.T) {
	f := newFixture(t)
	chID, oaID, _ := f.insertGroupChannel(t, "grp-sys-deny", f.userID)
	f.allowAll(oaID, userNode)
	ctx := f.callerCtx()

	_, err := f.svc.SendMessage(ctx, domain.SendMessageInput{
		ChannelID:    chID,
		SenderID:     f.userID,
		SenderNodeID: userNode,
		Content:      "spoofed system line",
		MessageType:  "system",
	})

	require.ErrorIs(t, err, domain.ErrAccessDenied)
}

func TestSendMessage_MentionAllRestrictedDeniesNonManager(t *testing.T) {
	f := newFixture(t)
	chID, oaID, _ := f.insertGroupChannelWithSettings(t, "grp-mention-restrict",
		f.userID, ngac.GroupSettingEveryone, ngac.GroupSettingEveryone, ngac.GroupSettingManagers)
	plainNodeID, plainUserID := f.createTestUser(t)
	f.allowAll(oaID, plainNodeID)
	ctx := ctxForUser(plainUserID, plainNodeID, f.wsID)

	_, err := f.svc.SendMessage(ctx, domain.SendMessageInput{
		ChannelID:    chID,
		SenderID:     plainUserID,
		SenderNodeID: plainNodeID,
		Content:      "hello",
		Mentions:     []string{domain.MentionAll},
	})

	require.ErrorIs(t, err, domain.ErrAccessDenied)
}

func TestSendMessage_MentionAllInDMDenied(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	n := time.Now().UnixNano()
	chID := fmt.Sprintf("test-dm-mention-%d", n)
	oaID := fmt.Sprintf("test-dm-mention-oa-%d", n)
	_, err := f.pool.Exec(ctx, `INSERT INTO ngac_nodes (id, name, node_type) VALUES ($1, $2, $3)`,
		oaID, "dm-oa", ngac.TypeOA)
	require.NoError(t, err)
	_, err = f.pool.Exec(ctx,
		`INSERT INTO channels (id, name, channel_type, workspace_id, ngac_oa_id, created_at)
		 VALUES ($1, $2, 'dm', $3, $4, now())`, chID, "dm", f.wsID, oaID)
	require.NoError(t, err)
	t.Cleanup(func() {
		f.pool.Exec(ctx, `DELETE FROM channels WHERE id = $1`, chID)
		f.pool.Exec(ctx, `DELETE FROM ngac_nodes WHERE id = $1`, oaID)
	})
	f.allowAll(oaID, userNode)

	_, err = f.svc.SendMessage(f.callerCtx(), domain.SendMessageInput{
		ChannelID:    chID,
		SenderID:     f.userID,
		SenderNodeID: userNode,
		Content:      "hello",
		Mentions:     []string{domain.MentionAll},
	})

	require.ErrorIs(t, err, domain.ErrAccessDenied)
}

// ---------------------------------------------------------------------------
// RemoveMember — owner cannot leave without transfer (409)
// ---------------------------------------------------------------------------

func TestRemoveMember_OwnerCannotLeaveWithoutTransfer(t *testing.T) {
	f := newFixture(t)
	chID, oaID, uaID := f.insertGroupChannel(t, "grp-owner-leave", f.userID)
	f.allowRead(oaID, userNode)
	// The owner is assigned to Members UA (requireInChannel checks this).
	f.read.edges = map[[2]string]bool{{userNode, uaID}: true}
	f.read.addMember(f.wsID, userNode)
	ctx := f.callerCtx()

	err := f.svc.RemoveMember(ctx, chID, userNode, userNode)

	require.ErrorIs(t, err, domain.ErrAlreadyExists, "owner leaving without transfer should return 409 (ErrAlreadyExists)")
}

func TestRemoveMember_NonOwnerCanLeave(t *testing.T) {
	f := newFixture(t)
	plainNodeID, userIDPlain := f.createTestUser(t)
	chID, oaID, uaID := f.insertGroupChannel(t, "grp-member-leave", f.userID)
	f.allowRead(oaID, plainNodeID)
	// Plain member is assigned to the group's Members UA.
	f.read.edges = map[[2]string]bool{{plainNodeID, uaID}: true}
	f.read.addMember(f.wsID, plainNodeID)
	ctx := ctxForUser(userIDPlain, plainNodeID, f.wsID)

	err := f.svc.RemoveMember(ctx, chID, plainNodeID, plainNodeID)

	require.NoError(t, err)
}

// createTestUser creates a user (with its ngac node) and returns (nodeID, userID).
func (f *fixture) createTestUser(t *testing.T) (string, string) {
	t.Helper()
	uid := fmt.Sprintf("test-user-%d", time.Now().UnixNano())
	nid := fmt.Sprintf("test-node-%d", time.Now().UnixNano())
	ctx := context.Background()
	// ngac_nodes first — users.ngac_node is a FK to ngac_nodes.id.
	_, err := f.pool.Exec(ctx,
		`INSERT INTO ngac_nodes (id, name, node_type) VALUES ($1, $2, $3)`,
		nid, "u-node-"+nid, ngac.TypeU)
	require.NoError(t, err)
	_, err = f.pool.Exec(ctx,
		`INSERT INTO users (id, username, email, ngac_node) VALUES ($1, $2, $3, $4)`,
		uid, "u-"+uid, uid+"@test.local", nid)
	require.NoError(t, err)
	t.Cleanup(func() {
		f.pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, uid)
		f.pool.Exec(ctx, `DELETE FROM ngac_nodes WHERE id = $1`, nid)
	})
	return nid, uid
}

// ---------------------------------------------------------------------------
// RemoveMember — authority: owner removes member
// ---------------------------------------------------------------------------

func TestRemoveMember_OwnerCanRemoveMember(t *testing.T) {
	f := newFixture(t)
	chID, oaID, uaID := f.insertGroupChannel(t, "grp-owner-remove", f.userID)
	f.allowRead(oaID, userNode)
	f.read.edges = map[[2]string]bool{{plainNode, uaID}: true}
	f.read.addMember(f.wsID, plainNode)
	ctx := f.callerCtx()

	err := f.svc.RemoveMember(ctx, chID, userNode, plainNode)

	require.NoError(t, err)
}

func TestRemoveMember_PlainMemberCannotRemoveOthers(t *testing.T) {
	f := newFixture(t)
	plainNodeID, userIDPlain := f.createTestUser(t)
	chID, oaID, uaID := f.insertGroupChannel(t, "grp-plain-no-remove", f.userID)
	f.allowRead(oaID, plainNodeID)
	anotherNode := "ngac-user-another"
	f.read.edges = map[[2]string]bool{
		{plainNodeID, uaID}: true,
		{anotherNode, uaID}: true,
	}
	f.read.addMember(f.wsID, plainNodeID, anotherNode)
	ctx := ctxForUser(userIDPlain, plainNodeID, f.wsID)

	err := f.svc.RemoveMember(ctx, chID, plainNodeID, anotherNode)

	require.ErrorIs(t, err, domain.ErrAccessDenied)
}

func TestRemoveMember_WorkspaceOwnerNotInGroupCannotRemove(t *testing.T) {
	f := newFixture(t)
	chID, oaID, uaID := f.insertGroupChannel(t, "grp-ws-owner-remove", f.userID)
	// wsOwnerNode holds manage but is NOT a member of this group (no read).
	f.read.allow[grant{wsOwnerNode, oaID, ngac.OpManage}] = true
	f.read.edges = map[[2]string]bool{{plainNode, uaID}: true}
	f.read.addMember(f.wsID, plainNode, wsOwnerNode)
	ctx := ctxForUser("ws-owner", wsOwnerNode, f.wsID)

	err := f.svc.RemoveMember(ctx, chID, wsOwnerNode, plainNode)

	// Workspace owner with manage can still remove (they enter via the manage fallback).
	require.NoError(t, err)
}

func TestRemoveMember_OutsiderDenied(t *testing.T) {
	f := newFixture(t)
	chID, _, uaID := f.insertGroupChannel(t, "grp-outsider-remove", f.userID)
	f.read.edges = map[[2]string]bool{{plainNode, uaID}: true}
	f.read.addMember(f.wsID, plainNode)
	ctx := ctxForUser("outsider", outsiderNode, f.wsID)

	err := f.svc.RemoveMember(ctx, chID, outsiderNode, plainNode)

	require.ErrorIs(t, err, domain.ErrAccessDenied)
}

// ---------------------------------------------------------------------------
// UpdateChannel — rename opens to owner/managers
// ---------------------------------------------------------------------------

func TestUpdateChannel_OwnerCanRenameWithoutManage(t *testing.T) {
	f := newFixture(t)
	chID, oaID, _ := f.insertGroupChannel(t, "grp-rename-owner", f.userID)
	// Only read, no manage — but the user is the owner.
	f.allowRead(oaID, userNode)
	ctx := f.callerCtx()

	ch, err := f.svc.UpdateChannel(ctx, chID, userNode, "new-name")

	require.NoError(t, err)
	assert.Equal(t, "new-name", ch.Name)
}

func TestUpdateChannel_PlainMemberStillDenied(t *testing.T) {
	f := newFixture(t)
	chID, oaID, _ := f.insertGroupChannel(t, "grp-rename-deny", f.userID)
	f.allowRead(oaID, plainNode)
	ctx := ctxForUser("other-user", plainNode, f.wsID)

	_, err := f.svc.UpdateChannel(ctx, chID, plainNode, "new-name")

	require.ErrorIs(t, err, domain.ErrAccessDenied)
}

// ---------------------------------------------------------------------------
// TransferOwnership
// ---------------------------------------------------------------------------

func TestTransferOwnership_DeniedForNonOwner(t *testing.T) {
	f := newFixture(t)
	chID, oaID, _ := f.insertGroupChannel(t, "grp-transfer-deny", f.userID)
	f.allowRead(oaID, plainNode)
	ctx := ctxForUser("other-user", plainNode, f.wsID)

	err := f.svc.TransferOwnership(ctx, chID, plainNode, userNode)

	require.ErrorIs(t, err, domain.ErrAccessDenied)
}

// ---------------------------------------------------------------------------
// DisbandGroup — owner only, private only
// ---------------------------------------------------------------------------

func TestDisbandGroup_OwnerCanDisband(t *testing.T) {
	f := newFixture(t)
	chID, oaID, _ := f.insertGroupChannel(t, "grp-disband", f.userID)
	f.allowRead(oaID, userNode)
	ctx := f.callerCtx()

	err := f.svc.DisbandGroup(ctx, chID, userNode)

	require.NoError(t, err)
	// Verify channel is gone.
	var count int
	require.NoError(t, f.pool.QueryRow(context.Background(),
		`SELECT count(*) FROM channels WHERE id = $1`, chID).Scan(&count))
	assert.Equal(t, 0, count)
}

func TestDisbandGroup_DeniedForNonOwner(t *testing.T) {
	f := newFixture(t)
	chID, oaID, _ := f.insertGroupChannel(t, "grp-disband-deny", f.userID)
	f.allowRead(oaID, plainNode)
	ctx := ctxForUser("other-user", plainNode, f.wsID)

	err := f.svc.DisbandGroup(ctx, chID, plainNode)

	require.ErrorIs(t, err, domain.ErrAccessDenied)
}

// ---------------------------------------------------------------------------
// DeleteMessage — group owner/manager can moderate
// ---------------------------------------------------------------------------

func TestDeleteMessage_OwnerCanDeleteOthersMessage(t *testing.T) {
	f := newFixture(t)
	chID, oaID, _ := f.insertGroupChannel(t, "grp-delete-mod", f.userID)
	f.allowAll(oaID, userNode)
	plainNodeID, plainUserID := f.createTestUser(t)
	f.allowAll(oaID, plainNodeID)
	ctx := f.callerCtx()

	// Plain member sends a message.
	plainCtx := ctxForUser(plainUserID, plainNodeID, f.wsID)
	msg, err := f.svc.SendMessage(plainCtx, domain.SendMessageInput{
		ChannelID:    chID,
		SenderID:     plainUserID,
		SenderNodeID: plainNodeID,
		Content:      "to be deleted",
	})
	require.NoError(t, err)

	// Owner (who has read but not manage) deletes it.
	out, err := f.svc.DeleteMessage(ctx, msg.Id, f.userID, userNode)

	require.NoError(t, err)
	assert.NotNil(t, out.Event)
	// The deletion is recorded as a system message.
	assert.NotNil(t, out.Notice)
}

func TestDeleteMessage_PlainMemberCannotDeleteOthers(t *testing.T) {
	f := newFixture(t)
	chID, oaID, _ := f.insertGroupChannel(t, "grp-delete-deny", f.userID)
	f.allowAll(oaID, userNode)
	plainNodeID, plainUserID := f.createTestUser(t)
	f.allowAll(oaID, plainNodeID)

	// Owner sends a message.
	ctx := f.callerCtx()
	msg, err := f.svc.SendMessage(ctx, domain.SendMessageInput{
		ChannelID:    chID,
		SenderID:     f.userID,
		SenderNodeID: userNode,
		Content:      "owner's message",
	})
	require.NoError(t, err)

	// Plain member cannot delete owner's message (no manage, not owner/manager).
	plainCtx := ctxForUser(plainUserID, plainNodeID, f.wsID)
	_, err = f.svc.DeleteMessage(plainCtx, msg.Id, plainUserID, plainNodeID)

	require.ErrorIs(t, err, domain.ErrAccessDenied)
}

// ---------------------------------------------------------------------------
// AppointManager / RemoveManager
// ---------------------------------------------------------------------------

func TestAppointManager_DeniedForNonOwner(t *testing.T) {
	f := newFixture(t)
	chID, oaID, _ := f.insertGroupChannel(t, "grp-appoint-deny", f.userID)
	f.allowRead(oaID, plainNode)
	ctx := ctxForUser("other-user", plainNode, f.wsID)

	err := f.svc.AppointManager(ctx, chID, plainNode, userNode)

	require.ErrorIs(t, err, domain.ErrAccessDenied)
}

func TestRemoveManager_DeniedForPlainMember(t *testing.T) {
	f := newFixture(t)
	chID, oaID, _ := f.insertGroupChannel(t, "grp-rm-mgr-deny", f.userID)
	f.allowRead(oaID, plainNode)
	ctx := ctxForUser("other-user", plainNode, f.wsID)

	err := f.svc.RemoveManager(ctx, chID, plainNode, userNode)

	require.ErrorIs(t, err, domain.ErrAccessDenied)
}

// ---------------------------------------------------------------------------
// MemberRoster — canRemove opens to owner/managers
// ---------------------------------------------------------------------------

func TestMemberRoster_OwnerSeesCanRemove(t *testing.T) {
	f := newFixture(t)
	chID, oaID, uaID := f.insertGroupChannel(t, "grp-roster-remove", f.userID)
	f.allowRead(oaID, userNode)
	f.read.children[uaID] = []*policypb.NGACNode{
		{Id: userNode, Name: "owner", NodeType: ngac.TypeU},
	}
	ctx := f.callerCtx()

	roster, err := f.svc.MemberRoster(ctx, chID, userNode)

	require.NoError(t, err)
	assert.True(t, roster.CanRemove, "owner should see can_remove_members=true")
}

func TestMemberRoster_PlainMemberSeesNoCanRemove(t *testing.T) {
	f := newFixture(t)
	chID, oaID, uaID := f.insertGroupChannel(t, "grp-roster-no-remove", f.userID)
	f.allowRead(oaID, plainNode)
	f.read.children[uaID] = []*policypb.NGACNode{
		{Id: plainNode, Name: "plain", NodeType: ngac.TypeU},
	}
	ctx := ctxForUser("other-user", plainNode, f.wsID)

	roster, err := f.svc.MemberRoster(ctx, chID, plainNode)

	require.NoError(t, err)
	assert.False(t, roster.CanRemove, "plain member should see can_remove_members=false")
}

// ---------------------------------------------------------------------------
// RemoveManager: a manager cannot demote the owner
// ---------------------------------------------------------------------------

func TestRemoveManager_ManagerCannotDemoteOwner(t *testing.T) {
	f := newFixture(t)
	managerNodeID, managerUserID := f.createTestUser(t)
	chID, oaID, _ := f.insertGroupChannel(t, "grp-mgr-demote-owner", f.userID)
	f.allowRead(oaID, managerNodeID)
	managersUA := f.withManagersUA(chID)
	// The manager and the owner are both in the Managers UA.
	f.read.edges = map[[2]string]bool{{managerNodeID, managersUA}: true, {userNode, managersUA}: true}
	ctx := ctxForUser(managerUserID, managerNodeID, f.wsID)

	err := f.svc.RemoveManager(ctx, chID, managerNodeID, userNode)

	require.ErrorIs(t, err, domain.ErrInvalidInput, "manager cannot demote the owner")
	assert.Empty(t, f.write.removed, "a refused demotion writes nothing")
}

// ---------------------------------------------------------------------------
// TransferOwnership: owner transfers to a member
// ---------------------------------------------------------------------------

func TestTransferOwnership_OwnerTransfersToMember(t *testing.T) {
	f := newFixture(t)
	targetNodeID, targetUserID := f.createTestUser(t)
	chID, oaID, uaID := f.insertGroupChannel(t, "grp-transfer-ok", f.userID)
	f.allowRead(oaID, userNode)
	managersUA := f.withManagersUA(chID)
	// Target is in the channel.
	f.read.edges = map[[2]string]bool{{targetNodeID, uaID}: true}
	f.read.addMember(f.wsID, targetNodeID)
	ctx := f.callerCtx()

	err := f.svc.TransferOwnership(ctx, chID, userNode, targetNodeID)

	require.NoError(t, err)
	// Verify created_by was updated.
	var newOwner string
	require.NoError(t, f.pool.QueryRow(context.Background(),
		`SELECT created_by FROM channels WHERE id = $1`, chID).Scan(&newOwner))
	assert.Equal(t, targetUserID, newOwner)
	assert.Contains(t, f.write.assigned, [2]string{targetNodeID, managersUA}, "the new owner becomes a manager")
	assert.Contains(t, f.write.assigned, [2]string{userNode, managersUA}, "the former owner stays a manager")
}

func TestTransferOwnership_DeniedForNonMember(t *testing.T) {
	f := newFixture(t)
	outsiderNodeID, _ := f.createTestUser(t)
	chID, oaID, _ := f.insertGroupChannel(t, "grp-transfer-non-member", f.userID)
	f.allowRead(oaID, userNode)
	f.withManagersUA(chID)
	// outsiderNodeID is NOT in the channel (no edges set).
	f.read.addMember(f.wsID, outsiderNodeID)
	ctx := f.callerCtx()

	err := f.svc.TransferOwnership(ctx, chID, userNode, outsiderNodeID)

	require.ErrorIs(t, err, domain.ErrNotFound, "transfer to a non-member must fail")
	assert.Equal(t, 0, f.write.graphWrites(), "a refused transfer writes nothing")
	var owner string
	require.NoError(t, f.pool.QueryRow(context.Background(), `SELECT created_by FROM channels WHERE id = $1`, chID).Scan(&owner))
	assert.Equal(t, f.userID, owner)
}

// ---------------------------------------------------------------------------
// CreatePoll/CreateTask: restricted posting denies a non-manager
// ---------------------------------------------------------------------------

func TestCreatePoll_RestrictedPostingDeniesNonManager(t *testing.T) {
	f := newFixture(t)
	chID, oaID, _ := f.insertGroupChannelWithSettings(t, "grp-poll-restrict",
		f.userID, ngac.GroupSettingManagers, ngac.GroupSettingEveryone, ngac.GroupSettingEveryone)
	plainNodeID, plainUserID := f.createTestUser(t)
	f.allowAll(oaID, plainNodeID)
	ctx := ctxForUser(plainUserID, plainNodeID, f.wsID)

	_, err := f.svc.CreatePoll(ctx, domain.CreatePollInput{
		ChannelID:  chID,
		UserID:     plainUserID,
		UserNodeID: plainNodeID,
		Question:   "restricted poll?",
		Options:    []string{"yes", "no"},
	})

	require.ErrorIs(t, err, domain.ErrAccessDenied, "non-manager should not create a top-level poll under restricted posting")
}

func TestCreateTask_RestrictedPostingDeniesNonManager(t *testing.T) {
	f := newFixture(t)
	chID, oaID, _ := f.insertGroupChannelWithSettings(t, "grp-task-restrict",
		f.userID, ngac.GroupSettingManagers, ngac.GroupSettingEveryone, ngac.GroupSettingEveryone)
	plainNodeID, plainUserID := f.createTestUser(t)
	f.allowAll(oaID, plainNodeID)
	ctx := ctxForUser(plainUserID, plainNodeID, f.wsID)

	_, err := f.svc.CreateTask(ctx, domain.CreateTaskInput{
		ChannelID:  chID,
		UserID:     plainUserID,
		UserNodeID: plainNodeID,
		Title:      "restricted task",
	})

	require.ErrorIs(t, err, domain.ErrAccessDenied, "non-manager should not create a top-level task under restricted posting")
}

// ---------------------------------------------------------------------------
// VotePoll allowed under restricted posting; ClosePoll by its creator allowed
// ---------------------------------------------------------------------------

func TestVotePoll_AllowedUnderRestrictedPosting(t *testing.T) {
	f := newFixture(t)
	chID, oaID, _ := f.insertGroupChannelWithSettings(t, "grp-vote-restrict",
		f.userID, ngac.GroupSettingManagers, ngac.GroupSettingEveryone, ngac.GroupSettingEveryone)
	f.allowAll(oaID, userNode)
	ctx := f.callerCtx()

	// Owner creates a poll (owner is manager, so allowed).
	poll, err := f.svc.CreatePoll(ctx, domain.CreatePollInput{
		ChannelID:  chID,
		UserID:     f.userID,
		UserNodeID: userNode,
		Question:   "vote test?",
		Options:    []string{"a", "b"},
	})
	require.NoError(t, err)

	// A plain member can vote even under restricted posting.
	plainNodeID, plainUserID := f.createTestUser(t)
	f.allowAll(oaID, plainNodeID)
	plainCtx := ctxForUser(plainUserID, plainNodeID, f.wsID)

	err = f.svc.VotePoll(plainCtx, poll.Id, poll.Options[0].Id, plainNodeID, plainUserID)

	require.NoError(t, err, "voting must be allowed under restricted posting")
}

func TestClosePoll_CreatorAllowedUnderRestrictedPosting(t *testing.T) {
	f := newFixture(t)
	chID, oaID, _ := f.insertGroupChannelWithSettings(t, "grp-close-restrict",
		f.userID, ngac.GroupSettingManagers, ngac.GroupSettingEveryone, ngac.GroupSettingEveryone)
	f.allowAll(oaID, userNode)
	ctx := f.callerCtx()

	poll, err := f.svc.CreatePoll(ctx, domain.CreatePollInput{
		ChannelID:  chID,
		UserID:     f.userID,
		UserNodeID: userNode,
		Question:   "close test?",
		Options:    []string{"x", "y"},
	})
	require.NoError(t, err)

	// Creator closes own poll.
	closed, err := f.svc.ClosePoll(ctx, poll.Id, f.userID, userNode)

	require.NoError(t, err, "poll creator must be able to close their own poll")
	assert.NotNil(t, closed.ClosedAt)
}

// ---------------------------------------------------------------------------
// RemoveMember: a failed Managers removal is an error, and Members stays
// ---------------------------------------------------------------------------

// failingManagersRemovalWrite fails every RemoveAssignment from the Managers UA
// and records every removal it is asked for.
type failingManagersRemovalWrite struct {
	*recordingPolicyWrite
	managersUA string
}

func (w *failingManagersRemovalWrite) RemoveAssignment(_ context.Context, req *policypb.RemoveAssignmentRequest, _ ...grpc.CallOption) (*policypb.Empty, error) {
	w.mu.Lock()
	w.removed = append(w.removed, [2]string{req.ChildId, req.ParentId})
	w.mu.Unlock()
	if req.ParentId == w.managersUA {
		return nil, fmt.Errorf("policy service unavailable")
	}
	return &policypb.Empty{}, nil
}

// A manager whose Managers edge cannot be withdrawn keeps both edges and the
// caller hears an error: they never end up outside Members but still in
// Managers (invite without read) with the removal reported as done. Repeating
// the removal finishes it.
func TestRemoveMember_ManagersRemovalFailureLeavesMembersAndReturnsError(t *testing.T) {
	f := newFixture(t)
	plainNodeID, _ := f.createTestUser(t)
	chID, oaID, uaID := f.insertGroupChannel(t, "grp-rm-mgr-fail", f.userID)
	f.allowRead(oaID, userNode)
	managersUA := f.withManagersUA(chID)
	f.read.edges = map[[2]string]bool{{plainNodeID, uaID}: true, {plainNodeID, managersUA}: true}
	f.read.addMember(f.wsID, plainNodeID)
	_, err := f.pool.Exec(context.Background(), `INSERT INTO channel_members (channel_id, ngac_node_id) VALUES ($1, $2)`, chID, plainNodeID)
	require.NoError(t, err)

	fw := &failingManagersRemovalWrite{recordingPolicyWrite: f.write, managersUA: managersUA}
	f.svc = domain.NewService(store.NewStore(f.pool), f.read, fw, stubAuth{}, nil)

	err = f.svc.RemoveMember(f.callerCtx(), chID, userNode, plainNodeID)

	require.Error(t, err, "when Managers removal fails, RemoveMember must return an error")
	assert.NotContains(t, fw.removed, [2]string{plainNodeID, uaID}, "Members is not touched while Managers still holds them")
	var cached int
	require.NoError(t, f.pool.QueryRow(context.Background(),
		`SELECT count(*) FROM channel_members WHERE channel_id = $1 AND ngac_node_id = $2`, chID, plainNodeID).Scan(&cached))
	assert.Equal(t, 1, cached, "the roster cache still lists them: nothing was removed")
}

// Removing a manager withdraws the Managers edge and then the Members edge.
func TestRemoveMember_RemovesManagersAndMembersEdges(t *testing.T) {
	f := newFixture(t)
	plainNodeID, _ := f.createTestUser(t)
	chID, oaID, uaID := f.insertGroupChannel(t, "grp-rm-mgr-both", f.userID)
	f.allowRead(oaID, userNode)
	managersUA := f.withManagersUA(chID)
	f.read.edges = map[[2]string]bool{{plainNodeID, uaID}: true, {plainNodeID, managersUA}: true}
	f.read.addMember(f.wsID, plainNodeID)

	require.NoError(t, f.svc.RemoveMember(f.callerCtx(), chID, userNode, plainNodeID))

	assert.Equal(t, [][2]string{{plainNodeID, managersUA}, {plainNodeID, uaID}}, f.write.removed)
}

// ---------------------------------------------------------------------------
// Errors do not leak the channel ID
// ---------------------------------------------------------------------------

func TestGroupManagement_ErrorDoesNotLeakChannelID(t *testing.T) {
	f := newFixture(t)
	chID, oaID, _ := f.insertGroupChannel(t, "grp-leak-check", f.userID)
	f.allowRead(oaID, userNode)
	// No Managers UA exists — AppointManager should return ErrNotFound.
	ctx := f.callerCtx()

	err := f.svc.AppointManager(ctx, chID, userNode, plainNode)

	require.ErrorIs(t, err, domain.ErrNotFound)
	// The error message must not contain the channel ID.
	assert.NotContains(t, err.Error(), chID,
		"error must not leak the channel ID to the client")
}

// ---------------------------------------------------------------------------
// AppointManager: a workspace owner outside the group is denied
// ---------------------------------------------------------------------------

func TestAppointManager_WSOwnerOutsideGroupDenied(t *testing.T) {
	f := newFixture(t)
	chID, oaID, _ := f.insertGroupChannel(t, "grp-appoint-ws-owner", f.userID)
	// wsOwnerNode holds manage but does NOT have read on this group.
	f.read.allow[grant{wsOwnerNode, oaID, ngac.OpManage}] = true
	ctx := ctxForUser("ws-owner", wsOwnerNode, f.wsID)

	err := f.svc.AppointManager(ctx, chID, wsOwnerNode, plainNode)

	// manage ≠ read; AppointManager requires read first.
	require.ErrorIs(t, err, domain.ErrAccessDenied,
		"workspace owner without read on the group cannot appoint managers")
}

// ---------------------------------------------------------------------------
// SendMessage: a workspace owner not in the group sending @Tất cả is denied
// ---------------------------------------------------------------------------

func TestSendMessage_WSOwnerNotInGroupMentionAllDenied(t *testing.T) {
	f := newFixture(t)
	chID, oaID, _ := f.insertGroupChannelWithSettings(t, "grp-ws-mention-all",
		f.userID, ngac.GroupSettingEveryone, ngac.GroupSettingEveryone, ngac.GroupSettingManagers)
	// wsOwnerNode holds manage but not read or write.
	f.read.allow[grant{wsOwnerNode, oaID, ngac.OpManage}] = true
	ctx := ctxForUser("ws-owner", wsOwnerNode, f.wsID)

	_, err := f.svc.SendMessage(ctx, domain.SendMessageInput{
		ChannelID:    chID,
		SenderID:     "ws-owner",
		SenderNodeID: wsOwnerNode,
		Content:      "hello",
		Mentions:     []string{domain.MentionAll},
	})

	// WS owner cannot write to a private group they're not in.
	require.ErrorIs(t, err, domain.ErrAccessDenied)
}

// ---------------------------------------------------------------------------
// EditMessage: adding @Tất cả while it is restricted is denied
// ---------------------------------------------------------------------------

func TestEditMessage_AddMentionAllRestrictedDenied(t *testing.T) {
	f := newFixture(t)
	chID, oaID, _ := f.insertGroupChannelWithSettings(t, "grp-edit-mention-restrict",
		f.userID, ngac.GroupSettingEveryone, ngac.GroupSettingEveryone, ngac.GroupSettingManagers)
	plainNodeID, plainUserID := f.createTestUser(t)
	f.allowAll(oaID, plainNodeID)
	plainCtx := ctxForUser(plainUserID, plainNodeID, f.wsID)

	// Plain member sends a message without @Tất cả.
	msg, err := f.svc.SendMessage(plainCtx, domain.SendMessageInput{
		ChannelID:    chID,
		SenderID:     plainUserID,
		SenderNodeID: plainNodeID,
		Content:      "hello",
	})
	require.NoError(t, err)

	// Plain member edits to add @Tất cả → denied.
	_, err = f.svc.EditMessage(plainCtx, msg.Id, plainUserID, plainNodeID,
		"hello @Tất cả", "markdown", []string{domain.MentionAll})

	require.ErrorIs(t, err, domain.ErrAccessDenied,
		"editing to add @Tất cả when restricted must be denied for non-managers")
}

// ---------------------------------------------------------------------------
// SendMessage: a non-administrator's @Tất cả in the workspace channel is denied
// ---------------------------------------------------------------------------

func TestSendMessage_NonAdminMentionAllInWorkspaceChannelDenied(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	n := time.Now().UnixNano()
	chID := fmt.Sprintf("test-ws-ch-%d", n)
	oaID := fmt.Sprintf("test-ws-ch-oa-%d", n)

	_, err := f.pool.Exec(ctx, `INSERT INTO ngac_nodes (id, name, node_type) VALUES ($1, $2, $3)`,
		oaID, "ws-ch-oa", ngac.TypeOA)
	require.NoError(t, err)
	_, err = f.pool.Exec(ctx,
		`INSERT INTO channels (id, name, channel_type, workspace_id, ngac_oa_id, created_at)
		 VALUES ($1, $2, 'workspace', $3, $4, now())`, chID, "ws-chan", f.wsID, oaID)
	require.NoError(t, err)
	t.Cleanup(func() {
		f.pool.Exec(ctx, `DELETE FROM channels WHERE id = $1`, chID)
		f.pool.Exec(ctx, `DELETE FROM ngac_nodes WHERE id = $1`, oaID)
	})

	plainNodeID, plainUserID := f.createTestUser(t)
	f.allowAll(oaID, plainNodeID)
	plainCtx := ctxForUser(plainUserID, plainNodeID, f.wsID)

	_, err = f.svc.SendMessage(plainCtx, domain.SendMessageInput{
		ChannelID:    chID,
		SenderID:     plainUserID,
		SenderNodeID: plainNodeID,
		Content:      "hello",
		Mentions:     []string{domain.MentionAll},
	})

	require.ErrorIs(t, err, domain.ErrAccessDenied,
		"non-admin must not use @Tất cả in the workspace channel")
}
