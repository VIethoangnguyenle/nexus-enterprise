//go:build realpolicy

package domain_test

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	"ngac-platform/pkg/grpcauth"
	"ngac-platform/pkg/realtime"
	authpb "ngac-platform/proto/auth"
	pb "ngac-platform/proto/messaging"
	policypb "ngac-platform/proto/policy"
	"ngac-platform/services/messaging/internal/domain"
	"ngac-platform/services/messaging/internal/store"
	"ngac-platform/testutil"
)

// These tests run the messaging domain against the real policy service (see
// backend/testutil/policy_process.go): a group is private because the real PDP
// says so, not because a scripted fake was told to. Graph fixtures are built only
// through the policy's write API; SQL is used for business rows alone.

func TestMain(m *testing.M) { os.Exit(testutil.RunMain(m)) }

type rpPerson struct{ id, node string }

type rpNamedAuth struct{ authpb.AuthServiceClient }

func (rpNamedAuth) GetUserByNGACNodeID(_ context.Context, r *authpb.GetUserByNGACNodeIDRequest, _ ...grpc.CallOption) (*authpb.UserInfo, error) {
	return &authpb.UserInfo{Id: "u-" + r.NgacNodeId, Username: r.NgacNodeId, NgacNodeId: r.NgacNodeId}, nil
}

func (rpNamedAuth) GetUserByID(_ context.Context, r *authpb.GetUserByIDRequest, _ ...grpc.CallOption) (*authpb.UserInfo, error) {
	return &authpb.UserInfo{Id: r.UserId, Username: r.UserId}, nil
}

// rpWorld is one workspace in the real policy: an owner, a member who starts the
// groups, another member, a member who is never put into #general, and a person
// of another workspace.
type rpWorld struct {
	t     *testing.T
	pool  *pgxpool.Pool
	read  policypb.PolicyReadServiceClient
	write policypb.PolicyWriteServiceClient
	svc   *domain.Service
	g     testutil.WorkspaceGraph

	owner, creator, member, latecomer, outsider rpPerson
}

func (w *rpWorld) person(parents ...string) rpPerson {
	w.t.Helper()
	node := testutil.NewUser(w.t, w.write, parents...)
	id := "rp-user-" + uuid.NewString()
	_, err := w.pool.Exec(context.Background(),
		`INSERT INTO users (id, username, password, ngac_node, display_name) VALUES ($1, $1, '', $2, $1)`, id, node)
	require.NoError(w.t, err)
	w.t.Cleanup(func() { w.pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, id) })
	return rpPerson{id: id, node: node}
}

func newRPWorld(t *testing.T) *rpWorld {
	t.Helper()
	ctx := context.Background()
	read, write := testutil.StartPolicy(t)
	pool := testutil.SetupTestDB(t)
	w := &rpWorld{t: t, pool: pool, read: read, write: write}
	w.g = testutil.SeedWorkspaceGraph(t, write)
	other := testutil.SeedWorkspaceGraph(t, write)

	w.owner = w.person(w.g.Owners)
	w.creator = w.person(w.g.Members)
	w.member = w.person(w.g.Members)
	w.latecomer = w.person(w.g.Members)
	w.outsider = w.person(other.Members)

	_, err := pool.Exec(ctx, `INSERT INTO workspaces (id, name, owner_id, ngac_pc_id, type) VALUES ($1, 'rp', $2, $3, 'organization')`,
		w.g.WorkspaceID, w.owner.id, w.g.PC)
	require.NoError(t, err)
	t.Cleanup(func() {
		// Channels and the nodes messaging made for them go before the workspace.
		rows, qerr := pool.Query(ctx, `SELECT COALESCE(ngac_oa_id,''), COALESCE(ngac_ua_id,'') FROM channels WHERE workspace_id = $1`, w.g.WorkspaceID)
		var nodes []string
		if qerr == nil {
			for rows.Next() {
				var oa, ua string
				if rows.Scan(&oa, &ua) == nil {
					nodes = append(nodes, oa, ua)
				}
			}
			rows.Close()
		}
		pool.Exec(ctx, `DELETE FROM channels WHERE workspace_id = $1`, w.g.WorkspaceID)
		pool.Exec(ctx, `DELETE FROM workspaces WHERE id = $1`, w.g.WorkspaceID)
		for _, n := range nodes {
			if n != "" {
				w.write.DeleteNode(ctx, &policypb.DeleteNodeRequest{NodeId: n})
			}
		}
	})

	w.svc = domain.NewService(store.NewStore(pool), read, write, rpNamedAuth{}, nil)
	return w
}

