package grpc

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"expvar"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
	"google.golang.org/protobuf/proto"

	pb "ngac-platform/proto/messaging"
)

// Event log. Every sequenced event is appended to a Redis Stream next to the
// counter that numbered it, in one script, so a number never exists without
// its frame and a frame never exists without its number:
//
//   - workspace events: stream rt:ws:<workspace>, counter wsseq:<workspace>
//   - user-addressed events: stream rt:user:<tenant>:<userNode>, counter
//     useq:<tenant>:<userNode> (a separate sequence per user)
//
// The stream ID of the entry numbered n is "n-0", so a range read by number is
// a range read by ID. Entries hold the frame as it was built before its
// sequence number was known (the number is in the ID); a replay stamps the
// number back on, which yields the very bytes the live frame had.
const (
	wsStreamPrefix   = "rt:ws:"
	userStreamPrefix = "rt:user:"
	userSeqPrefix    = "useq:"

	// streamMaxLen caps every stream (approximate trim).
	streamMaxLen = 10000
	// streamRetention is how long an entry is kept: older entries are trimmed
	// as the stream grows, and an idle stream expires whole.
	streamRetention = 24 * time.Hour
	// trimEvery is how often (in appended entries) the append script also trims
	// by age.
	trimEvery = 50
	// replayLimit is the most frames one replay sends; a client further behind
	// than this refreshes instead.
	replayLimit = 2000

	redisCallTimeout   = 2 * time.Second
	replaySendTimeout  = 5 * time.Second
	streamMetricPeriod = time.Minute
)

// appendScript numbers an event and appends it to its stream atomically.
//
// KEYS[1] counter, KEYS[2] stream; ARGV: frame, max length, now (ms),
// retention (ms), expiry (s), trim interval.
//
// If the stream cannot take the new ID (the counter was lost or reset while an
// old stream survived), the stale stream is dropped: its numbers no longer
// mean anything.
var appendScript = redis.NewScript(`
local n = redis.call('INCR', KEYS[1])
local id = n .. '-0'
local added = redis.pcall('XADD', KEYS[2], 'MAXLEN', '~', ARGV[2], id, 'f', ARGV[1], 't', ARGV[3])
if type(added) == 'table' and added.err then
  redis.call('DEL', KEYS[2])
  redis.call('XADD', KEYS[2], 'MAXLEN', '~', ARGV[2], id, 'f', ARGV[1], 't', ARGV[3])
end
redis.call('EXPIRE', KEYS[2], ARGV[5])
if n % tonumber(ARGV[6]) == 0 then
  local cutoff = tonumber(ARGV[3]) - tonumber(ARGV[4])
  local rows = redis.call('XRANGE', KEYS[2], '-', '+', 'COUNT', 100)
  local keep
  for _, r in ipairs(rows) do
    if tonumber(r[2][4]) >= cutoff then keep = r[1]; break end
  end
  if keep then
    if keep ~= rows[1][1] then redis.call('XTRIM', KEYS[2], 'MINID', keep) end
  elseif #rows == 100 then
    redis.call('XTRIM', KEYS[2], 'MINID', rows[100][1])
  end
end
return n
`)

// appendEvent runs appendScript and remembers the stream for the length metric.
func (h *Hub) appendEvent(counterKey, streamKey string, frame []byte) (uint64, error) {
	if h.rdb == nil {
		return 0, errors.New("no redis")
	}
	ctx, cancel := context.WithTimeout(h.ctx, redisCallTimeout)
	defer cancel()
	n, err := appendScript.Run(ctx, h.rdb, []string{counterKey, streamKey},
		frame, streamMaxLen, time.Now().UnixMilli(), streamRetention.Milliseconds(),
		int(streamRetention.Seconds()), trimEvery).Uint64()
	if err != nil {
		return 0, err
	}
	h.dom.streamKeys.Store(streamKey, struct{}{})
	return n, nil
}

func userCounterKey(tenantID, node string) string { return userSeqPrefix + tenantID + ":" + node }
func userStreamKey(tenantID, node string) string  { return userStreamPrefix + tenantID + ":" + node }

