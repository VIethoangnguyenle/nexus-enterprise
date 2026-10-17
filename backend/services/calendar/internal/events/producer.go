// Package events publishes what happened to a calendar event to the people it
// concerns. The records travel on "calendar.events", the same topic the realtime
// frames use; Record tells them apart. The messaging service turns a notice into
// a notification (and a push for an invitation or a reminder).
package events

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
)

// Topic is where calendar notices and realtime frames are published.
const Topic = "calendar.events"

// RecordNotice marks a record as a notice, not a realtime frame.
const RecordNotice = "notice"

// What happened.
const (
	TypeInvited   = "event.invited"
	TypeUpdated   = "event.updated"
	TypeCancelled = "event.cancelled"
	TypeRsvp      = "event.rsvp"
	TypeReminder  = "event.reminder"
	// TypeCreated is announced only for an event on a group's calendar and carries
	// the group; nobody is notified by it.
	TypeCreated = "event.created"
	TypeShared  = "calendar.shared"
)

// Recipient is one person a notice is for: the user node the policy knows them
// by and the account the notification is stored under.
type Recipient struct {
	NodeID string `json:"node_id"`
	UserID string `json:"user_id"`
}

// Notice is one thing that happened to an event or a calendar. It carries ids,
// the actor, the audience and the title and times; the client writes the
// sentence. Times are RFC 3339 in UTC.
type Notice struct {
	Record      string `json:"record"`
	Type        string `json:"type"`
	TenantID    string `json:"tenant_id"`
	WorkspaceID string `json:"workspace_id"`
	EventID     string `json:"event_id,omitempty"`
	CalendarID  string `json:"calendar_id"`
	// ChannelID is the group of a group calendar.
	ChannelID string `json:"channel_id,omitempty"`
	// OAID is the node a recipient must still be able to read for the notice to
	// reach them: the event's own, else its calendar's.
	OAID string `json:"oa_id,omitempty"`
	// ActorNodeID and ActorUserID are whoever caused it; empty for a reminder.
	ActorNodeID string      `json:"actor_node_id,omitempty"`
	ActorUserID string      `json:"actor_user_id,omitempty"`
	Recipients  []Recipient `json:"recipients,omitempty"`
	Title       string      `json:"title,omitempty"`
	Start       string      `json:"start,omitempty"`
	End         string      `json:"end,omitempty"`
	AllDay      bool        `json:"all_day,omitempty"`
	// OccurrenceStart names the occurrence of a repeating event the notice is about.
	OccurrenceStart string `json:"occurrence_start,omitempty"`
	// Response and Comment are a guest's answer (TypeRsvp).
	Response string `json:"response,omitempty"`
	Comment  string `json:"comment,omitempty"`
	// MinutesBefore is how long before the start a reminder fell due.
	MinutesBefore *int  `json:"minutes_before,omitempty"`
	At            int64 `json:"at"`
}

// Publisher is what the domain announces notices through. *Producer is the
// Kafka-backed one; a nil *Producer discards everything.
type Publisher interface {
	Publish(Notice)
}

var _ Publisher = (*Producer)(nil)

// Producer wraps a Kafka client.
type Producer struct {
	client *kgo.Client
	// send hands a serialized notice to the broker. Tests replace it.
	send func(key string, data []byte)
}

// NewProducer connects to the brokers.
func NewProducer(brokers []string) (*Producer, error) {
	client, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.AllowAutoTopicCreation(),
		// A broker that is down costs notices, never request latency.
		kgo.RecordDeliveryTimeout(10*time.Second),
		kgo.MaxBufferedRecords(2000),
	)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Ping(ctx); err != nil {
		client.Close()
		return nil, err
	}
	slog.Info("kafka producer connected for calendar service")
	p := &Producer{client: client}
	p.send = p.produce
	return p, nil
}

// Close flushes and closes the client.
func (p *Producer) Close() {
	if p != nil && p.client != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = p.client.Flush(ctx)
		p.client.Close()
	}
}

// Publish sends the notice, keyed by workspace so one workspace's notices keep
// their order. A notice that cannot be sent is logged and dropped: the change it
// reports is already committed.
func (p *Producer) Publish(n Notice) {
	if p == nil || p.send == nil {
		return
	}
	n.Record = RecordNotice
	if n.At == 0 {
		n.At = time.Now().UnixMilli()
	}
	data, err := json.Marshal(n)
	if err != nil {
		slog.Error("calendar notice not encoded", "type", n.Type, "error", err)
		return
	}
	p.send(n.WorkspaceID, data)
}

func (p *Producer) produce(key string, data []byte) {
	p.client.TryProduce(context.Background(), &kgo.Record{Topic: Topic, Key: []byte(key), Value: data},
		func(_ *kgo.Record, err error) {
			if err != nil {
				slog.Warn("kafka publish failed", "topic", Topic, "error", err)
			}
		})
}

// Recorder is a Publisher that keeps what it is given, for tests.
type Recorder struct {
	mu      sync.Mutex
	notices []Notice
}

// Publish records the notice.
func (r *Recorder) Publish(n Notice) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.notices = append(r.notices, n)
}

// Notices returns a copy of everything recorded so far.
func (r *Recorder) Notices() []Notice {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]Notice(nil), r.notices...)
}
