// Package domain — web push for chat messages.
//
// A message is pushed to the people it concerns: the other side of a direct
// message, the users it mentions, the followers of the thread it replies in.
// Each recipient is checked again when the push is sent: they must still read
// the channel, want that kind of push, be outside their quiet hours and not be
// looking at the conversation right now. Several messages of one conversation
// within a short window become one notification on the device.
package domain

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"ngac-platform/ngac"
	"ngac-platform/pkg/push"
	pb "ngac-platform/proto/messaging"
	"ngac-platform/services/messaging/internal/store"
)

// PushKind is why a message concerns someone.
type PushKind string

const (
	PushDirect      PushKind = "direct"
	PushMention     PushKind = "mention"
	PushThreadReply PushKind = "thread_reply"
	// PushEveryMessage: the person set the group to "all".
	PushEveryMessage PushKind = "message"
)

// ViewingChecker reports whether a user has the conversation open right now.
// The hub implements it.
type ViewingChecker interface {
	IsViewing(userID, channelID string) bool
}

// pushWindow is how long further messages of one conversation are gathered
// into a single follow-up notification.
const pushWindow = 30 * time.Second

// previewChars bounds the text shown in a notification.
const previewChars = 140

// MaxMentionPushes bounds how many mentioned people one message pushes to. The
// mention list comes from the client; without a bound a writer could page
// every reader of a channel with one message.
const MaxMentionPushes = 20

type pushState struct {
	sender  push.Sender
	viewing ViewingChecker
	now     func() time.Time
	window  time.Duration

	mu      sync.Mutex
	batches map[string]*pushBatch // by user and conversation
}

type pushBatch struct {
	count int
	last  push.Message
}

// SetPush turns web push on. A nil or disabled sender keeps it off.
func (s *Service) SetPush(sender push.Sender, viewing ViewingChecker) {
	if sender == nil || !sender.Enabled() {
		s.push = nil
		return
	}
	s.push = &pushState{sender: sender, viewing: viewing, now: time.Now, window: pushWindow, batches: map[string]*pushBatch{}}
}

// SetPushWindowForTest shortens the gathering window so a test need not wait.
func SetPushWindowForTest(s *Service, d time.Duration) {
	if s.push != nil {
		s.push.window = d
	}
}

// PushRecipient is one person a message concerns, and why.
type PushRecipient struct {
	Person store.Person
	Kind   PushKind
}

