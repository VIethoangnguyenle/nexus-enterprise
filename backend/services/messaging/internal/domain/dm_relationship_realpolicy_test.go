//go:build realpolicy

package domain_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"ngac-platform/ngac"
	authpb "ngac-platform/proto/auth"
	pb "ngac-platform/proto/messaging"
	policypb "ngac-platform/proto/policy"
	"ngac-platform/services/messaging/internal/domain"
	"ngac-platform/services/messaging/internal/store"
	"ngac-platform/testutil"
)

// Direct conversations and calls are for people who share a workspace. A DM hangs
// under PC_Global, so the graph alone keeps granting its two members write for as
// long as the DM exists; the relationship is what these tests hold the service to.
// Every deny is paired with an allow on the same fixture.

// rpDBAuth answers the auth service's user lookups from the users table the
// fixture inserts into, so the DM target's node is the one the table holds.
type rpDBAuth struct {
	authpb.AuthServiceClient
	pool *pgxpool.Pool
}

func (a rpDBAuth) GetUserByID(ctx context.Context, r *authpb.GetUserByIDRequest, _ ...grpc.CallOption) (*authpb.UserInfo, error) {
	var u authpb.UserInfo
	err := a.pool.QueryRow(ctx, `SELECT id, username, COALESCE(ngac_node, ''), COALESCE(display_name, '') FROM users WHERE id = $1`, r.UserId).
		Scan(&u.Id, &u.Username, &u.NgacNodeId, &u.DisplayName)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, status.Error(codes.NotFound, "user not found")
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (a rpDBAuth) GetUserByNGACNodeID(_ context.Context, r *authpb.GetUserByNGACNodeIDRequest, _ ...grpc.CallOption) (*authpb.UserInfo, error) {
	return &authpb.UserInfo{Id: "u-" + r.NgacNodeId, Username: r.NgacNodeId, NgacNodeId: r.NgacNodeId}, nil
}

// dmWorld is the rpWorld with the real user lookup and a media server.
type dmWorld struct {
	*rpWorld
	media *fakeMedia
}

func newDMWorld(t *testing.T) *dmWorld {
	t.Helper()
	w := newRPWorld(t)
	w.svc = domain.NewService(store.NewStore(w.pool), w.read, w.write, rpDBAuth{pool: w.pool}, nil)
	d := &dmWorld{rpWorld: w, media: newFakeMedia()}
	w.svc.SetCallMedia(d.media)
	return d
}

// openDM opens a DM through the service, as a does, and removes what it made.
func (w *dmWorld) openDM(a, b rpPerson) (*pb.Channel, error) {
	w.t.Helper()
	ch, err := w.svc.FindOrCreateDM(w.as(a), a.id, a.node, b.id, b.node)
	if err == nil {
		w.cleanChannel(ch)
	}
	return ch, err
}

func (w *dmWorld) cleanChannel(ch *pb.Channel) {
	w.t.Cleanup(func() {
		ctx := context.Background()
		w.pool.Exec(ctx, `DELETE FROM channels WHERE id = $1`, ch.Id)
		for _, n := range []string{ch.NgacOaId, ch.NgacUaId} {
			if n != "" {
				w.write.DeleteNode(ctx, &policypb.DeleteNodeRequest{NodeId: n})
			}
		}
	})
}

// mustDM opens a DM that is expected to open.
func (w *dmWorld) mustDM(a, b rpPerson) *pb.Channel {
	w.t.Helper()
	ch, err := w.openDM(a, b)
	require.NoError(w.t, err)
	return ch
}

// leaveWorkspace is what removing someone from the workspace does to the graph.
func (w *dmWorld) leaveWorkspace(p rpPerson) {
	w.t.Helper()
	testutil.Unassign(w.t, w.write, p.node, w.g.Members)
}

func (w *dmWorld) dmsOf(p rpPerson) int {
	w.t.Helper()
	var n int
	require.NoError(w.t, w.pool.QueryRow(context.Background(),
		`SELECT count(*) FROM channel_members WHERE ngac_node_id = $1`, p.node).Scan(&n))
	return n
}

func (w *dmWorld) send(p rpPerson, chID, text string) (*pb.Message, error) {
	return w.svc.SendMessage(w.as(p), domain.SendMessageInput{
		ChannelID: chID, SenderID: p.id, SenderNodeID: p.node, Content: text,
	})
}

func (w *dmWorld) mustSend(p rpPerson, chID, text string) *pb.Message {
	w.t.Helper()
	m, err := w.send(p, chID, text)
	require.NoError(w.t, err)
	return m
}

func (w *dmWorld) readable(p rpPerson, chID string) bool {
	w.t.Helper()
	_, err := w.svc.GetMessages(w.as(p), chID, p.id, p.node, "", 20)
	return err == nil
}

// --- Creating a DM ---

func TestRealPolicy_CreateDMNeedsASharedWorkspace(t *testing.T) {
	w := newDMWorld(t)

	// Allow: two members of the same workspace.
	ch, err := w.openDM(w.creator, w.member)
	require.NoError(t, err)
	assert.Equal(t, "dm", ch.ChannelType)
	assert.Equal(t, 1, w.dmsOf(w.member), "the other person is in the conversation")

	// Deny: a person of another workspace, and no DM is left behind.
	_, err = w.openDM(w.creator, w.outsider)
	assert.ErrorIs(t, err, domain.ErrAccessDenied, "another workspace's person")
	assert.Equal(t, 0, w.dmsOf(w.outsider), "nothing was created for them")

	// Deny: someone who reaches the workspace only through a group's UA is not a
	// workspace-mate; the same person, once a member, is.
	group := w.createGroup(w.creator, "dm-rel-group")
	chanOnly := w.person(group.NgacUaId)
	_, err = w.openDM(w.creator, chanOnly)
	assert.ErrorIs(t, err, domain.ErrAccessDenied, "a channel is not a workspace")
	assert.Equal(t, 0, w.dmsOf(chanOnly))
	testutil.Assign(t, w.write, chanOnly.node, w.g.Members)
	_, err = w.openDM(w.creator, chanOnly)
	require.NoError(t, err, "the same person, now a member")
}

func TestRealPolicy_CreateDMTargetIsResolvedByTheServer(t *testing.T) {
	w := newDMWorld(t)

	// Allow: the ids agree.
	_, err := w.openDM(w.creator, w.member)
	require.NoError(t, err)

	// Deny: a node the client sends that is not the user's.
	_, err = w.svc.FindOrCreateDM(w.as(w.creator), w.creator.id, w.creator.node, w.member.id, w.outsider.node)
	assert.ErrorIs(t, err, domain.ErrInvalidInput, "a node that is not the target's")
	assert.Equal(t, 0, w.dmsOf(w.outsider))

	// Allow: no node sent at all; the server derives it.
	ch, err := w.svc.FindOrCreateDM(w.as(w.creator), w.creator.id, w.creator.node, w.latecomer.id, "")
	require.NoError(t, err)
	w.cleanChannel(ch)
	assert.Equal(t, 1, w.dmsOf(w.latecomer))

	// Deny: oneself, no one, and someone who does not exist.
	_, err = w.svc.FindOrCreateDM(w.as(w.creator), w.creator.id, w.creator.node, w.creator.id, w.creator.node)
	assert.ErrorIs(t, err, domain.ErrInvalidInput, "a conversation with oneself")
	_, err = w.svc.FindOrCreateDM(w.as(w.creator), w.creator.id, w.creator.node, "", w.member.node)
	assert.ErrorIs(t, err, domain.ErrInvalidInput, "a node alone names no person")
	_, err = w.svc.FindOrCreateDM(w.as(w.creator), w.creator.id, w.creator.node, "rp-no-such-user", "")
	assert.ErrorIs(t, err, domain.ErrNotFound)
}

// A user row whose node is a UA (one that even sits under the workspace's Members)
// is not a person: a DM with it would put a whole group into a conversation.
func TestRealPolicy_CreateDMTargetMustBeAPerson(t *testing.T) {
	w := newDMWorld(t)
	group := w.createGroup(w.creator, "dm-rel-ua")

	asUA := func(node string) rpPerson {
		id := "rp-user-" + uuid.NewString()
		_, err := w.pool.Exec(context.Background(),
			`INSERT INTO users (id, username, password, ngac_node, display_name) VALUES ($1, $1, '', $2, $1)`, id, node)
		require.NoError(t, err)
		t.Cleanup(func() { w.pool.Exec(context.Background(), `DELETE FROM users WHERE id = $1`, id) })
		return rpPerson{id: id, node: node}
	}
	for name, ua := range map[string]string{"the Owners UA": w.g.Owners, "a group's UA": group.NgacUaId, "the Members UA": w.g.Members} {
		p := asUA(ua)
		_, err := w.openDM(w.creator, p)
		assert.ErrorIs(t, err, domain.ErrAccessDenied, name)
		assert.Equal(t, 0, w.dmsOf(p), "no DM for "+name)
	}
	_, err := w.openDM(w.creator, w.member)
	require.NoError(t, err, "a person is accepted on the same fixture")
}

// Opening an existing DM is the same question as creating one: it answers nothing
// about conversations the caller has no standing to ask for.
func TestRealPolicy_ExistingDMIsNotOpenedAcrossNoWorkspace(t *testing.T) {
	w := newDMWorld(t)
	ch := w.mustDM(w.creator, w.member)

	again, err := w.openDM(w.creator, w.member)
	require.NoError(t, err)
	assert.Equal(t, ch.Id, again.Id, "while they share a workspace the same DM is found")

	w.leaveWorkspace(w.member)
	_, err = w.openDM(w.creator, w.member)
	assert.ErrorIs(t, err, domain.ErrAccessDenied, "no workspace in common: nothing is found or created")
}

// --- Every write path of an existing DM ---

type dmPath struct {
	name string
	// prep builds what the path needs while the two still share a workspace and
	// returns the operation. The operation is run once before and once after the
	// relationship ends, with attempt 0 and 1: prep readies two of whatever it uses.
	prep func(t *testing.T, w *dmWorld, dm *pb.Channel) func(attempt int) error
}

func dmPaths() []dmPath {
	messages := func(w *dmWorld, dm *pb.Channel, from rpPerson) [2]*pb.Message {
		return [2]*pb.Message{w.mustSend(from, dm.Id, "one"), w.mustSend(from, dm.Id, "two")}
	}
	startCall := func(t *testing.T, w *dmWorld, dm *pb.Channel, by rpPerson) string {
		t.Helper()
		join, _, err := w.svc.StartCall(w.as(by), domain.StartCallInput{ChannelID: dm.Id, UserID: by.id, UserNodeID: by.node, Kind: "audio"})
		require.NoError(t, err)
		return join.Call.Id
	}
	return []dmPath{
		{"send a message", func(t *testing.T, w *dmWorld, dm *pb.Channel) func(int) error {
			return func(int) error { _, err := w.send(w.creator, dm.Id, "hello"); return err }
		}},
		{"send a message typed as a system line", func(t *testing.T, w *dmWorld, dm *pb.Channel) func(int) error {
			return func(int) error {
				_, err := w.svc.SendMessage(w.as(w.creator), domain.SendMessageInput{
					ChannelID: dm.Id, SenderID: w.creator.id, SenderNodeID: w.creator.node, Content: "hello", MessageType: "system",
				})
				return err
			}
		}},
		{"create a poll", func(t *testing.T, w *dmWorld, dm *pb.Channel) func(int) error {
			return func(int) error {
				_, err := w.svc.CreatePoll(w.as(w.creator), domain.CreatePollInput{
					ChannelID: dm.Id, UserID: w.creator.id, UserNodeID: w.creator.node, Question: "Lunch?", Options: []string{"yes", "no"},
				})
				return err
			}
		}},
		{"vote in a poll", func(t *testing.T, w *dmWorld, dm *pb.Channel) func(int) error {
			poll, err := w.svc.CreatePoll(w.as(w.member), domain.CreatePollInput{
				ChannelID: dm.Id, UserID: w.member.id, UserNodeID: w.member.node, Question: "Lunch?", Options: []string{"yes", "no"}, IsMulti: true,
			})
			require.NoError(t, err)
			return func(i int) error {
				return w.svc.VotePoll(w.as(w.creator), poll.Id, poll.Options[i].Id, w.creator.node, w.creator.id)
			}
		}},
		{"create a task", func(t *testing.T, w *dmWorld, dm *pb.Channel) func(int) error {
			return func(int) error {
				_, err := w.svc.CreateTask(w.as(w.creator), domain.CreateTaskInput{
					ChannelID: dm.Id, UserID: w.creator.id, UserNodeID: w.creator.node, Title: "Send the file",
				})
				return err
			}
		}},
		{"retitle a task", func(t *testing.T, w *dmWorld, dm *pb.Channel) func(int) error {
			task, err := w.svc.CreateTask(w.as(w.creator), domain.CreateTaskInput{
				ChannelID: dm.Id, UserID: w.creator.id, UserNodeID: w.creator.node, Title: "Send the file",
			})
			require.NoError(t, err)
			return func(i int) error {
				_, err := w.svc.UpdateTask(w.as(w.creator), domain.UpdateTaskInput{
					TaskID: task.Id, UserID: w.creator.id, UserNodeID: w.creator.node, Title: fmt.Sprintf("Send the file %d", i),
				})
				return err
			}
		}},
		{"set a task's status as its assignee", func(t *testing.T, w *dmWorld, dm *pb.Channel) func(int) error {
			task, err := w.svc.CreateTask(w.as(w.member), domain.CreateTaskInput{
				ChannelID: dm.Id, UserID: w.member.id, UserNodeID: w.member.node, Title: "Send the file", AssigneeID: w.creator.id,
			})
			require.NoError(t, err)
			return func(i int) error {
				_, err := w.svc.UpdateTask(w.as(w.creator), domain.UpdateTaskInput{
					TaskID: task.Id, UserID: w.creator.id, UserNodeID: w.creator.node, Status: []string{"done", "todo"}[i],
				})
				return err
			}
		}},
		{"hand a task to someone as its creator", func(t *testing.T, w *dmWorld, dm *pb.Channel) func(int) error {
			task, err := w.svc.CreateTask(w.as(w.creator), domain.CreateTaskInput{
				ChannelID: dm.Id, UserID: w.creator.id, UserNodeID: w.creator.node, Title: "Send the file",
			})
			require.NoError(t, err)
			return func(i int) error {
				_, err := w.svc.UpdateTask(w.as(w.creator), domain.UpdateTaskInput{
					TaskID: task.Id, UserID: w.creator.id, UserNodeID: w.creator.node, AssigneeID: []string{w.member.id, w.creator.id}[i],
				})
				return err
			}
		}},
		{"react to a message", func(t *testing.T, w *dmWorld, dm *pb.Channel) func(int) error {
			m := messages(w, dm, w.member)
			return func(i int) error {
				return w.svc.AddReaction(w.as(w.creator), m[i].Id, w.creator.node, w.creator.id, "👍")
			}
		}},
		{"pin a message", func(t *testing.T, w *dmWorld, dm *pb.Channel) func(int) error {
			m := messages(w, dm, w.member)
			return func(i int) error {
				return w.svc.PinMessage(w.as(w.creator), dm.Id, m[i].Id, w.creator.node, w.creator.id)
			}
		}},
		{"edit one's own message", func(t *testing.T, w *dmWorld, dm *pb.Channel) func(int) error {
			m := messages(w, dm, w.creator)
			return func(i int) error {
				_, err := w.svc.EditMessage(w.as(w.creator), m[i].Id, w.creator.id, w.creator.node, "edited", "", nil)
				return err
			}
		}},
		{"start a call", func(t *testing.T, w *dmWorld, dm *pb.Channel) func(int) error {
			// Each attempt starts one: the first is ended in between so a second may open.
			return func(int) error {
				join, _, err := w.svc.StartCall(w.as(w.creator), domain.StartCallInput{ChannelID: dm.Id, UserID: w.creator.id, UserNodeID: w.creator.node, Kind: "audio"})
				if err == nil {
					require.NoError(t, w.svc.HandleCallEvent(context.Background(), domain.CallEvent{Kind: domain.CallEventRoomFinished, Room: join.Call.Id}))
				}
				return err
			}
		}},
		{"start a call into one already open", func(t *testing.T, w *dmWorld, dm *pb.Channel) func(int) error {
			startCall(t, w, dm, w.member)
			return func(int) error {
				_, _, err := w.svc.StartCall(w.as(w.creator), domain.StartCallInput{ChannelID: dm.Id, UserID: w.creator.id, UserNodeID: w.creator.node, Kind: "audio"})
				return err
			}
		}},
		{"take a ticket into a call", func(t *testing.T, w *dmWorld, dm *pb.Channel) func(int) error {
			id := startCall(t, w, dm, w.member)
			return func(int) error {
				_, err := w.svc.CallToken(w.as(w.creator), id, w.creator.id, w.creator.node)
				return err
			}
		}},
		{"invite to a call", func(t *testing.T, w *dmWorld, dm *pb.Channel) func(int) error {
			id := startCall(t, w, dm, w.creator)
			return func(int) error {
				return w.svc.InviteToCall(w.as(w.creator), id, w.creator.id, w.creator.node, []string{w.member.id})
			}
		}},
	}
}

// Every way of writing to a DM is refused once the two no longer share a
// workspace, though the graph still grants write; the same operation, by the same
// person in the same DM, worked a moment before. Reading the history still works.
func TestRealPolicy_DMWritesNeedASharedWorkspace(t *testing.T) {
	for _, p := range dmPaths() {
		t.Run(p.name, func(t *testing.T) {
			w := newDMWorld(t)
			dm := w.mustDM(w.creator, w.member)
			op := p.prep(t, w, dm)

			require.NoError(t, op(0), "allowed while they share a workspace")

			w.leaveWorkspace(w.member)
			require.True(t, testutil.Allowed(t, w.read, w.creator.node, dm.NgacOaId, ngac.OpWrite),
				"the graph alone still grants write: it is the relationship that must refuse")
			err := op(1)
			assert.ErrorIs(t, err, domain.ErrAccessDenied, "refused once they share none")

			assert.True(t, w.readable(w.creator, dm.Id), "the history stays readable")
			assert.True(t, w.readable(w.member, dm.Id), "for the person who left, too")
		})
	}
}

// The relationship is the existence of a shared workspace, not one in particular,
// and it is judged now: coming back to a workspace in common restores the DM.
func TestRealPolicy_DMSurvivesOnAnyOtherSharedWorkspace(t *testing.T) {
	w := newDMWorld(t)
	dm := w.mustDM(w.creator, w.member)
	w.mustSend(w.creator, dm.Id, "before")

	other := testutil.SeedWorkspaceGraph(t, w.write)
	w.leaveWorkspace(w.member)
	_, err := w.send(w.creator, dm.Id, "alone")
	require.ErrorIs(t, err, domain.ErrAccessDenied)

	testutil.Assign(t, w.write, w.creator.node, other.Members)
	testutil.Assign(t, w.write, w.member.node, other.Members)
	_, err = w.send(w.creator, dm.Id, "in the other workspace")
	require.NoError(t, err, "they now share another workspace")

	testutil.Unassign(t, w.write, w.member.node, other.Members)
	_, err = w.send(w.creator, dm.Id, "gone again")
	assert.ErrorIs(t, err, domain.ErrAccessDenied)
}

// --- Calls in progress ---

func TestRealPolicy_ExColleagueIsEvictedFromADMCall(t *testing.T) {
	w := newDMWorld(t)
	dm := w.mustDM(w.creator, w.member)
	ctx := context.Background()

	join, _, err := w.svc.StartCall(w.as(w.creator), domain.StartCallInput{ChannelID: dm.Id, UserID: w.creator.id, UserNodeID: w.creator.node, Kind: "audio"})
	require.NoError(t, err)
	callID := join.Call.Id
	_, err = w.svc.CallToken(w.as(w.member), callID, w.member.id, w.member.node)
	require.NoError(t, err)
	for _, p := range []rpPerson{w.creator, w.member} {
		require.NoError(t, w.svc.HandleCallEvent(ctx, domain.CallEvent{Kind: domain.CallEventJoined, Room: callID, Identity: p.id}))
	}

	// Allow: while they share a workspace, the sweep leaves both in.
	require.NoError(t, w.svc.EvictLostCallParticipants(ctx, dm.Id, ""))
	assert.Empty(t, w.media.removed, "nobody is put out while the relationship holds")

	// Deny: the same sweep, once the relationship is over.
	w.leaveWorkspace(w.member)
	require.NoError(t, w.svc.EvictLostCallParticipants(ctx, dm.Id, ""))
	assert.ElementsMatch(t, []string{callID + "/" + w.creator.id, callID + "/" + w.member.id}, w.media.removed,
		"neither may stay in a call with someone they no longer share a workspace with")

	// And nobody can walk back in through the media server's webhook.
	w.media.removed = nil
	require.NoError(t, w.svc.HandleCallEvent(ctx, domain.CallEvent{Kind: domain.CallEventJoined, Room: callID, Identity: w.member.id}))
	assert.Equal(t, []string{callID + "/" + w.member.id}, w.media.removed)
}

// --- Event calls ---

// eventCallFixture is a calendar event with a call, readable through a guest UA that
// sits beneath the workspace's policy class but not beneath its Members UA, as an
// event's guest list does.
func (w *dmWorld) eventCallFixture(guests ...rpPerson) string {
	w.t.Helper()
	ctx := context.Background()
	id := uuid.NewString()
	calOA := testutil.NewNode(w.t, w.write, "rp-cal-"+id, ngac.TypeOA, w.g.Documents)
	evOA := testutil.NewNode(w.t, w.write, "rp-ev-"+id, ngac.TypeOA, calOA)
	guestUA := testutil.NewNode(w.t, w.write, "rp-guests-"+id, ngac.TypeUA, w.g.PC)
	testutil.Associate(w.t, w.write, guestUA, evOA, []string{ngac.OpRead})
	for _, g := range guests {
		testutil.Assign(w.t, w.write, g.node, guestUA)
	}
	calID := "rp-calid-" + id
	_, err := w.pool.Exec(ctx, `INSERT INTO calendars (id, workspace_id, kind, owner_node_id, ngac_oa_id) VALUES ($1, $2, 'personal', $3, $4)`,
		calID, w.g.WorkspaceID, w.owner.node, calOA)
	require.NoError(w.t, err)
	eventID := "rp-event-" + id
	_, err = w.pool.Exec(ctx,
		`INSERT INTO calendar_events (id, workspace_id, calendar_id, organizer_node_id, title, starts_at, ends_at, has_call, ngac_oa_id)
		 VALUES ($1, $2, $3, $4, 'Standup', now(), now() + interval '1 hour', TRUE, $5)`,
		eventID, w.g.WorkspaceID, calID, w.owner.node, evOA)
	require.NoError(w.t, err)
	w.t.Cleanup(func() {
		w.pool.Exec(ctx, `DELETE FROM calls WHERE calendar_event_id = $1`, eventID)
		w.pool.Exec(ctx, `DELETE FROM calendar_events WHERE id = $1`, eventID)
		w.pool.Exec(ctx, `DELETE FROM calendars WHERE id = $1`, calID)
	})
	return eventID
}

func (w *dmWorld) joinEventCall(p rpPerson, eventID string) (*pb.CallJoin, error) {
	join, _, err := w.svc.StartEventCall(w.as(p), domain.StartEventCallInput{EventID: eventID, UserID: p.id, UserNodeID: p.node})
	return join, err
}

func TestRealPolicy_EventCallNeedsWorkspaceMembership(t *testing.T) {
	w := newDMWorld(t)
	eventID := w.eventCallFixture(w.member, w.latecomer)

	// Allow: a guest who is a member of the workspace.
	join, err := w.joinEventCall(w.member, eventID)
	require.NoError(t, err)
	require.True(t, testutil.Allowed(t, w.read, w.latecomer.node, w.evOA(eventID), ngac.OpRead))

	// Deny: the same guest, removed from the workspace, still on the guest UA and
	// so still reading the event in the graph, holds neither a new nor an existing call.
	w.leaveWorkspace(w.latecomer)
	require.True(t, testutil.Allowed(t, w.read, w.latecomer.node, w.evOA(eventID), ngac.OpRead),
		"the graph alone still lets the removed guest read the event")
	_, err = w.joinEventCall(w.latecomer, eventID)
	assert.ErrorIs(t, err, domain.ErrAccessDenied, "a removed member does not join the event's call")
	_, err = w.svc.CallToken(w.as(w.latecomer), join.Call.Id, w.latecomer.id, w.latecomer.node)
	assert.ErrorIs(t, err, domain.ErrAccessDenied, "nor take a ticket into the running one")

	// The guest who stayed is unaffected.
	_, err = w.svc.CallToken(w.as(w.member), join.Call.Id, w.member.id, w.member.node)
	require.NoError(t, err)
}

// evOA is the node a guest reads to be in the event's call.
func (w *dmWorld) evOA(eventID string) string {
	var oa string
	require.NoError(w.t, w.pool.QueryRow(context.Background(), `SELECT ngac_oa_id FROM calendar_events WHERE id = $1`, eventID).Scan(&oa))
	return oa
}

// Backing out of a conversation stays possible for someone who no longer shares a
// workspace with the other: declining or cancelling a call, deleting one's own
// message, and taking back a reaction or a vote.
func TestRealPolicy_BackingOutOfADMStaysAllowed(t *testing.T) {
	w := newDMWorld(t)
	dm := w.mustDM(w.creator, w.member)
	own := w.mustSend(w.creator, dm.Id, "mine")
	theirs := w.mustSend(w.member, dm.Id, "theirs")
	require.NoError(t, w.svc.AddReaction(w.as(w.creator), theirs.Id, w.creator.node, w.creator.id, "👍"))
	poll, err := w.svc.CreatePoll(w.as(w.member), domain.CreatePollInput{
		ChannelID: dm.Id, UserID: w.member.id, UserNodeID: w.member.node, Question: "Lunch?", Options: []string{"yes", "no"},
	})
	require.NoError(t, err)
	require.NoError(t, w.svc.VotePoll(w.as(w.creator), poll.Id, poll.Options[0].Id, w.creator.node, w.creator.id))
	declined, _, err := w.svc.StartCall(w.as(w.member), domain.StartCallInput{ChannelID: dm.Id, UserID: w.member.id, UserNodeID: w.member.node, Kind: "audio"})
	require.NoError(t, err)
	// A second conversation, in which the person who rang takes the call back.
	dm2 := w.mustDM(w.creator, w.latecomer)
	rung, _, err := w.svc.StartCall(w.as(w.creator), domain.StartCallInput{ChannelID: dm2.Id, UserID: w.creator.id, UserNodeID: w.creator.node, Kind: "audio"})
	require.NoError(t, err)

	w.leaveWorkspace(w.member)
	w.leaveWorkspace(w.latecomer)

	_, err = w.svc.DeclineCall(w.as(w.creator), declined.Call.Id, w.creator.id, w.creator.node)
	assert.NoError(t, err, "turning a call down")
	_, err = w.svc.DeleteMessage(w.as(w.creator), own.Id, w.creator.id, w.creator.node)
	assert.NoError(t, err, "deleting one's own message")
	assert.NoError(t, w.svc.RemoveReaction(w.as(w.creator), theirs.Id, w.creator.node, w.creator.id, "👍"), "taking a reaction back")
	assert.NoError(t, w.svc.RemoveVote(w.as(w.creator), poll.Id, poll.Options[0].Id, w.creator.node, w.creator.id), "taking a vote back")

	_, err = w.svc.CancelCall(w.as(w.creator), rung.Call.Id, w.creator.id, w.creator.node)
	assert.NoError(t, err, "taking back a call one rang")
}

// A manager who is not in a group deletes a member's message there: the deletion
// is recorded as a line in the group even though the manager cannot write in it.
func TestRealPolicy_OwnerOutsideAGroupModeratesIt(t *testing.T) {
	w := newDMWorld(t)
	g := w.createGroup(w.creator, "dm-rel-moderated")
	require.NoError(t, w.svc.AddMember(w.as(w.creator), g.Id, w.creator.node, w.member.node))
	require.NoError(t, w.svc.AddMember(w.as(w.creator), g.Id, w.creator.node, w.latecomer.node))
	msg := w.mustSend(w.member, g.Id, "to be removed")

	// Deny: a member of the group who does not manage it cannot delete another's message.
	_, err := w.svc.DeleteMessage(w.as(w.latecomer), msg.Id, w.latecomer.id, w.latecomer.node)
	assert.ErrorIs(t, err, domain.ErrAccessDenied)

	// Allow: the owner holds manage there, and no write.
	require.False(t, testutil.Allowed(t, w.read, w.owner.node, g.NgacOaId, ngac.OpWrite), "the owner cannot write in the group")
	out, err := w.svc.DeleteMessage(w.as(w.owner), msg.Id, w.owner.id, w.owner.node)
	require.NoError(t, err)
	assert.NotNil(t, out.Event, "the deletion is announced")
	require.NotNil(t, out.Notice, "and recorded in the group")
	assert.Equal(t, "system", out.Notice.MessageType)
}
