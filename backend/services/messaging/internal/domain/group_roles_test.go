package domain_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"ngac-platform/ngac"
	policypb "ngac-platform/proto/policy"
	"ngac-platform/services/messaging/internal/domain"
	"ngac-platform/services/messaging/internal/store"
)

// The deny and allow boundary of every group-management decision, one path per
// test: who may appoint, demote, transfer, disband, remove, post and mention.

// groupWorld is a private group owned by the fixture's user (signing in as
// userNode), with a Managers UA, and people put into it per test.
type groupWorld struct {
	f                *fixture
	chID, oaID, uaID string
	managersUA       string
}

func newGroupWorld(t *testing.T, postPolicy, invitePolicy, mentionAllPolicy string) *groupWorld {
	t.Helper()
	f := newFixture(t)
	chID, oaID, uaID := f.insertGroupChannelWithSettings(t, "grp-roles", f.userID, postPolicy, invitePolicy, mentionAllPolicy)
	w := &groupWorld{f: f, chID: chID, oaID: oaID, uaID: uaID, managersUA: f.withManagersUA(chID)}
	f.read.edges = map[[2]string]bool{{userNode, uaID}: true, {userNode, w.managersUA}: true}
	f.read.addMember(f.wsID, userNode)
	f.allowAll(oaID, userNode)
	t.Cleanup(func() {
		f.pool.Exec(context.Background(), `DELETE FROM messages WHERE channel_id = $1`, chID)
		f.pool.Exec(context.Background(), `DELETE FROM polls WHERE channel_id = $1`, chID)
		f.pool.Exec(context.Background(), `DELETE FROM chat_tasks WHERE channel_id = $1`, chID)
		f.pool.Exec(context.Background(), `DELETE FROM message_pins WHERE channel_id = $1`, chID)
	})
	return w
}

type groupPerson struct{ id, node string }

// member adds a person directly under the group's Members UA.
func (w *groupWorld) member(t *testing.T) groupPerson {
	t.Helper()
	node, id := w.f.createTestUser(t)
	w.f.read.edges[[2]string{node, w.uaID}] = true
	w.f.read.addMember(w.f.wsID, node)
	w.f.allowAll(w.oaID, node)
	return groupPerson{id: id, node: node}
}

// manager is a member who is also under the Managers UA.
func (w *groupWorld) manager(t *testing.T) groupPerson {
	t.Helper()
	p := w.member(t)
	w.f.read.edges[[2]string{p.node, w.managersUA}] = true
	return p
}

func (w *groupWorld) ctx(p groupPerson) context.Context {
	return ctxForUser(p.id, p.node, w.f.wsID)
}

func (w *groupWorld) owner() groupPerson { return groupPerson{id: w.f.userID, node: userNode} }

func (w *groupWorld) createdBy(t *testing.T) string {
	t.Helper()
	var owner string
	require.NoError(t, w.f.pool.QueryRow(context.Background(), `SELECT created_by FROM channels WHERE id = $1`, w.chID).Scan(&owner))
	return owner
}

func (w *groupWorld) send(p groupPerson, in domain.SendMessageInput) (string, error) {
	in.ChannelID, in.SenderID, in.SenderNodeID = w.chID, p.id, p.node
	if in.Content == "" {
		in.Content = "xin chào"
	}
	msg, err := w.f.svc.SendMessage(w.ctx(p), in)
	if err != nil {
		return "", err
	}
	return msg.Id, nil
}

// ---------------------------------------------------------------------------
// A new group is born with its Managers UA, the creator in it
// ---------------------------------------------------------------------------

func TestCreateChannel_GroupGetsAManagersUAHoldingItsOwner(t *testing.T) {
	f := newFixture(t)
	channelsOA := f.withChannelsOA()
	f.read.allow[grant{userNode, channelsOA, ngac.OpCreateChannel}] = true

	ch, err := f.svc.CreateChannel(context.Background(), f.createInput(fmt.Sprintf("grp-born-%d", time.Now().UnixNano())))
	require.NoError(t, err)

	want := ngac.ChannelManagersUAName(ngac.ChannelID(ch.Id))
	var managersUA string
	for _, id := range f.write.nodes {
		if strings.HasPrefix(id, "test-node-"+want+"-") {
			managersUA = id
		}
	}
	require.NotEmpty(t, managersUA, "the group has a Managers UA named by its ID")
	assert.Contains(t, f.write.assigned, [2]string{managersUA, f.pcID}, "beside Members, under the workspace PC")
	assert.Contains(t, f.write.assigned, [2]string{userNode, managersUA}, "the owner is a manager")
}

