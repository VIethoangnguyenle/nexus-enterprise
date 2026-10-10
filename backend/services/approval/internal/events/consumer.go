// Package events provides Kafka consumer for policy change reconciliation.
// Listens to ngac.graph.mutated events and updates approval assignments when
// users are added to or removed from UAs (role changes).
package events

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"
	"github.com/twmb/franz-go/pkg/kgo"

	"ngac-platform/services/approval/internal/domain"
)

// GraphMutatedEvent mirrors the policy service's published event schema.
type GraphMutatedEvent struct {
	MutationType string   `json:"mutation_type"`
	NodeIDs      []string `json:"node_ids"`
	ChildType    string   `json:"child_type,omitempty"`
	ParentType   string   `json:"parent_type,omitempty"`
	Timestamp    int64    `json:"timestamp"`
}

// ReconciliationStore defines the store methods needed for reconciliation.
// Separated from the full Store interface for minimal coupling.
type ReconciliationStore interface {
	// FindPendingByGrantSource finds pending assignments whose grant_source
	// contains the given pattern (e.g., "role:KeToan_Chief" or "department:KeToan_Dept").
	FindPendingByGrantSource(ctx context.Context, grantSourcePattern string) ([]*domain.AssignmentRecord, error)
	// FindPendingByUserAndSource finds a user's pending assignments matching a grant source pattern.
	FindPendingByUserAndSource(ctx context.Context, userNodeID, grantSourcePattern string) ([]*domain.AssignmentRecord, error)
	// InsertAssignments inserts new approval assignments.
	InsertAssignments(ctx context.Context, assignments []*domain.AssignmentRecord) error
	// UpdateAssignmentStatus updates an assignment's status.
	UpdateAssignmentStatus(ctx context.Context, id, status, comment string) error
	// InsertAuditEntry appends an audit record.
	InsertAuditEntry(ctx context.Context, entry *domain.AuditEntry) error
}

// ReconciliationConsumer listens for NGAC graph mutations and reconciles
// pending approval assignments when user roles change.
type ReconciliationConsumer struct {
	client *kgo.Client
	store  ReconciliationStore
}

// NewReconciliationConsumer creates a consumer connected to the given brokers.
func NewReconciliationConsumer(brokers []string, store ReconciliationStore) (*ReconciliationConsumer, error) {
	client, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
		kgo.ConsumerGroup("approval-reconciliation"),
		kgo.ConsumeTopics("ngac.graph.mutated"),
		kgo.AllowAutoTopicCreation(),
	)
	if err != nil {
		return nil, fmt.Errorf("create kafka client: %w", err)
	}

	slog.Info("reconciliation consumer connected", "brokers", brokers, "topic", "ngac.graph.mutated")
	return &ReconciliationConsumer{client: client, store: store}, nil
}

// Run starts the consumer loop. Blocks until context is cancelled.
func (c *ReconciliationConsumer) Run(ctx context.Context) {
	for {
		fetches := c.client.PollFetches(ctx)
		if ctx.Err() != nil {
			return
		}
		if errs := fetches.Errors(); len(errs) > 0 {
			for _, e := range errs {
				slog.Warn("kafka fetch error", "topic", e.Topic, "error", e.Err)
			}
			continue
		}

		fetches.EachRecord(func(record *kgo.Record) {
			c.handleRecord(ctx, record)
		})
	}
}

// Close shuts down the consumer.
func (c *ReconciliationConsumer) Close() {
	if c != nil && c.client != nil {
		c.client.Close()
	}
}

// handleRecord processes a single Kafka record.
func (c *ReconciliationConsumer) handleRecord(ctx context.Context, record *kgo.Record) {
	var evt GraphMutatedEvent
	if err := json.Unmarshal(record.Value, &evt); err != nil {
		slog.Warn("unmarshal graph event", "error", err)
		return
	}

	// Only react to user↔UA assignment changes
	if evt.ChildType != "U" || evt.ParentType != "UA" {
		return
	}

	if len(evt.NodeIDs) < 2 {
		return
	}

	userNodeID := evt.NodeIDs[0]
	uaNodeID := evt.NodeIDs[1]

	switch evt.MutationType {
	case "create_assignment":
		c.handleUserAddedToUA(ctx, userNodeID, uaNodeID)
	case "remove_assignment":
		c.handleUserRemovedFromUA(ctx, userNodeID, uaNodeID)
	}
}

// handleUserAddedToUA does nothing, on purpose. A role or department step is one
// group row; a member is matched to it when they act (and in the pending list),
// by the memberships the policy service reports at that moment. Copying the
// request into a row of the new member here would put two rows for one person on
// the same step.
func (c *ReconciliationConsumer) handleUserAddedToUA(_ context.Context, _, _ string) {}

// handleUserRemovedFromUA revokes pending assignments for the user
// that were granted by the removed UA.
func (c *ReconciliationConsumer) handleUserRemovedFromUA(ctx context.Context, userNodeID, uaNodeID string) {
	patterns := []string{
		fmt.Sprintf("role:%s", uaNodeID),
		fmt.Sprintf("department:%s", uaNodeID),
	}

	revokedCount := 0
	for _, pattern := range patterns {
		assignments, err := c.store.FindPendingByUserAndSource(ctx, userNodeID, pattern)
		if err != nil {
			continue
		}

		for _, a := range assignments {
			if !strings.Contains(a.GrantSource, pattern) {
				continue
			}

			if err := c.store.UpdateAssignmentStatus(ctx, a.ID, "revoked", "policy_change: user removed from UA"); err != nil {
				slog.Error("revoke assignment", "id", a.ID, "error", err)
				continue
			}
			revokedCount++

			c.auditReconciliation(ctx, a.RequestID, "revoked_policy_change", userNodeID, a.StepOrder,
				map[string]string{
					"reason":       "user_removed_from_ua",
					"ua_node_id":   uaNodeID,
					"grant_source": a.GrantSource,
				})
		}
	}

	if revokedCount > 0 {
		slog.Info("reconciled: user removed from UA",
			"user", userNodeID, "ua", uaNodeID,
			"revoked_count", revokedCount)
	}
}

// auditReconciliation logs a reconciliation action to the audit trail.
func (c *ReconciliationConsumer) auditReconciliation(ctx context.Context, requestID, action, actorNodeID string, stepOrder int, detail map[string]string) {
	detailJSON, _ := json.Marshal(detail)
	c.store.InsertAuditEntry(ctx, &domain.AuditEntry{
		ID:          uuid.New().String(),
		RequestID:   requestID,
		Action:      action,
		ActorNodeID: actorNodeID,
		StepOrder:   stepOrder,
		DetailJSON:  string(detailJSON),
	})
}
