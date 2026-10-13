// Package events publishes workspace events the notification service files.
package events

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"

	"ngac-platform/services/workspace/internal/domain"
)

// InvitationTopic carries one record per account offered a workspace. The
// messaging service's notification consumer turns each into a notification.
const InvitationTopic = "workspace.invitation"

const publishTimeout = 10 * time.Second

// InvitationOffered is the JSON published to InvitationTopic. It names the one
// account it is for (users.id) and carries only what the sentence needs.
type InvitationOffered struct {
	InvitationID  string `json:"invitation_id"`
	UserID        string `json:"user_id"`
	WorkspaceName string `json:"workspace_name"`
	InviterName   string `json:"inviter_name,omitempty"`
	Timestamp     int64  `json:"timestamp"`
}

// Producer publishes invitation notices to Redpanda. It is fire-and-forget: a
// broker that is down costs the notification, never the invitation.
type Producer struct {
	client *kgo.Client
}

// Connect builds the producer from KAFKA_BROKERS (default localhost:19092). An
// unreachable broker is logged and returns nil, which InvitationOffered tolerates.
func Connect() *Producer {
	brokers := os.Getenv("KAFKA_BROKERS")
	if brokers == "" {
		brokers = "localhost:19092"
	}
	client, err := kgo.NewClient(
		kgo.SeedBrokers(strings.Split(brokers, ",")...),
		kgo.AllowAutoTopicCreation(),
		kgo.RecordDeliveryTimeout(publishTimeout),
		kgo.MaxBufferedRecords(2000),
	)
	if err != nil {
		slog.Warn("invitation notice producer unavailable, invitees get no notification", "error", err)
		return nil
	}
	return &Producer{client: client}
}

// Close flushes what is buffered and closes the client. Safe on nil.
func (p *Producer) Close() {
	if p == nil || p.client == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = p.client.Flush(ctx)
	p.client.Close()
}

// InvitationOffered publishes the notice. Safe on a nil Producer.
func (p *Producer) InvitationOffered(n domain.InvitationNotice) {
	if p == nil || p.client == nil || n.UserID == "" || n.InvitationID == "" {
		return
	}
	data, err := json.Marshal(InvitationOffered{
		InvitationID: n.InvitationID, UserID: n.UserID, WorkspaceName: n.WorkspaceName,
		InviterName: n.InviterName, Timestamp: time.Now().UnixMilli(),
	})
	if err != nil {
		slog.Warn("invitation notice not published", "error", err)
		return
	}
	p.client.TryProduce(context.Background(), &kgo.Record{Topic: InvitationTopic, Key: []byte(n.UserID), Value: data},
		func(_ *kgo.Record, err error) {
			if err != nil {
				slog.Warn("invitation notice dropped", "topic", InvitationTopic, "error", err)
			}
		})
}
