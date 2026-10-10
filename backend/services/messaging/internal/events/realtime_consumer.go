package events

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"

	"ngac-platform/pkg/realtime"
)

// Asset topics predate the shared realtime contract; they are translated into
// asset-domain DomainEvents here so assets reach the workspace like every
// other domain.
const (
	topicAssetLifecycle  = "asset.lifecycle"
	topicAssetRequest    = "asset.request"
	topicAssetAssignment = "asset.assignment"
)

// maxEventAge is how old an event may be and still be delivered. A restart
// replays nothing older than this: a browser that was connected through the
// outage resynchronises on its sequence hole, one that was not on reconnect.
const maxEventAge = 30 * time.Second

// DomainPublisher takes a committed change and fans it out. *grpc.Hub does.
type DomainPublisher interface {
	PublishDomain(realtime.Event)
}

// WorkspaceRevoker ends a user's live subscriptions to a workspace. *grpc.Hub
// does; a publisher that cannot is simply not asked.
type WorkspaceRevoker interface {
	RevokeWorkspaceSubscriptions(workspaceID, userNodeID string)
}

// permissionSettle is how long a permission event is held before delivery.
// Policy read replicas learn of a graph change from their own Kafka consumer,
// separately from this event, so a client that refetched the instant it was
// told could still be answered from the graph as it was. The frontend repeats
// its refresh once more after a delay for the replica that is slower still.
const permissionSettle = 750 * time.Millisecond

// RealtimeConsumer reads the "<domain>.events" topics (and the asset topics)
// and hands each event to the hub.
//
// It has its own consumer group and starts at the end of each topic: realtime
// events are nudges about the present, so a fresh group must not replay
// history.
type RealtimeConsumer struct {
	client *kgo.Client
	pub    DomainPublisher
	cancel context.CancelFunc
	now    func() time.Time
	settle time.Duration
}

// NewRealtimeConsumer starts consuming. Every messaging instance joins the same
// group, so each event is handled by exactly one of them; the hub's Redis
// channels carry it to the sessions on the others.
func NewRealtimeConsumer(brokers []string, pub DomainPublisher) (*RealtimeConsumer, error) {
	topics := append(realtime.Topics(), topicAssetLifecycle, topicAssetRequest, topicAssetAssignment)
	client, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.ConsumerGroup("messaging-realtime"),
		kgo.ConsumeTopics(topics...),
		kgo.ConsumeResetOffset(kgo.NewOffset().AtEnd()),
		kgo.AllowAutoTopicCreation(),
	)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	c := &RealtimeConsumer{client: client, pub: pub, cancel: cancel, now: time.Now, settle: permissionSettle}
	go c.run(ctx)
	return c, nil
}

// Close stops the consumer.
func (c *RealtimeConsumer) Close() {
	if c != nil {
		c.cancel()
		c.client.Close()
	}
}

func (c *RealtimeConsumer) run(ctx context.Context) {
	slog.Info("realtime event consumer started")
	for {
		fetches := c.client.PollRecords(ctx, 200)
		if ctx.Err() != nil {
			return
		}
		if errs := fetches.Errors(); len(errs) > 0 {
			for _, e := range errs {
				slog.Warn("realtime consumer error", "topic", e.Topic, "error", e.Err)
			}
			time.Sleep(time.Second)
			continue
		}
		fetches.EachRecord(func(r *kgo.Record) { c.handle(r.Topic, r.Value) })
	}
}

// handle translates one record and publishes it. A record that does not parse,
// lacks its boundary, or is stale is dropped: there is nobody to tell.
func (c *RealtimeConsumer) handle(topic string, value []byte) {
	evt, err := Translate(topic, value)
	if err != nil {
		slog.Warn("realtime event skipped", "topic", topic, "error", err)
		return
	}
	if age := c.now().Sub(time.UnixMilli(evt.At)); evt.At > 0 && age > maxEventAge {
		slog.Debug("realtime event stale, skipped", "topic", topic, "age", age)
		return
	}
	// A person who stops belonging to the workspace stops following it before
	// anyone is told they left.
	if evt.Domain == realtime.DomainWorkspace && evt.Kind == realtime.KindMemberRemoved {
		if r, ok := c.pub.(WorkspaceRevoker); ok {
			for _, node := range evt.IDs {
				r.RevokeWorkspaceSubscriptions(evt.WorkspaceID, node)
			}
		}
	}
	if evt.Domain == realtime.DomainPermission && c.settle > 0 {
		time.AfterFunc(c.settle, func() { c.pub.PublishDomain(evt) })
		return
	}
	c.pub.PublishDomain(evt)
}

// Translate turns a topic record into a realtime.Event.
func Translate(topic string, value []byte) (realtime.Event, error) {
	switch topic {
	case topicAssetLifecycle:
		var a AssetLifecycleEvent
		if err := json.Unmarshal(value, &a); err != nil {
			return realtime.Event{}, err
		}
		return assetEvent(realtime.KindUpdated, a.TenantID, a.WorkspaceID, a.ActorID, a.Timestamp, a.AssetID)
	case topicAssetRequest:
		var a AssetRequestEvent
		if err := json.Unmarshal(value, &a); err != nil {
			return realtime.Event{}, err
		}
		return assetEvent(realtime.KindRequestChanged, a.TenantID, a.WorkspaceID, a.RequesterID, a.Timestamp, a.RequestID)
	case topicAssetAssignment:
		var a AssetAssignmentEvent
		if err := json.Unmarshal(value, &a); err != nil {
			return realtime.Event{}, err
		}
		return assetEvent(realtime.KindAssigned, a.TenantID, a.WorkspaceID, a.ActorID, a.Timestamp, a.AssetID)
	}
	for _, t := range realtime.Topics() {
		if t == topic {
			return realtime.Decode(value)
		}
	}
	return realtime.Event{}, fmt.Errorf("unexpected topic %q", topic)
}

func assetEvent(kind, tenantID, workspaceID, actor string, at int64, id string) (realtime.Event, error) {
	if id == "" {
		return realtime.Event{}, errors.New("asset event names no entity")
	}
	e := realtime.Event{
		Domain: realtime.DomainAsset, Kind: kind, TenantID: tenantID, WorkspaceID: workspaceID,
		IDs: []string{id}, ActorUserID: actor, At: at,
	}
	return e, e.Validate()
}