// as returns the context the REST edge would build for the person.
func (w *rpWorld) as(p rpPerson) context.Context {
	return grpcauth.WithCaller(context.Background(), grpcauth.Caller{UserID: p.id, NGACNodeID: p.node, TenantID: w.g.WorkspaceID})
}

// join is what the auth and workspace services ask when they admit someone.
func (w *rpWorld) join(p rpPerson) error {
	return w.svc.JoinWorkspaceChannel(context.Background(), w.g.WorkspaceID, p.id, p.node)
}

func (w *rpWorld) createGroup(p rpPerson, name string) *pb.Channel {
	w.t.Helper()
	ch, err := w.svc.CreateChannel(w.as(p), domain.CreateChannelInput{
		Name: name, WorkspaceID: w.g.WorkspaceID, UserID: p.id, UserNodeID: p.node, ChannelType: "workspace",
	})
	require.NoError(w.t, err)
	return ch
}

func (w *rpWorld) listed(p rpPerson, chID string) bool {
	w.t.Helper()
	chs, err := w.svc.ListChannels(w.as(p), w.g.WorkspaceID, p.node)
	require.NoError(w.t, err)
	for _, c := range chs {
		if c.Id == chID {
			return true
		}
	}
	return false
}

// A workspace member who is not in a group cannot see it, read it, post in it,
// list its people or get a ticket into its call; the member who made it can do all
// of that. An owner outside the group moderates it (manage) and nothing else.
func TestRealPolicy_PrivateGroupIsClosedToWorkspaceMembersOutsideIt(t *testing.T) {
	w := newRPWorld(t)
	g := w.createGroup(w.creator, "rp-private")
	assert.Equal(t, "private", g.ChannelType, "a request for a workspace-wide channel still makes a private group")
	w.svc.SetCallMedia(newFakeMedia())

	// The creator: everything.
	assert.True(t, w.listed(w.creator, g.Id))
	_, err := w.svc.GetChannel(w.as(w.creator), g.Id, w.creator.node)
	require.NoError(t, err)
	_, err = w.svc.SendMessage(w.as(w.creator), domain.SendMessageInput{ChannelID: g.Id, SenderID: w.creator.id, SenderNodeID: w.creator.node, Content: "hello"})
	require.NoError(t, err)
	members, err := w.svc.ListMembers(w.as(w.creator), g.Id, w.creator.node)
	require.NoError(t, err)
	assert.Len(t, members, 1)
	join, _, err := w.svc.StartCall(w.as(w.creator), domain.StartCallInput{ChannelID: g.Id, UserID: w.creator.id, UserNodeID: w.creator.node, Kind: "audio"})
	require.NoError(t, err)
	callID := join.Call.Id

	// Another member of the same workspace, and the workspace's owner: nothing.
	for name, p := range map[string]rpPerson{"member": w.member, "owner": w.owner, "member of another workspace": w.outsider} {
		assert.False(t, w.listed(p, g.Id), name+" must not see the group listed")
		_, err = w.svc.GetChannel(w.as(p), g.Id, p.node)
		assert.ErrorIs(t, err, domain.ErrAccessDenied, name+": GetChannel")
		_, err = w.svc.SendMessage(w.as(p), domain.SendMessageInput{ChannelID: g.Id, SenderID: p.id, SenderNodeID: p.node, Content: "let me in"})
		assert.ErrorIs(t, err, domain.ErrAccessDenied, name+": SendMessage")
		_, err = w.svc.ListMembers(w.as(p), g.Id, p.node)
		assert.ErrorIs(t, err, domain.ErrAccessDenied, name+": ListMembers")
		_, err = w.svc.CallToken(w.as(p), callID, p.id, p.node)
		assert.ErrorIs(t, err, domain.ErrAccessDenied, name+": CallToken")
		err = w.svc.AddMember(w.as(p), g.Id, p.node, p.node)
		assert.ErrorIs(t, err, domain.ErrAccessDenied, name+": adding themselves")
	}

	// The owner moderates: manage is theirs, as the plain member's is not.
	_, err = w.svc.UpdateChannel(w.as(w.owner), g.Id, w.owner.node, "renamed by an owner")
	require.NoError(t, err, "owners hold manage on every group")
	_, err = w.svc.UpdateChannel(w.as(w.member), g.Id, w.member.node, "renamed by a member")
	assert.ErrorIs(t, err, domain.ErrAccessDenied)
	_, err = w.svc.UpdateChannel(w.as(w.creator), g.Id, w.creator.node, "renamed by a group member")
	assert.ErrorIs(t, err, domain.ErrAccessDenied, "being in the group does not confer manage")
}