// ---------------------------------------------------------------------------
// Reading the roles
// ---------------------------------------------------------------------------

func TestGetGroupManagement_NamesManagersByUserID(t *testing.T) {
	w := newGroupWorld(t, ngac.GroupSettingEveryone, ngac.GroupSettingEveryone, ngac.GroupSettingEveryone)
	m := w.manager(t)
	w.f.read.children[w.managersUA] = []*policypb.NGACNode{{Id: m.node, NodeType: ngac.TypeU}}

	got, err := w.f.svc.GetGroupManagement(w.ctx(w.owner()), w.chID, userNode)

	require.NoError(t, err)
	assert.Equal(t, w.f.userID, got.OwnerID)
	assert.Equal(t, []string{m.id}, got.ManagerIDs)
}

// ---------------------------------------------------------------------------
// Appointing and removing managers
// ---------------------------------------------------------------------------

func TestAppointManager_OwnerAppointsAMember(t *testing.T) {
	w := newGroupWorld(t, ngac.GroupSettingEveryone, ngac.GroupSettingEveryone, ngac.GroupSettingEveryone)
	p := w.member(t)

	require.NoError(t, w.f.svc.AppointManager(w.ctx(w.owner()), w.chID, userNode, p.node))

	assert.Equal(t, [][2]string{{p.node, w.managersUA}}, w.f.write.assigned)
}

func TestAppointManager_ManagerAppointsAMember(t *testing.T) {
	w := newGroupWorld(t, ngac.GroupSettingEveryone, ngac.GroupSettingEveryone, ngac.GroupSettingEveryone)
	m, p := w.manager(t), w.member(t)

	require.NoError(t, w.f.svc.AppointManager(w.ctx(m), w.chID, m.node, p.node))

	assert.Equal(t, [][2]string{{p.node, w.managersUA}}, w.f.write.assigned)
}

func TestAppointManager_NonMemberIsRefusedAndNothingWritten(t *testing.T) {
	w := newGroupWorld(t, ngac.GroupSettingEveryone, ngac.GroupSettingEveryone, ngac.GroupSettingEveryone)
	outsiderNode, _ := w.f.createTestUser(t)
	w.f.read.addMember(w.f.wsID, outsiderNode) // in the workspace, not in the group

	err := w.f.svc.AppointManager(w.ctx(w.owner()), w.chID, userNode, outsiderNode)

	require.ErrorIs(t, err, domain.ErrNotFound)
	assert.Equal(t, 0, w.f.write.graphWrites())
}

// A workspace owner holds manage on every group through the Channels OA; that
// is not a way into a group's roles.
func TestAppointManager_WorkspaceOwnerOutsideTheGroupCannotAppointThemself(t *testing.T) {
	w := newGroupWorld(t, ngac.GroupSettingEveryone, ngac.GroupSettingEveryone, ngac.GroupSettingEveryone)
	w.f.read.allow[grant{wsOwnerNode, w.oaID, ngac.OpManage}] = true
	ws := groupPerson{id: "ws-owner", node: wsOwnerNode}

	err := w.f.svc.AppointManager(w.ctx(ws), w.chID, wsOwnerNode, wsOwnerNode)

	require.ErrorIs(t, err, domain.ErrAccessDenied)
	assert.Equal(t, 0, w.f.write.graphWrites())
}

// A workspace owner who is also a plain member of the group gains nothing from
// manage either.
func TestUpdateGroupSettings_WorkspaceOwnerInsideTheGroupAsMemberDenied(t *testing.T) {
	w := newGroupWorld(t, ngac.GroupSettingEveryone, ngac.GroupSettingEveryone, ngac.GroupSettingEveryone)
	p := w.member(t)
	w.f.read.allow[grant{p.node, w.oaID, ngac.OpManage}] = true

	err := w.f.svc.UpdateGroupSettings(w.ctx(p), w.chID, p.node, ngac.GroupSettingManagers, "", "")

	require.ErrorIs(t, err, domain.ErrAccessDenied)
	assert.Equal(t, 0, w.f.write.graphWrites())
}

