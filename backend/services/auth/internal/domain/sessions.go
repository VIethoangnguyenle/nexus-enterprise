package domain

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"
)

// userSessionsPrefix keys a sorted set of a user's session ids, scored by the
// moment each session began. It is an index only: a session listed there counts
// only while its family still exists and no user-wide revocation postdates it.
const userSessionsPrefix = "user_sessions:"

func userSessionsKey(uid string) string { return userSessionsPrefix + uid }

// SessionInfo describes one signed-in session of a person.
type SessionInfo struct {
	// SessionID is the opaque handle used to end the session. It is never shown.
	SessionID string `json:"session_id"`
	// Device is a coarse "browser · system" label, or empty when unknown.
	Device    string    `json:"device"`
	StartedAt time.Time `json:"started_at"`
	// Current is true for the session the request came from.
	Current bool `json:"current"`
}

// sessionIdentity reads the identity of a live session family from any of its
// tokens. ok is false when the family no longer exists.
func (s *RefreshStore) sessionIdentity(ctx context.Context, sessionID string) (RefreshIdentity, bool, error) {
	tokens, err := s.rdb.SMembers(ctx, sessionKey(sessionID)).Result()
	if err != nil {
		return RefreshIdentity{}, false, fmt.Errorf("%w: read session family: %v", ErrUnavailable, err)
	}
	for _, t := range tokens {
		rec, err := s.read(ctx, t)
		if errors.Is(err, ErrRefreshRejected) {
			continue
		}
		if err != nil {
			return RefreshIdentity{}, false, err
		}
		return rec.RefreshIdentity, true, nil
	}
	return RefreshIdentity{}, false, nil
}

// liveSession reports the identity of a session that still belongs to userID and
// has not been ended, by itself or by a user-wide revocation.
func (s *RefreshStore) liveSession(ctx context.Context, userID, sessionID string) (RefreshIdentity, bool, error) {
	id, ok, err := s.sessionIdentity(ctx, sessionID)
	if err != nil || !ok || id.UserID != userID {
		return RefreshIdentity{}, false, err
	}
	revoked, err := s.revokedForUser(ctx, id)
	if err != nil {
		return RefreshIdentity{}, false, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	return id, !revoked, nil
}

// ListUserSessions returns the live sessions of userID, oldest first. Index
// entries whose session has ended are dropped as they are found.
func (s *RefreshStore) ListUserSessions(ctx context.Context, userID string) ([]SessionInfo, error) {
	if s == nil || s.rdb == nil || userID == "" {
		return nil, nil
	}
	sids, err := s.rdb.ZRange(ctx, userSessionsKey(userID), 0, -1).Result()
	if err != nil {
		return nil, fmt.Errorf("%w: list user sessions: %v", ErrUnavailable, err)
	}
	out := make([]SessionInfo, 0, len(sids))
	for _, sid := range sids {
		id, ok, err := s.liveSession(ctx, userID, sid)
		if err != nil {
			return nil, err
		}
		if !ok {
			// A stale entry is harmless (it is filtered on every read), so a failed
			// clean-up is logged, not returned.
			if err := s.rdb.ZRem(ctx, userSessionsKey(userID), sid).Err(); err != nil {
				slog.Warn("stale session index entry not removed", "error", err)
			}
			continue
		}
		out = append(out, SessionInfo{SessionID: sid, Device: id.Device, StartedAt: time.Unix(0, id.StartedAt).UTC()})
	}
	return out, nil
}

// ListSessions returns the caller's own sessions: the one in use first, then the
// others newest first.
func (s *Service) ListSessions(ctx context.Context, callerUserID, callerSessionID string) ([]SessionInfo, error) {
	sessions, err := s.refresh.ListUserSessions(ctx, callerUserID)
	if err != nil {
		return nil, err
	}
	for i := range sessions {
		sessions[i].Current = callerSessionID != "" && sessions[i].SessionID == callerSessionID
	}
	sort.SliceStable(sessions, func(i, j int) bool {
		if sessions[i].Current != sessions[j].Current {
			return sessions[i].Current
		}
		return sessions[i].StartedAt.After(sessions[j].StartedAt)
	})
	return sessions, nil
}

// RevokeOwnSession ends one of the caller's other sessions through the existing
// RevokeSession, which takes a bare session id, so ownership is checked first.
// A session of someone else answers exactly like one that does not exist, so the
// route cannot be used to learn which session ids are live. The session in use is
// refused: signing out is how a person ends that one.
func (s *Service) RevokeOwnSession(ctx context.Context, callerUserID, callerSessionID, targetSessionID string) error {
	if targetSessionID == "" {
		return fmt.Errorf("%w: session not found", ErrNotFound)
	}
	if targetSessionID == callerSessionID {
		return fmt.Errorf("%w: the session in use is ended by signing out", ErrInvalidInput)
	}
	if s.refresh == nil || s.refresh.rdb == nil {
		return fmt.Errorf("%w: session not found", ErrNotFound)
	}
	_, ok, err := s.refresh.liveSession(ctx, callerUserID, targetSessionID)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("%w: session not found", ErrNotFound)
	}
	if err := s.refresh.RevokeSession(ctx, targetSessionID); err != nil {
		return fmt.Errorf("%w: %v", ErrUnavailable, err)
	}
	return nil
}

// DeviceLabel turns a User-Agent into a coarse "browser · system" label, so a
// person can tell their sessions apart without the raw header being stored. An
// agent that is not a known browser gives "".
func DeviceLabel(ua string) string {
	var system string
	switch {
	case strings.Contains(ua, "iPhone"):
		system = "iPhone"
	case strings.Contains(ua, "iPad"):
		system = "iPad"
	case strings.Contains(ua, "Android"):
		system = "Android"
	case strings.Contains(ua, "Windows"):
		system = "Windows"
	case strings.Contains(ua, "Macintosh"), strings.Contains(ua, "Mac OS X"):
		system = "macOS"
	case strings.Contains(ua, "CrOS"):
		system = "ChromeOS"
	case strings.Contains(ua, "Linux"):
		system = "Linux"
	}
	var browser string
	switch {
	case strings.Contains(ua, "Edg/"), strings.Contains(ua, "EdgiOS/"), strings.Contains(ua, "EdgA/"):
		browser = "Edge"
	case strings.Contains(ua, "Firefox/"), strings.Contains(ua, "FxiOS/"):
		browser = "Firefox"
	case strings.Contains(ua, "Chrome/"), strings.Contains(ua, "CriOS/"):
		browser = "Chrome"
	case strings.Contains(ua, "Safari/"):
		browser = "Safari"
	}
	if browser == "" || system == "" {
		return ""
	}
	return browser + " · " + system
}