// Being put into the group is what opens it: before, denied; after, the same
// person reads, is listed, and the group is listed for them.
func TestRealPolicy_AddingAMemberOpensTheGroupToThem(t *testing.T) {
	w := newRPWorld(t)
	g := w.createGroup(w.creator, "rp-added")

	_, err := w.svc.GetChannel(w.as(w.member), g.Id, w.member.node)
	require.ErrorIs(t, err, domain.ErrAccessDenied)
	require.False(t, w.listed(w.member, g.Id))

	require.NoError(t, w.svc.AddMember(w.as(w.creator), g.Id, w.creator.node, w.member.node))

	_, err = w.svc.GetChannel(w.as(w.member), g.Id, w.member.node)
	require.NoError(t, err)
	assert.True(t, w.listed(w.member, g.Id))
	assert.False(t, w.listed(w.latecomer, g.Id), "and a third member is still outside")
	members, err := w.svc.ListMembers(w.as(w.creator), g.Id, w.creator.node)
	require.NoError(t, err)
	assert.Len(t, members, 2)
}

func (w *rpWorld) provisionGeneral() *pb.Channel {
	w.t.Helper()
	ch, err := w.svc.ProvisionWorkspaceChannel(context.Background(), domain.ProvisionWorkspaceChannelInput{
		WorkspaceID: w.g.WorkspaceID, Name: "general", CreatorUserID: w.owner.id, CreatorNodeID: w.owner.node,
	})
	require.NoError(w.t, err)
	require.Equal(w.t, "workspace", ch.ChannelType)
	return ch
}

func (w *rpWorld) rosterNodes(chID string, as rpPerson) []string {
	w.t.Helper()
	members, err := w.svc.ListMembers(w.as(as), chID, as.node)
	require.NoError(w.t, err)
	var out []string
	for _, m := range members {
		out = append(out, m.NgacNodeId)
	}
	return out
}