// nextUserSeq numbers a user-addressed frame and logs it in the user's stream.
// It returns 0 when there is no Redis or the append failed: the frame is then
// delivered live like any unsequenced one and a later replay reports "too far".
func (h *Hub) nextUserSeq(tenantID, node string, frame []byte) uint64 {
	if h.rdb == nil {
		return 0
	}
	n, err := h.appendEvent(userCounterKey(tenantID, node), userStreamKey(tenantID, node), frame)
	if err != nil {
		slog.Warn("user event not logged", "error", err)
		return 0
	}
	return n
}

// readStream returns the frames numbered since+1 .. current of one stream, in
// order, and the current counter. ok is false when the stream cannot give
// exactly that range: no Redis, a counter older than the client's cursor, more
// than replayLimit frames, or entries already trimmed.
func (h *Hub) readStream(counterKey, streamKey string, since uint64) (frames [][]byte, current uint64, ok bool) {
	if h.rdb == nil {
		return nil, 0, false
	}
	ctx, cancel := context.WithTimeout(h.ctx, redisCallTimeout)
	defer cancel()
	cur, err := h.rdb.Get(ctx, counterKey).Uint64()
	if err != nil && !errors.Is(err, redis.Nil) {
		slog.Warn("replay: counter read failed", "error", err)
		return nil, 0, false
	}
	if since > cur || cur-since > replayLimit {
		return nil, cur, false
	}
	if since == cur {
		return nil, cur, true
	}
	entries, err := h.rdb.XRange(ctx, streamKey, fmt.Sprintf("%d-0", since+1), fmt.Sprintf("%d-0", cur)).Result()
	if err != nil {
		slog.Warn("replay: stream read failed", "error", err)
		return nil, cur, false
	}
	if uint64(len(entries)) != cur-since {
		return nil, cur, false
	}
	frames = make([][]byte, 0, len(entries))
	for i, e := range entries {
		if e.ID != strconv.FormatUint(since+uint64(i)+1, 10)+"-0" {
			return nil, cur, false
		}
		f, isStr := e.Values["f"].(string)
		if !isStr {
			return nil, cur, false
		}
		frames = append(frames, []byte(f))
	}
	return frames, cur, true
}

// --- user-sequence payloads -------------------------------------------------

// A user-addressed frame that has a number travels between hub instances as
// userSeqMarker + 8-byte big-endian number + the frame. A protobuf envelope
// never starts with a zero byte (field number 0 is invalid), so a payload
// without the marker is a plain frame.
const userSeqMarker = 0x00

func wrapUserSeq(useq uint64, frame []byte) []byte {
	out := make([]byte, 9+len(frame))
	out[0] = userSeqMarker
	binary.BigEndian.PutUint64(out[1:9], useq)
	copy(out[9:], frame)
	return out
}

func unwrapUserSeq(payload []byte) (frame []byte, useq uint64) {
	if len(payload) >= 9 && payload[0] == userSeqMarker {
		return payload[9:], binary.BigEndian.Uint64(payload[1:9])
	}
	return payload, 0
}

// --- control messages (JSON text frames) ------------------------------------

// Control messages are JSON text frames, beside the binary protobuf frames.
// They carry no domain data: cursors and the outcome of a sync. Inside the
// session's send queue they are told apart by their first byte, '{'.
type controlMessage struct {
	Type        string `json:"type"`
	Scope       string `json:"scope,omitempty"`
	WorkspaceID string `json:"workspace_id,omitempty"`
	Seq         uint64 `json:"seq"`
	Replayed    int    `json:"replayed,omitempty"`
}

func marshalControl(m controlMessage) []byte {
	data, err := json.Marshal(m)
	if err != nil {
		slog.Error("failed to marshal control message", "error", err)
		return nil
	}
	return data
}

func isControl(msg []byte) bool { return len(msg) > 0 && msg[0] == '{' }

// --- metric -----------------------------------------------------------------

