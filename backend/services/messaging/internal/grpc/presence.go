package grpc

import (
	"time"
)

// defaultPresenceGrace is how long a user's last session may stay gone before
// the tenant is told they went offline. A page reload or a brief network drop
// reconnects well inside it, so colleagues do not see the user flicker.
const defaultPresenceGrace = 3 * time.Second

func presenceKey(tenantID, userID string) string { return tenantID + "|" + userID }

// sessionsOf counts the user's open sessions in the tenant.
func (h *Hub) sessionsOf(tenantID, userID string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	n := 0
	for c := range h.users[userID] {
		if c.tenantID == tenantID {
			n++
		}
	}
	return n
}

// userCameOnline announces a user, unless a recent absence was never announced
// (then they never looked gone) or another session of theirs already did.
func (h *Hub) userCameOnline(c *Client) {
	key := presenceKey(c.tenantID, c.userID)
	h.dom.presenceMu.Lock()
	t, pending := h.dom.pendingAbsent[key]
	if pending {
		t.Stop()
		delete(h.dom.pendingAbsent, key)
	}
	h.dom.presenceMu.Unlock()
	if pending {
		return
	}
	h.BroadcastPresence(c.tenantID, c.userID, c.username, "online")
}

// userLeft schedules the offline announcement for after the grace period and
// sends it only if the user still has no session in the tenant by then.
func (h *Hub) userLeft(c *Client) {
	tenantID, userID, username := c.tenantID, c.userID, c.username
	announce := func() {
		if h.sessionsOf(tenantID, userID) == 0 {
			h.BroadcastPresence(tenantID, userID, username, "offline")
		}
	}
	h.dom.presenceMu.Lock()
	grace := h.dom.presenceGrace
	key := presenceKey(tenantID, userID)
	if old, ok := h.dom.pendingAbsent[key]; ok {
		old.Stop()
	}
	if grace <= 0 {
		delete(h.dom.pendingAbsent, key)
		h.dom.presenceMu.Unlock()
		announce()
		return
	}
	var timer *time.Timer
	timer = time.AfterFunc(grace, func() {
		h.dom.presenceMu.Lock()
		if h.dom.pendingAbsent[key] != timer {
			h.dom.presenceMu.Unlock()
			return // superseded by a reconnect or a newer departure
		}
		delete(h.dom.pendingAbsent, key)
		h.dom.presenceMu.Unlock()
		announce()
	})
	h.dom.pendingAbsent[key] = timer
	h.dom.presenceMu.Unlock()
}