// #general: nobody is in it by being in the workspace; JoinWorkspaceChannel, which
// admission calls, is what puts a member in. A joined member reads and posts, is on
// the roster, counts in member_count and is a push recipient; someone who never
// joined and someone from another workspace are denied; leaving removes them.
func TestRealPolicy_WorkspaceWideChannelFollowsAdmission(t *testing.T) {
	w := newRPWorld(t)
	general := w.provisionGeneral()
	ctx := context.Background()

	// The creator (the owner) is its first member.
	assert.Equal(t, []string{w.owner.node}, w.rosterNodes(general.Id, w.owner))

	// Before joining, a workspace member is denied: membership of the workspace alone opens nothing.
	_, err := w.svc.GetChannel(w.as(w.member), general.Id, w.member.node)
	require.ErrorIs(t, err, domain.ErrAccessDenied)

	// Admission joins them, and the channel's people are told through events that
	// carry the workspace as their boundary and no person as their cause.
	rec := &realtime.Recorder{}
	w.svc.SetEmitter(rec)
	require.NoError(t, w.join(w.member))
	require.NotEmpty(t, rec.Events(), "joining announces the new member")
	for _, e := range rec.Events() {
		assert.Equal(t, realtime.KindMemberAdded, e.Kind)
		assert.Equal(t, w.g.WorkspaceID, e.TenantID, "an event without its tenant would be dropped")
		assert.Empty(t, e.ActorUserID, "no person caused it; a node id must not pass as a user id")
	}
	w.svc.SetEmitter(nil)
	got, err := w.svc.GetChannel(w.as(w.member), general.Id, w.member.node)
	require.NoError(t, err)
	assert.EqualValues(t, 2, got.MemberCount, "member_count counts the admitted member")
	assert.True(t, w.listed(w.member, general.Id))
	assert.ElementsMatch(t, []string{w.owner.node, w.member.node}, w.rosterNodes(general.Id, w.owner), "the roster lists them")
	_, err = w.svc.SendMessage(w.as(w.member), domain.SendMessageInput{ChannelID: general.Id, SenderID: w.member.id, SenderNodeID: w.member.node, Content: "hi all"})
	require.NoError(t, err)

	posted, err := w.svc.SendMessage(w.as(w.owner), domain.SendMessageInput{ChannelID: general.Id, SenderID: w.owner.id, SenderNodeID: w.owner.node, Content: "welcome"})
	require.NoError(t, err)
	assert.Equal(t, []string{w.member.node}, w.pushedTo(posted), "the admitted member is pushed; the sender is not")

	// Joining twice changes nothing.
	require.NoError(t, w.join(w.member))
	got, err = w.svc.GetChannel(w.as(w.member), general.Id, w.member.node)
	require.NoError(t, err)
	assert.EqualValues(t, 2, got.MemberCount)

	// The member who was never joined is still outside; so is a stranger, and the
	// stranger cannot be joined by asking.
	_, err = w.svc.GetChannel(w.as(w.latecomer), general.Id, w.latecomer.node)
	assert.ErrorIs(t, err, domain.ErrAccessDenied)
	err = w.join(w.outsider)
	assert.ErrorIs(t, err, domain.ErrAccessDenied, "a member of another workspace cannot be put into this workspace's channel")
	_, err = w.svc.GetChannel(w.as(w.outsider), general.Id, w.outsider.node)
	assert.ErrorIs(t, err, domain.ErrAccessDenied)
	// A node that is not a person at all is no member either.
	err = w.svc.JoinWorkspaceChannel(ctx, w.g.WorkspaceID, "not-a-user", w.g.Members)
	assert.ErrorIs(t, err, domain.ErrAccessDenied, "a UA passed as a user is not a member")
	assert.ElementsMatch(t, []string{w.owner.node, w.member.node}, w.rosterNodes(general.Id, w.owner))

	// The late member, once joined, is in.
	require.NoError(t, w.join(w.latecomer))
	_, err = w.svc.GetChannel(w.as(w.latecomer), general.Id, w.latecomer.node)
	require.NoError(t, err)

	// Leaving the workspace strips every UA under it, the channel's included (detachMember);
	// the consumer then drops the cache rows.
	testutil.Unassign(t, w.write, w.member.node, general.NgacUaId)
	testutil.Unassign(t, w.write, w.member.node, w.g.Members)
	require.NoError(t, w.svc.ForgetWorkspaceMember(ctx, w.g.WorkspaceID, w.member.node))
	_, err = w.svc.GetChannel(w.as(w.member), general.Id, w.member.node)
	assert.ErrorIs(t, err, domain.ErrAccessDenied, "removed from the workspace, the member no longer reads #general")
	assert.ElementsMatch(t, []string{w.owner.node, w.latecomer.node}, w.rosterNodes(general.Id, w.owner), "gone from the roster")
	got, err = w.svc.GetChannel(w.as(w.owner), general.Id, w.owner.node)
	require.NoError(t, err)
	assert.EqualValues(t, 2, got.MemberCount, "and from the count")
}