var streamLenVar = expvar.NewMap("realtime_stream_len")

// reportStreams logs and publishes (expvar realtime_stream_len) the length of
// every stream this instance has written to, once a minute.
func (h *Hub) reportStreams() {
	t := time.NewTicker(streamMetricPeriod)
	defer t.Stop()
	for {
		select {
		case <-h.ctx.Done():
			return
		case <-t.C:
			h.measureStreams()
		}
	}
}

func (h *Hub) measureStreams() (streams int, entries, longest int64) {
	h.dom.streamKeys.Range(func(k, _ any) bool {
		key := k.(string)
		ctx, cancel := context.WithTimeout(h.ctx, redisCallTimeout)
		n, err := h.rdb.XLen(ctx, key).Result()
		cancel()
		if err != nil {
			return true
		}
		if n == 0 { // expired
			h.dom.streamKeys.Delete(key)
			return true
		}
		streams++
		entries += n
		longest = max(longest, n)
		return true
	})
	streamLenVar.Set("streams", intVar(int64(streams)))
	streamLenVar.Set("entries", intVar(entries))
	streamLenVar.Set("longest", intVar(longest))
	slog.Info("realtime event log", "streams", streams, "entries", entries, "longest", longest)
	return
}

func intVar(n int64) *expvar.Int {
	v := new(expvar.Int)
	v.Set(n)
	return v
}

// --- replay -----------------------------------------------------------------

// stampWorkspaceSeq turns a stored workspace frame into the live frame of
// number seq. A frame of another tenant is refused: the stream key is not a
// tenant boundary, the frame's own tenant is.
func stampWorkspaceSeq(stored []byte, tenantID string, seq uint64) ([]byte, bool) {
	var env pb.ServerEnvelope
	if proto.Unmarshal(stored, &env) != nil {
		return nil, false
	}
	d := env.GetDomainEvent()
	if d == nil || d.TenantId != tenantID {
		return nil, false
	}
	d.Seq = seq
	data := marshalEnvelope(&env)
	return data, data != nil
}

func frameTenantIs(stored []byte, tenantID string) bool {
	var env pb.ServerEnvelope
	if proto.Unmarshal(stored, &env) != nil {
		return false
	}
	d := env.GetDomainEvent()
	return d != nil && d.TenantId == tenantID
}

// sendWait queues data for the session, waiting for room: a replay is a burst
// the session asked for and must not be dropped like a live hint.
func (c *Client) sendWait(data []byte) bool {
	t := time.NewTimer(replaySendTimeout)
	defer t.Stop()
	select {
	case c.send <- data:
		return true
	case <-t.C:
		return false
	case <-c.hub.ctx.Done():
		return false
	}
}

// holdWorkspace makes the session set aside live frames of the workspace until
// endHold, so a replay is delivered before anything newer.
func (c *Client) holdWorkspace(workspaceID string) {
	c.holdMu.Lock()
	defer c.holdMu.Unlock()
	c.holdWS = workspaceID
	c.held = nil
}

// endHold releases the held frames numbered above after, in order. The
// session has just taken a burst, so each frame waits for room like a replayed
// one (sendWait) instead of being dropped; if the session stops reading the rest
// are dropped and it sees the hole. While the backlog drains the hold stays on,
// so frames arriving meanwhile queue behind it, in order, and a publisher never
// waits for a slow session.
func (c *Client) endHold(after uint64) {
	for {
		c.holdMu.Lock()
		if len(c.held) == 0 {
			c.holdWS = ""
			c.holdMu.Unlock()
			return
		}
		data := c.held[0]
		c.held = c.held[1:]
		c.holdMu.Unlock()

		var env pb.ServerEnvelope
		if proto.Unmarshal(data, &env) != nil || env.GetDomainEvent().GetSeq() <= after {
			continue
		}
		if !c.sendWait(data) {
			c.holdMu.Lock()
			c.holdWS, c.held = "", nil
			c.holdMu.Unlock()
			return
		}
	}
}