func TestRemoveManager_ManagerDemotesAnotherWhoStaysAMember(t *testing.T) {
	w := newGroupWorld(t, ngac.GroupSettingEveryone, ngac.GroupSettingEveryone, ngac.GroupSettingEveryone)
	m, other := w.manager(t), w.manager(t)

	require.NoError(t, w.f.svc.RemoveManager(w.ctx(m), w.chID, m.node, other.node))

	assert.Equal(t, [][2]string{{other.node, w.managersUA}}, w.f.write.removed, "only the Managers edge goes")
}

func TestRemoveManager_OwnerCannotDemoteThemself(t *testing.T) {
	w := newGroupWorld(t, ngac.GroupSettingEveryone, ngac.GroupSettingEveryone, ngac.GroupSettingEveryone)

	err := w.f.svc.RemoveManager(w.ctx(w.owner()), w.chID, userNode, userNode)

	require.ErrorIs(t, err, domain.ErrInvalidInput)
	assert.Empty(t, w.f.write.removed)
}

// ---------------------------------------------------------------------------
// Transfer and what the former owner keeps
// ---------------------------------------------------------------------------

func TestTransferOwnership_ManagerCannotTransfer(t *testing.T) {
	w := newGroupWorld(t, ngac.GroupSettingEveryone, ngac.GroupSettingEveryone, ngac.GroupSettingEveryone)
	m, p := w.manager(t), w.member(t)

	err := w.f.svc.TransferOwnership(w.ctx(m), w.chID, m.node, p.node)

	require.ErrorIs(t, err, domain.ErrAccessDenied)
	assert.Equal(t, 0, w.f.write.graphWrites())
	assert.Equal(t, w.f.userID, w.createdBy(t))
}

// Someone of another workspace is never under this group's Members UA, so a
// transfer to them is refused like any non-member.
func TestTransferOwnership_PersonOfAnotherWorkspaceRefused(t *testing.T) {
	w := newGroupWorld(t, ngac.GroupSettingEveryone, ngac.GroupSettingEveryone, ngac.GroupSettingEveryone)
	strangerNode, _ := w.f.createTestUser(t)
	w.f.read.addMember("some-other-workspace", strangerNode)

	err := w.f.svc.TransferOwnership(w.ctx(w.owner()), w.chID, userNode, strangerNode)

	require.ErrorIs(t, err, domain.ErrNotFound)
	assert.Equal(t, 0, w.f.write.graphWrites())
	assert.Equal(t, w.f.userID, w.createdBy(t))
}

func TestTransferOwnership_FailedManagersWriteLeavesTheOwner(t *testing.T) {
	w := newGroupWorld(t, ngac.GroupSettingEveryone, ngac.GroupSettingEveryone, ngac.GroupSettingEveryone)
	p := w.member(t)
	fw := &failingAssignWrite{recordingPolicyWrite: w.f.write}
	w.f.withPolicyWrite(fw)

	err := w.f.svc.TransferOwnership(w.ctx(w.owner()), w.chID, userNode, p.node)

	require.Error(t, err)
	assert.Equal(t, w.f.userID, w.createdBy(t), "ownership does not move when the new owner could not be made a manager")
}

// After A hands the group to B, A is a manager like any other: A cannot remove
// B or disband the group, can still remove a plain member, and once demoted as
// well cannot remove anyone, since the creator shortcut moved with the transfer.
func TestTransferOwnership_FormerOwnerKeepsOnlyManagerPowers(t *testing.T) {
	w := newGroupWorld(t, ngac.GroupSettingEveryone, ngac.GroupSettingEveryone, ngac.GroupSettingEveryone)
	a := w.owner()
	b, c, d := w.member(t), w.member(t), w.member(t)

	require.NoError(t, w.f.svc.TransferOwnership(w.ctx(a), w.chID, a.node, b.node))
	require.Equal(t, b.id, w.createdBy(t))
	w.f.read.edges[[2]string{b.node, w.managersUA}] = true

	require.ErrorIs(t, w.f.svc.RemoveMember(w.ctx(a), w.chID, a.node, b.node), domain.ErrAccessDenied,
		"the former owner cannot remove the new owner")
	require.ErrorIs(t, w.f.svc.DisbandGroup(w.ctx(a), w.chID, a.node), domain.ErrAccessDenied,
		"the former owner cannot disband")
	require.ErrorIs(t, w.f.svc.TransferOwnership(w.ctx(a), w.chID, a.node, c.node), domain.ErrAccessDenied,
		"the former owner cannot transfer again")
	require.NoError(t, w.f.svc.RemoveMember(w.ctx(a), w.chID, a.node, c.node), "as a manager A still removes a member")

	delete(w.f.read.edges, [2]string{a.node, w.managersUA}) // A demoted too
	require.ErrorIs(t, w.f.svc.RemoveMember(w.ctx(a), w.chID, a.node, d.node), domain.ErrAccessDenied,
		"a demoted former owner removes nobody")
}