// A workspace has one workspace-wide channel. Whoever asks for a second, in any way
// open to a person, gets none; the one allowed path is refused a second time.
func TestRealPolicy_OnlyOneWorkspaceWideChannelAndOnlyFromProvisioning(t *testing.T) {
	w := newRPWorld(t)
	general := w.provisionGeneral()

	_, err := w.svc.ProvisionWorkspaceChannel(context.Background(), domain.ProvisionWorkspaceChannelInput{
		WorkspaceID: w.g.WorkspaceID, Name: "general-again", CreatorUserID: w.owner.id, CreatorNodeID: w.owner.node,
	})
	assert.ErrorIs(t, err, domain.ErrAlreadyExists)

	// A person's own request, whatever type it names, makes a private group.
	sneaky := w.createGroup(w.owner, "general-too")
	assert.Equal(t, "private", sneaky.ChannelType)

	// A member without create_channel on Channels cannot start a group, and the
	// provisioning path does not lend them the right: the creator is checked too.
	_, err = w.svc.ProvisionWorkspaceChannel(context.Background(), domain.ProvisionWorkspaceChannelInput{
		WorkspaceID: w.g.WorkspaceID, Name: "x", CreatorUserID: w.outsider.id, CreatorNodeID: w.outsider.node,
	})
	assert.Error(t, err)

	var n int
	require.NoError(t, w.pool.QueryRow(context.Background(),
		`SELECT count(*) FROM channels WHERE workspace_id = $1 AND channel_type = 'workspace'`, w.g.WorkspaceID).Scan(&n))
	assert.Equal(t, 1, n)
	got, err := w.svc.GetChannel(w.as(w.owner), general.Id, w.owner.node)
	require.NoError(t, err)
	assert.Equal(t, "workspace", got.ChannelType)
}

func (w *rpWorld) pushedTo(msg *pb.Message) []string {
	w.t.Helper()
	recipients, err := w.svc.PushRecipients(w.as(w.owner), msg)
	require.NoError(w.t, err)
	var out []string
	for _, r := range recipients {
		out = append(out, r.Person.NodeID)
	}
	return out
}

// Joining touches no notification level: an explicit choice survives, and a member who
// never chose keeps the default.
func TestRealPolicy_JoiningLeavesNotificationLevelsAlone(t *testing.T) {
	w := newRPWorld(t)
	general := w.provisionGeneral()
	_, err := w.pool.Exec(context.Background(), `INSERT INTO channel_notification_levels (user_id, channel_id, level) VALUES ($1, $2, 'all')`, w.member.id, general.Id)
	require.NoError(t, err)

	require.NoError(t, w.join(w.member))
	require.NoError(t, w.join(w.latecomer))

	level := func(p rpPerson) string {
		l, err := w.svc.GetNotificationLevel(w.as(p), general.Id, p.id, p.node)
		require.NoError(t, err)
		return l
	}
	assert.Equal(t, "all", level(w.member), "an explicit choice survives joining")
	assert.Equal(t, "main", level(w.latecomer), "no level is written for a newly joined member: the default stands")
}

// A member of the workspace whose #general edge is missing (the admission-time join
// failed, or they were admitted before the channel was reachable) gets it when they list
// their channels; someone who is not a member of the workspace gets nothing.
func TestRealPolicy_ListingChannelsRepairsAMissingWorkspaceChannelEdge(t *testing.T) {
	w := newRPWorld(t)
	general := w.provisionGeneral()

	// Deny: a person of another workspace asks for this workspace's channels.
	chs, err := w.svc.ListChannels(w.as(w.outsider), w.g.WorkspaceID, w.outsider.node)
	require.NoError(t, err)
	assert.Empty(t, chs)
	assert.False(t, w.listed(w.outsider, general.Id))
	_, err = w.svc.GetChannel(w.as(w.outsider), general.Id, w.outsider.node)
	assert.ErrorIs(t, err, domain.ErrAccessDenied, "the stranger was not joined by asking")
	assert.NotContains(t, w.rosterNodes(general.Id, w.owner), w.outsider.node)

	// Allow: a member without the edge sees it listed, and is on its roster afterwards.
	_, err = w.svc.GetChannel(w.as(w.latecomer), general.Id, w.latecomer.node)
	require.ErrorIs(t, err, domain.ErrAccessDenied, "precondition: no edge yet")
	assert.True(t, w.listed(w.latecomer, general.Id), "listing joins a member who lacks the edge")
	_, err = w.svc.GetChannel(w.as(w.latecomer), general.Id, w.latecomer.node)
	require.NoError(t, err)
	assert.Contains(t, w.rosterNodes(general.Id, w.owner), w.latecomer.node)
}
