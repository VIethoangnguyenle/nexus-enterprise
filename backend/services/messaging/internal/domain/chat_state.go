package domain

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"ngac-platform/pkg/grpcauth"
	"ngac-platform/pkg/httputil"
	pb "ngac-platform/proto/messaging"
	"ngac-platform/services/messaging/internal/store"
)

// ChatState is everything the chat navigator needs the moment the app opens,
// each part exactly what its own endpoint answers.
type ChatState struct {
	Channels []*pb.Channel
	DMs      []*pb.Channel
	Unread   []*pb.ChannelUnread
	Activity []*pb.ConversationActivity
	Sections []*Section
	// UnreadMentions counts unread mentions of the caller.
	UnreadMentions int
	// UnreadThreads are the followed threads with news, newest first; their
	// number is the unread thread count.
	UnreadThreads []*pb.Message
}

// unreadThreadsLimit matches the list the unread threads screen asks for.
const unreadThreadsLimit = 20

// ChatState gathers the chat navigator's state for a workspace the caller
// belongs to. Each part goes through the domain function of its own endpoint, so
// every part is authorized as that endpoint authorizes it: a conversation the
// caller cannot read is in none of them.
//
// A workspace other than the caller's own tenant is refused before anything is
// read.
func (s *Service) ChatState(ctx context.Context, workspaceID string, c Caller) (*ChatState, error) {
	if c.UserID == "" || c.NodeID == "" || workspaceID == "" {
		return nil, ErrInvalidInput
	}
	if workspaceID != c.TenantID {
		return nil, fmt.Errorf("%w: another workspace", ErrAccessDenied)
	}

	out := &ChatState{}
	var wg sync.WaitGroup
	var mu sync.Mutex
	var firstErr error
	part := func(run func() error) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := run(); err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				mu.Unlock()
			}
		}()
	}
	part(func() (err error) { out.Channels, err = s.ListChannels(ctx, workspaceID, c.NodeID); return })
	part(func() (err error) { out.DMs, err = s.ListDMs(ctx, c.NodeID); return })
	part(func() (err error) { out.Unread, err = s.GetUnreadCounts(ctx, c.UserID, c.NodeID, workspaceID); return })
	part(func() (err error) {
		out.Activity, err = s.ConversationActivity(ctx, c.UserID, c.NodeID, workspaceID)
		return
	})
	part(func() (err error) {
		out.UnreadMentions, err = s.UnreadMentionCount(ctx, c.UserID, c.NodeID, workspaceID)
		return
	})
	part(func() error {
		secs, err := s.ListSections(ctx, workspaceID, c)
		if errors.Is(err, ErrAccessDenied) {
			// Sections need read on the workspace itself; without it the
			// navigator simply has none, as when it asks the sections
			// endpoint and is refused.
			return nil
		}
		out.Sections = secs
		return err
	})
	part(func() error {
		list, err := s.UnreadThreads(ctx, c.UserID, c.NodeID, workspaceID, unreadThreadsLimit)
		if list != nil {
			out.UnreadThreads = list.Messages
		}
		return err
	})
	wg.Wait()
	if firstErr != nil {
		return nil, firstErr
	}
	return out, nil
}

// ChatVersionKeys are the counters the chat state of one user in one tenant
// depends on. Everything the state shows about other people in the workspace
// (new messages, edits, a channel created, a member removed, a permission
// changed) moves the tenant's counter. What belongs to the person across
// workspaces moves their own: read marks, stars, notification level, sections,
// thread follows, and direct messages, which are not tenant-scoped (a DM
// written in one workspace changes the list another workspace shows). A counter
// is never shared between tenants, and the per-user one never between users.
func ChatVersionKeys(tenantID, userID string) []string {
	return []string{
		httputil.VersionKey("chat", tenantID),
		httputil.VersionKey("chat", "user", userID),
	}
}

// SetVersions wires the counters that mark the chat state as changed. Without
// them (no Redis) nothing is counted and no ETag is issued.
func (s *Service) SetVersions(v httputil.Versions) { s.versions = v }

// Versions are the wired counters; nil when there is no Redis.
func (s *Service) Versions() httputil.Versions {
	if s == nil {
		return nil
	}
	return s.versions
}

// versionTimeout bounds a bump: a slow Redis must not hold up the request that
// made the change, and a missed bump is healed by the ETag lifetime.
const versionTimeout = 500 * time.Millisecond

// BumpChat records that the chat state changed for a tenant, and for the named
// users' own state. It is called wherever a change is announced, so a write
// path that announces cannot forget it.
func (s *Service) BumpChat(tenantID string, userIDs ...string) {
	if s == nil || s.versions == nil || tenantID == "" {
		return
	}
	keys := []string{httputil.VersionKey("chat", tenantID)}
	for _, u := range userIDs {
		if u != "" {
			keys = append(keys, httputil.VersionKey("chat", "user", u))
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), versionTimeout)
	defer cancel()
	if err := s.versions.Bump(ctx, keys...); err != nil {
		slog.Warn("chat state version not bumped", "tenant", tenantID, "error", err)
	}
}

// bumpChannel records a change to a conversation's content (a message sent,
// edited, deleted, a call card rewritten). A direct message belongs to no
// workspace and its people may read it from any of theirs, so each of them is
// bumped as well as the tenant the change was made in.
func (s *Service) bumpChannel(ctx context.Context, ch *store.Channel) {
	if s.versions == nil || ch == nil {
		return
	}
	tenant := ch.WorkspaceID
	if tenant == "" {
		tenant = grpcauth.CallerFrom(ctx).TenantID
	}
	var users []string
	if ch.ChannelType == "dm" {
		var err error
		if users, err = s.store.DMOthers(ctx, ch.ID, ""); err != nil {
			slog.Warn("direct message people not found for chat state version", "channel", ch.ID, "error", err)
		}
	}
	s.BumpChat(tenant, users...)
}

// bumpChannelID is bumpChannel for a caller that holds only the id.
func (s *Service) bumpChannelID(ctx context.Context, channelID string) {
	if s.versions == nil {
		return
	}
	ch, err := s.store.GetChannel(ctx, channelID)
	if err != nil || ch == nil {
		return
	}
	s.bumpChannel(ctx, ch)
}