// A manager cannot remove the group's owner either.
func TestRemoveMember_ManagerCannotRemoveTheOwner(t *testing.T) {
	w := newGroupWorld(t, ngac.GroupSettingEveryone, ngac.GroupSettingEveryone, ngac.GroupSettingEveryone)
	m := w.manager(t)

	err := w.f.svc.RemoveMember(w.ctx(m), w.chID, m.node, userNode)

	require.ErrorIs(t, err, domain.ErrAccessDenied)
	assert.Empty(t, w.f.write.removed)
}

func TestRemoveMember_ManagerRemovesAPlainMember(t *testing.T) {
	w := newGroupWorld(t, ngac.GroupSettingEveryone, ngac.GroupSettingEveryone, ngac.GroupSettingEveryone)
	m, p := w.manager(t), w.member(t)

	require.NoError(t, w.f.svc.RemoveMember(w.ctx(m), w.chID, m.node, p.node))

	assert.Contains(t, w.f.write.removed, [2]string{p.node, w.uaID})
}

// ---------------------------------------------------------------------------
// Disband
// ---------------------------------------------------------------------------

func TestDisbandGroup_DeletesTheManagersUAWithTheGroup(t *testing.T) {
	w := newGroupWorld(t, ngac.GroupSettingEveryone, ngac.GroupSettingEveryone, ngac.GroupSettingEveryone)

	require.NoError(t, w.f.svc.DisbandGroup(w.ctx(w.owner()), w.chID, userNode))

	assert.ElementsMatch(t, []string{w.managersUA, w.uaID, w.oaID}, w.f.write.deleted)
}

func TestDisbandGroup_ManagerDeniedAndGroupStays(t *testing.T) {
	w := newGroupWorld(t, ngac.GroupSettingEveryone, ngac.GroupSettingEveryone, ngac.GroupSettingEveryone)
	m := w.manager(t)

	require.ErrorIs(t, w.f.svc.DisbandGroup(w.ctx(m), w.chID, m.node), domain.ErrAccessDenied)

	assert.Empty(t, w.f.write.deleted)
	assert.Equal(t, w.f.userID, w.createdBy(t), "the group is still there")
}

// ---------------------------------------------------------------------------
// Restricted posting: what a plain member keeps
// ---------------------------------------------------------------------------

func TestRestrictedPosting_MemberKeepsReactionsPinsAndEditingTheirReply(t *testing.T) {
	w := newGroupWorld(t, ngac.GroupSettingManagers, ngac.GroupSettingEveryone, ngac.GroupSettingEveryone)
	p := w.member(t)
	root, err := w.send(w.owner(), domain.SendMessageInput{Content: "thông báo"})
	require.NoError(t, err)

	require.NoError(t, w.f.svc.AddReaction(w.ctx(p), root, p.node, p.id, "👍"), "react")
	require.NoError(t, w.f.svc.PinMessage(w.ctx(p), w.chID, root, p.node, p.id), "pin")
	reply, err := w.send(p, domain.SendMessageInput{Content: "đã rõ", ParentMessageID: root})
	require.NoError(t, err, "reply in the thread")
	_, err = w.f.svc.EditMessage(w.ctx(p), reply, p.id, p.node, "đã rõ, cảm ơn", "markdown", nil)
	require.NoError(t, err, "edit their own reply")
}