// offerWorkspace delivers a live workspace frame, or sets it aside while the
// session replays that workspace.
func (c *Client) offerWorkspace(workspaceID string, data []byte) {
	c.holdMu.Lock()
	defer c.holdMu.Unlock()
	if c.holdWS == workspaceID && c.holdWS != "" {
		if len(c.held) < replayLimit {
			c.held = append(c.held, data)
		}
		return
	}
	offer(c, data)
}

// handleWorkspaceSync follows a workspace like handleWorkspaceSubscribe and
// then brings the session up to date from `since`: the frames it missed, in
// order, then "synced"; or "too_far" when the log cannot cover the gap.
// Authorisation is exactly the subscribe check and comes first.
func (c *Client) handleWorkspaceSync(workspaceID string, since uint64) {
	if !c.authorizeWorkspaceOrDeny(workspaceID) {
		return
	}
	c.holdWorkspace(workspaceID)
	c.hub.subscribeWorkspace(workspaceID, c)

	frames, cur, ok := c.hub.readStream(workspaceSeqPrefix+workspaceID, wsStreamPrefix+workspaceID, since)
	if !ok && cur == 0 {
		cur = c.hub.currentSeq(workspaceID)
	}
	if ok {
		for i, stored := range frames {
			data, valid := stampWorkspaceSeq(stored, c.tenantID, since+uint64(i)+1)
			if !valid {
				ok = false
				break
			}
			if !c.sendWait(data) {
				c.endHold(^uint64(0))
				return
			}
		}
	}
	msg := controlMessage{Type: "synced", Scope: "workspace", WorkspaceID: workspaceID, Seq: cur, Replayed: len(frames)}
	if !ok {
		msg = controlMessage{Type: "too_far", Scope: "workspace", WorkspaceID: workspaceID, Seq: cur}
	}
	if data := marshalControl(msg); data != nil {
		c.sendWait(data)
	}
	c.endHold(cur)
}

// handleUserSync replays the session's own user stream from `since`. The
// stream is addressed by the session's verified tenant and node, never by
// anything the client names, so no one can read another user's stream.
func (c *Client) handleUserSync(since uint64) {
	if c.tenantID == "" || c.ngacNodeID == "" {
		c.sendError(403, "no user stream for this session")
		return
	}
	frames, cur, ok := c.hub.readStream(userCounterKey(c.tenantID, c.ngacNodeID), userStreamKey(c.tenantID, c.ngacNodeID), since)
	if ok {
		for _, stored := range frames {
			if !frameTenantIs(stored, c.tenantID) {
				ok = false
				break
			}
			if !c.sendWait(stored) {
				return
			}
		}
	}
	msg := controlMessage{Type: "synced", Scope: "user", Seq: cur, Replayed: len(frames)}
	if !ok {
		msg = controlMessage{Type: "too_far", Scope: "user", Seq: cur}
	}
	if data := marshalControl(msg); data != nil {
		c.sendWait(data)
	}
}

// syncRequest is the JSON text frame a client sends to subscribe with a cursor.
type syncRequest struct {
	Type        string  `json:"type"`
	WorkspaceID string  `json:"workspace_id"`
	Since       *uint64 `json:"since"`
}

// handleTextMessage processes a JSON control frame from the client:
// {"type":"sync","workspace_id":"…","since":N} and {"type":"user_sync","since":N}.
func (c *Client) handleTextMessage(message []byte) {
	if !c.authenticated {
		c.sendError(401, "not authenticated")
		return
	}
	var req syncRequest
	if err := json.Unmarshal(message, &req); err != nil || req.Since == nil {
		c.sendError(400, "invalid control message")
		return
	}
	switch req.Type {
	case "sync":
		if req.WorkspaceID == "" {
			c.sendError(400, "workspace_id required")
			return
		}
		c.handleWorkspaceSync(req.WorkspaceID, *req.Since)
	case "user_sync":
		c.handleUserSync(*req.Since)
	default:
		c.sendError(400, "unknown control message")
	}
}
