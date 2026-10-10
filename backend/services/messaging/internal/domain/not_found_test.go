package domain_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"ngac-platform/ngac"
	"ngac-platform/services/messaging/internal/domain"
	"ngac-platform/services/messaging/internal/store"
)

// An unknown channel or message is a refusal the caller may hear (not found),
// never an unclassified failure; a database that cannot answer is the opposite.
func TestUnknownChannelAndMessageAreNotFound(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	err := f.svc.AuthorizeChannelAccess(ctx, "no-such-channel", userNode, ngac.OpRead)
	require.ErrorIs(t, err, domain.ErrNotFound)
	require.Contains(t, err.Error(), "channel not found")

	_, err = f.svc.GetChannel(ctx, "no-such-channel", userNode)
	require.ErrorIs(t, err, domain.ErrNotFound)

	_, err = f.svc.GetThread(ctx, "no-such-message", userNode)
	require.ErrorIs(t, err, domain.ErrNotFound)
	require.Empty(t, f.read.asked(), "an unknown id is refused before policy is asked")
}

func TestLookupFailureIsNotReportedAsNotFound(t *testing.T) {
	f := newFixture(t)
	down, err := pgxpool.New(context.Background(), "postgres://x:x@127.0.0.1:1/x?connect_timeout=1")
	require.NoError(t, err)
	t.Cleanup(down.Close)
	svc := domain.NewService(store.NewStore(down), f.read, f.write, stubAuth{}, nil)

	err = svc.AuthorizeChannelAccess(context.Background(), "any-channel", userNode, ngac.OpRead)
	require.Error(t, err)
	require.NotErrorIs(t, err, domain.ErrNotFound)
	require.NotErrorIs(t, err, domain.ErrAccessDenied)
}

func TestSendAndReadMessagesInAnUnknownChannelAreNotFound(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()

	_, err := f.svc.SendMessage(ctx, domain.SendMessageInput{ChannelID: "no-such-channel", SenderNodeID: userNode, Content: "hi"})
	require.ErrorIs(t, err, domain.ErrNotFound)

	_, err = f.svc.GetMessages(ctx, "no-such-channel", userNode, "", 10)
	require.ErrorIs(t, err, domain.ErrNotFound)
}

func TestGetMessagesRefusesAMalformedCursor(t *testing.T) {
	f := newFixture(t)
	chID, oaID := f.insertChannel(t, "cursor")
	f.read.allow[grant{userNode, oaID, ngac.OpRead}] = true

	_, err := f.svc.GetMessages(context.Background(), chID, userNode, "yesterday", 10)
	require.ErrorIs(t, err, domain.ErrInvalidInput)

	_, err = f.svc.GetMessages(context.Background(), chID, userNode, "2026-10-10T08:00:00Z", 10)
	require.NoError(t, err)
}

// A member list the graph could not answer is an error, never a quietly empty
// room.
func TestListMembersFailsWhenTheGraphCannotAnswer(t *testing.T) {
	f := newFixture(t)
	chID, oaID := f.insertChannel(t, "members-down")
	f.read.allow[grant{userNode, oaID, ngac.OpRead}] = true
	f.read.childrenErr = errors.New("policy down")

	members, err := f.svc.ListMembers(context.Background(), chID, userNode)
	require.Error(t, err)
	require.Nil(t, members)
	require.NotErrorIs(t, err, domain.ErrNotFound)
}

func TestPollAndTaskLookupFailuresAreNotNotFound(t *testing.T) {
	f := newFixture(t)
	down, err := pgxpool.New(context.Background(), "postgres://x:x@127.0.0.1:1/x?connect_timeout=1")
	require.NoError(t, err)
	t.Cleanup(down.Close)
	svc := domain.NewService(store.NewStore(down), f.read, f.write, stubAuth{}, nil)

	_, err = svc.GetPoll(context.Background(), "p1", userNode)
	require.Error(t, err)
	require.NotErrorIs(t, err, domain.ErrNotFound)

	_, err = svc.UpdateTask(context.Background(), domain.UpdateTaskInput{TaskID: "t1", UserNodeID: userNode})
	require.Error(t, err)
	require.NotErrorIs(t, err, domain.ErrNotFound)

	_, err = f.svc.GetPoll(context.Background(), "no-such-poll", userNode)
	require.ErrorIs(t, err, domain.ErrNotFound)
}