func TestRestrictedPosting_ManagerPostsAtTheTopLevel(t *testing.T) {
	w := newGroupWorld(t, ngac.GroupSettingManagers, ngac.GroupSettingEveryone, ngac.GroupSettingEveryone)
	m := w.manager(t)

	_, err := w.send(m, domain.SendMessageInput{Content: "lịch họp"})

	require.NoError(t, err)
}

func TestRestrictedPosting_PollAndTaskInAThreadAreReplies(t *testing.T) {
	w := newGroupWorld(t, ngac.GroupSettingManagers, ngac.GroupSettingEveryone, ngac.GroupSettingEveryone)
	p := w.member(t)
	root, err := w.send(w.owner(), domain.SendMessageInput{Content: "chọn ngày"})
	require.NoError(t, err)

	_, err = w.f.svc.CreatePoll(w.ctx(p), domain.CreatePollInput{
		ChannelID: w.chID, UserID: p.id, UserNodeID: p.node, Question: "thứ mấy?", Options: []string{"2", "3"}, ParentMessageID: root,
	})
	require.NoError(t, err, "a poll inside a thread is a reply")
	_, err = w.f.svc.CreateTask(w.ctx(p), domain.CreateTaskInput{
		ChannelID: w.chID, UserID: p.id, UserNodeID: p.node, Title: "đặt phòng", ParentMessageID: root,
	})
	require.NoError(t, err, "a task inside a thread is a reply")
}

func TestRestrictedPosting_MemberClosesTheirOwnThreadPoll(t *testing.T) {
	w := newGroupWorld(t, ngac.GroupSettingManagers, ngac.GroupSettingEveryone, ngac.GroupSettingEveryone)
	p := w.member(t)
	root, err := w.send(w.owner(), domain.SendMessageInput{Content: "chọn ngày"})
	require.NoError(t, err)
	poll, err := w.f.svc.CreatePoll(w.ctx(p), domain.CreatePollInput{
		ChannelID: w.chID, UserID: p.id, UserNodeID: p.node, Question: "thứ mấy?", Options: []string{"2", "3"}, ParentMessageID: root,
	})
	require.NoError(t, err)

	closed, err := w.f.svc.ClosePoll(w.ctx(p), poll.Id, p.id, p.node)

	require.NoError(t, err)
	assert.NotNil(t, closed.ClosedAt)
}

// An approval card is a post: refused from a plain member in the main stream
// while posting is restricted, accepted inside a thread.
func TestRestrictedPosting_ApprovalCardFollowsThePostingRule(t *testing.T) {
	w := newGroupWorld(t, ngac.GroupSettingManagers, ngac.GroupSettingEveryone, ngac.GroupSettingEveryone)
	_, err := w.f.pool.Exec(context.Background(), `UPDATE workspaces SET type = 'organization' WHERE id = $1`, w.f.wsID)
	require.NoError(t, err)
	p := w.member(t)
	card := domain.SendMessageInput{LinkedEntityType: "approval", LinkedEntityID: "req-1"}

	_, err = w.send(p, card)
	require.ErrorIs(t, err, domain.ErrAccessDenied, "top-level approval card from a plain member")

	root, err := w.send(w.owner(), domain.SendMessageInput{Content: "đề nghị"})
	require.NoError(t, err)
	card.ParentMessageID = root
	_, err = w.send(p, card)
	require.NoError(t, err, "the same card inside a thread is a reply")
}

// ---------------------------------------------------------------------------
// @Tất cả
// ---------------------------------------------------------------------------

func TestMentionAll_RestrictedRefusesAMemberOnReplyAndAllowsAManager(t *testing.T) {
	w := newGroupWorld(t, ngac.GroupSettingEveryone, ngac.GroupSettingEveryone, ngac.GroupSettingManagers)
	p, m := w.member(t), w.manager(t)
	root, err := w.send(w.owner(), domain.SendMessageInput{Content: "gốc"})
	require.NoError(t, err)

	_, err = w.send(p, domain.SendMessageInput{ParentMessageID: root, Mentions: []string{domain.MentionAll}})
	require.ErrorIs(t, err, domain.ErrAccessDenied, "a reply with @Tất cả from a plain member")

	_, err = w.send(m, domain.SendMessageInput{Mentions: []string{domain.MentionAll}})
	require.NoError(t, err, "a manager mentions everyone")
}