// PushRecipients returns who a message should be pushed to, after every check.
func (s *Service) PushRecipients(ctx context.Context, msg *pb.Message) ([]PushRecipient, error) {
	if msg == nil || (msg.MessageType != "" && msg.MessageType != "user") {
		return nil, nil
	}
	ch, err := s.loadChannel(ctx, msg.ChannelId)
	if err != nil {
		return nil, err
	}
	kinds := map[string]PushKind{}
	people := map[string]store.Person{}
	add := func(ps []store.Person, k PushKind) {
		for _, p := range ps {
			if p.UserID == msg.SenderId {
				continue
			}
			// A mention outranks a direct message, which outranks a thread reply.
			if cur, ok := kinds[p.UserID]; !ok || rank(k) > rank(cur) {
				kinds[p.UserID] = k
			}
			people[p.UserID] = p
		}
	}
	if msg.ParentMessageId != "" {
		followers, err := s.store.ThreadFollowers(ctx, msg.ParentMessageId)
		if err != nil {
			return nil, err
		}
		ps := make([]store.Person, 0, len(followers))
		for _, f := range followers {
			ps = append(ps, store.Person{UserID: f.UserID, NodeID: f.NodeID})
		}
		add(ps, PushThreadReply)
	}
	if ch.ChannelType == "dm" {
		ps, err := s.store.ChannelPeople(ctx, ch.ID)
		if err != nil {
			return nil, err
		}
		add(ps, PushDirect)
	}
	if len(msg.Mentions) > 0 {
		mentions := msg.Mentions
		if len(mentions) > MaxMentionPushes {
			mentions = mentions[:MaxMentionPushes]
		}
		ps, err := s.store.PeopleByID(ctx, mentions)
		if err != nil {
			return nil, err
		}
		add(ps, PushMention)
	}
	// "all" on a group: every message concerns the people who chose it.
	if ch.ChannelType != "dm" {
		ps, err := s.store.PeopleWithLevel(ctx, ch.ID, NotifyAll)
		if err != nil {
			return nil, err
		}
		add(ps, PushEveryMessage)
	}

	ids := make([]string, 0, len(kinds))
	for id := range kinds {
		ids = append(ids, id)
	}
	levels, err := s.store.NotificationLevels(ctx, ch.ID, ids)
	if err != nil {
		return nil, err
	}

	var out []PushRecipient
	for id, k := range kinds {
		p := people[id]
		// "off" silences the conversation; in a group a mention still gets through.
		if levels[id] == NotifyOff && (k != PushMention || ch.ChannelType == "dm") {
			continue
		}
		// Following or being mentioned is not access: a removed member, or a
		// mentioned non-member, is told nothing; a policy error tells nobody.
		if p.NodeID == "" || s.checkAccess(ctx, p.NodeID, ch.NGACOaID, ngac.OpRead) != nil {
			continue
		}
		prefs, err := s.store.GetPushPreferences(ctx, id)
		if err != nil {
			return nil, err
		}
		if !wants(prefs, k) || inQuietHours(prefs, s.pushNow()) {
			continue
		}
		if s.push != nil && s.push.viewing != nil && s.push.viewing.IsViewing(id, ch.ID) {
			continue
		}
		out = append(out, PushRecipient{Person: p, Kind: k})
	}
	return out, nil
}

func rank(k PushKind) int {
	switch k {
	case PushMention:
		return 3
	case PushDirect:
		return 2
	case PushThreadReply:
		return 1
	default:
		return 0
	}
}

func wants(p store.PushPreferences, k PushKind) bool {
	switch k {
	case PushDirect:
		return p.Direct
	case PushMention:
		return p.Mention
	case PushThreadReply:
		return p.ThreadReply
	default:
		// "all" was chosen for this very conversation: that choice is the wish.
		return true
	}
}

// inQuietHours reports whether now falls in the user's quiet window, in
// their time zone; a window with start > end wraps midnight.
func inQuietHours(p store.PushPreferences, now time.Time) bool {
	if !p.QuietEnabled || p.QuietStart == p.QuietEnd {
		return false
	}
	loc, err := time.LoadLocation(p.TimeZone)
	if err != nil {
		loc = time.UTC
	}
	t := now.In(loc)
	m := t.Hour()*60 + t.Minute()
	if p.QuietStart < p.QuietEnd {
		return m >= p.QuietStart && m < p.QuietEnd
	}
	return m >= p.QuietStart || m < p.QuietEnd
}

func (s *Service) pushNow() time.Time {
	if s.push != nil {
		return s.push.now()
	}
	return time.Now()
}

// PushForMessage sends the push notifications a stored message calls for. It
// does nothing when push is off. Errors are logged without the message's text.
func (s *Service) PushForMessage(ctx context.Context, msg *pb.Message) {
	if s.push == nil {
		return
	}
	recipients, err := s.PushRecipients(ctx, msg)
	if err != nil {
		slog.Warn("push recipients not resolved", "message_id", msg.Id, "error", err)
		return
	}
	if len(recipients) == 0 {
		return
	}
	ch, err := s.loadChannel(ctx, msg.ChannelId)
	if err != nil {
		return
	}
	sender, _ := s.store.DisplayName(ctx, msg.SenderId)
	for _, r := range recipients {
		// The caller's deadline bounds the whole delivery, not each send.
		if ctx.Err() != nil {
			slog.Warn("push delivery cut short", "message_id", msg.Id, "error", ctx.Err())
			return
		}
		s.deliver(ctx, r.Person.UserID, ch, notificationFor(ch, sender, msg))
	}
}