func TestMentionAll_UnrestrictedLetsAMemberMentionEveryone(t *testing.T) {
	w := newGroupWorld(t, ngac.GroupSettingEveryone, ngac.GroupSettingEveryone, ngac.GroupSettingEveryone)
	p := w.member(t)

	_, err := w.send(p, domain.SendMessageInput{Mentions: []string{domain.MentionAll}})

	require.NoError(t, err)
}

// insertWorkspaceChannel writes the workspace-wide channel with a content OA.
func (f *fixture) insertWorkspaceChannel(t *testing.T) (chID, oaID string) {
	t.Helper()
	ctx := context.Background()
	n := time.Now().UnixNano()
	chID, oaID = fmt.Sprintf("test-wsall-ch-%d", n), fmt.Sprintf("test-wsall-oa-%d", n)
	_, err := f.pool.Exec(ctx, `INSERT INTO ngac_nodes (id, name, node_type) VALUES ($1, $1, $2)`, oaID, ngac.TypeOA)
	require.NoError(t, err)
	_, err = f.pool.Exec(ctx,
		`INSERT INTO channels (id, name, channel_type, workspace_id, ngac_oa_id, created_at) VALUES ($1, 'chung', 'workspace', $2, $3, now())`,
		chID, f.wsID, oaID)
	require.NoError(t, err)
	t.Cleanup(func() {
		f.pool.Exec(ctx, `DELETE FROM messages WHERE channel_id = $1`, chID)
		f.pool.Exec(ctx, `DELETE FROM channels WHERE id = $1`, chID)
		f.pool.Exec(ctx, `DELETE FROM ngac_nodes WHERE id = $1`, oaID)
	})
	return chID, oaID
}

// withMgmtOA places the workspace's Mgmt OA under its PC; manage on it is what
// makes someone a workspace administrator.
func (f *fixture) withMgmtOA() string {
	id := fmt.Sprintf("test-mgmt-oa-%d", time.Now().UnixNano())
	f.read.children[f.pcID] = append(f.read.children[f.pcID], &policypb.NGACNode{
		Id: id, Name: ngac.MgmtOAName(ngac.WorkspaceID(f.wsID)), NodeType: ngac.TypeOA,
	})
	return id
}

func TestMentionAll_WorkspaceChannelAdministratorAllowed(t *testing.T) {
	f := newFixture(t)
	chID, oaID := f.insertWorkspaceChannel(t)
	f.allowAll(oaID, userNode)
	f.read.allow[grant{userNode, f.withMgmtOA(), ngac.OpManage}] = true

	_, err := f.svc.SendMessage(f.callerCtx(), domain.SendMessageInput{
		ChannelID: chID, SenderID: f.userID, SenderNodeID: userNode, Content: "cả nhà", Mentions: []string{domain.MentionAll},
	})

	require.NoError(t, err)
}

func TestMentionAll_WorkspaceChannelNonAdministratorEditRefused(t *testing.T) {
	f := newFixture(t)
	chID, oaID := f.insertWorkspaceChannel(t)
	f.allowAll(oaID, userNode)
	f.withMgmtOA() // exists, but the caller holds nothing on it
	msg, err := f.svc.SendMessage(f.callerCtx(), domain.SendMessageInput{
		ChannelID: chID, SenderID: f.userID, SenderNodeID: userNode, Content: "chào",
	})
	require.NoError(t, err)

	_, err = f.svc.EditMessage(f.callerCtx(), msg.Id, f.userID, userNode, "chào @Tất cả", "markdown", []string{domain.MentionAll})

	require.ErrorIs(t, err, domain.ErrAccessDenied)
}

// ---------------------------------------------------------------------------
// Doubles
// ---------------------------------------------------------------------------

// failingAssignWrite refuses every CreateAssignment.
type failingAssignWrite struct{ *recordingPolicyWrite }

func (failingAssignWrite) CreateAssignment(context.Context, *policypb.CreateAssignmentRequest, ...grpc.CallOption) (*policypb.Assignment, error) {
	return nil, fmt.Errorf("policy service unavailable")
}

// withPolicyWrite rebuilds the service on another policy writer.
func (f *fixture) withPolicyWrite(w policypb.PolicyWriteServiceClient) {
	f.svc = domain.NewService(store.NewStore(f.pool), f.read, w, stubAuth{}, nil)
}