// PushLater runs PushForMessage off the request path: the sender's response
// does not wait for it, and a client that hangs up does not cancel it.
func (s *Service) PushLater(ctx context.Context, msg *pb.Message) {
	if s.push == nil || msg == nil {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
		defer cancel()
		s.PushForMessage(ctx, msg)
	}()
}

func notificationFor(ch *store.Channel, sender string, msg *pb.Message) push.Message {
	text := plainText(msg.ContentFormat, msg.Content)
	if strings.TrimSpace(text) == "" && len(msg.Attachments) > 0 {
		text = "📎 Tệp đính kèm"
	}
	text = strings.Join(strings.Fields(text), " ")
	if utf8.RuneCountInString(text) > previewChars {
		text = string([]rune(text)[:previewChars-1]) + "…"
	}
	m := push.Message{CollapseID: "ch-" + ch.ID, URL: "/channels/" + ch.ID}
	if msg.ParentMessageId != "" {
		m.URL += "?thread=" + msg.ParentMessageId
	}
	if ch.ChannelType == "dm" {
		m.Title, m.Body = sender, text
	} else {
		m.Title, m.Body = ch.Name, sender+": "+text
	}
	return m
}

// deliver sends at once when nothing was sent for this conversation lately;
// otherwise it gathers the message into one follow-up sent at the end of the
// window, which replaces the earlier notification on the device.
func (s *Service) deliver(ctx context.Context, userID string, ch *store.Channel, m push.Message) {
	m.ExternalIDs = []string{userID}
	key := userID + "\x00" + ch.ID
	p := s.push
	p.mu.Lock()
	if b, open := p.batches[key]; open {
		b.count++
		b.last = m
		p.mu.Unlock()
		return
	}
	p.batches[key] = &pushBatch{}
	p.mu.Unlock()

	s.send(ctx, m)
	// The follow-up runs after the request is long over: it gets its own
	// bounded context from send, not the caller's.
	time.AfterFunc(p.window, func() {
		p.mu.Lock()
		b := p.batches[key]
		delete(p.batches, key)
		p.mu.Unlock()
		if b == nil || b.count == 0 {
			return
		}
		follow := b.last
		follow.Body = fmt.Sprintf("%d tin mới", b.count+1)
		s.send(context.WithoutCancel(ctx), follow)
	})
}

// send delivers one notification within the caller's deadline, and at most
// 20 seconds; a client hanging up does not cancel it (see PushLater).
func (s *Service) send(ctx context.Context, m push.Message) {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	if err := s.push.sender.Send(ctx, m); err != nil {
		slog.Warn("push not delivered", "recipients", len(m.ExternalIDs), "error", err)
	}
}

// GetPushPreferences returns the caller's push preferences.
func (s *Service) GetPushPreferences(ctx context.Context, userID string) (store.PushPreferences, error) {
	if userID == "" {
		return store.PushPreferences{}, fmt.Errorf("%w: a signed-in user is required", ErrAccessDenied)
	}
	return s.store.GetPushPreferences(ctx, userID)
}

// SavePushPreferences stores the caller's push preferences.
func (s *Service) SavePushPreferences(ctx context.Context, userID string, p store.PushPreferences) error {
	if userID == "" {
		return fmt.Errorf("%w: a signed-in user is required", ErrAccessDenied)
	}
	if p.QuietStart < 0 || p.QuietStart > 1439 || p.QuietEnd < 0 || p.QuietEnd > 1439 {
		return fmt.Errorf("%w: quiet hours are minutes of the day", ErrInvalidInput)
	}
	if p.TimeZone == "" {
		p.TimeZone = store.DefaultPushPreferences.TimeZone
	}
	if _, err := time.LoadLocation(p.TimeZone); err != nil {
		return fmt.Errorf("%w: unknown time zone", ErrInvalidInput)
	}
	return s.store.SavePushPreferences(ctx, userID, p)
}
