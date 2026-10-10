package grpc

import (
	"log/slog"
	"strings"

	"google.golang.org/protobuf/proto"

	pb "ngac-platform/proto/messaging"
	"ngac-platform/services/messaging/internal/events"
)

// messageToChatMessage converts a gRPC Message to a WebSocket ChatMessage.
func messageToChatMessage(m *pb.Message) *pb.ChatMessage {
	return &pb.ChatMessage{
		Id:              m.Id,
		ChannelId:       m.ChannelId,
		SenderId:        m.SenderId,
		SenderName:      m.SenderName,
		Content:         m.Content,
		CreatedAt:       m.CreatedAt,
		MessageType:     m.MessageType,
		ParentMessageId: m.ParentMessageId,
		ReplyCount:      m.ReplyCount,
	}
}

// marshalEnvelope serializes a ServerEnvelope to protobuf bytes.
func marshalEnvelope(env *pb.ServerEnvelope) []byte {
	data, err := proto.Marshal(env)
	if err != nil {
		slog.Error("failed to marshal server envelope", "error", err)
		return nil
	}
	return data
}

// BroadcastToChannel publishes to Redis (cross-instance) or local broadcast.
func (h *Hub) BroadcastToChannel(channelID string, msg *pb.Message) {
	chatMsg := messageToChatMessage(msg)
	env := &pb.ServerEnvelope{
		Payload: &pb.ServerEnvelope_ChatMessage{ChatMessage: chatMsg},
	}
	data := marshalEnvelope(env)
	if data == nil {
		return
	}

	if h.rdb != nil {
		if err := h.rdb.Publish(h.ctx, redisChanKey(channelID), data).Err(); err != nil {
			slog.Warn("redis publish failed, falling back to local", "error", err)
			h.broadcastLocal(channelID, data)
		}
		return
	}
	h.broadcastLocal(channelID, data)
}

// broadcastLocal sends data to local WebSocket clients in a channel.
func (h *Hub) broadcastLocal(channelID string, data []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	clients, ok := h.channels[channelID]
	if !ok {
		return
	}
	for client := range clients {
		select {
		case client.send <- data:
		default:
			close(client.send)
			delete(clients, client)
		}
	}
}

// broadcastTyping sends typing indicator via Redis or local.
func (h *Hub) broadcastTyping(channelID, username string, sender *Client) {
	env := &pb.ServerEnvelope{
		Payload: &pb.ServerEnvelope_TypingEvent{
			TypingEvent: &pb.TypingEvent{
				ChannelId: channelID,
				Username:  username,
			},
		},
	}
	data := marshalEnvelope(env)
	if data == nil {
		return
	}

	if h.rdb != nil {
		if err := h.rdb.Publish(h.ctx, redisChanKey(channelID), data).Err(); err != nil {
			slog.Warn("redis typing publish failed", "error", err)
		}
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()
	if clients, ok := h.channels[channelID]; ok {
		for other := range clients {
			if other != sender {
				select {
				case other.send <- data:
				default:
				}
			}
		}
	}
}

// subscribeRedis listens for cross-instance traffic and delivers it to local
// clients: channel:<id> to that channel's subscribers, user:<id> to that user's
// sessions, presence:<tenant> to sessions in that tenant.
func (h *Hub) subscribeRedis() {
	pubsub := h.rdb.PSubscribe(h.ctx, "channel:*", "user:*", notifyKeyPrefix+"*", presenceKeyPrefix+"*", revokeKeyPrefix+"*",
		workspaceEventPrefix+"*", channelEventPrefix+"*", userEventPrefix+"*", permEventPrefix+"*", workspaceRevokePrefix+"*")
	defer pubsub.Close()

	slog.Info("redis pub/sub subscriber started")

	ch := pubsub.Channel()
	for {
		select {
		case <-h.ctx.Done():
			slog.Info("redis subscriber shutting down")
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			payload := []byte(msg.Payload)

			switch {
			case h.deliverRedisDomain(msg.Channel, payload):
			case strings.HasPrefix(msg.Channel, workspaceRevokePrefix):
				h.revokeWorkspaceLocal(strings.TrimPrefix(msg.Channel, workspaceRevokePrefix), msg.Payload)
			case strings.HasPrefix(msg.Channel, revokeKeyPrefix):
				h.revokeLocal(strings.TrimPrefix(msg.Channel, revokeKeyPrefix), msg.Payload)
			case strings.HasPrefix(msg.Channel, presenceKeyPrefix):
				h.broadcastToTenant(strings.TrimPrefix(msg.Channel, presenceKeyPrefix), payload)
			case strings.HasPrefix(msg.Channel, notifyKeyPrefix):
				if tenant, user, ok := strings.Cut(strings.TrimPrefix(msg.Channel, notifyKeyPrefix), ":"); ok {
					h.sendToUserInTenant(tenant, user, payload)
				}
			case strings.HasPrefix(msg.Channel, "user:"):
				h.sendToUser(strings.TrimPrefix(msg.Channel, "user:"), payload)
			case strings.HasPrefix(msg.Channel, "channel:"):
				h.broadcastLocal(strings.TrimPrefix(msg.Channel, "channel:"), payload)
			}
		}
	}
}

// sendToUser delivers data to all WebSocket clients for a given user.
func (h *Hub) sendToUser(userID string, data []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if clients, ok := h.users[userID]; ok {
		for client := range clients {
			select {
			case client.send <- data:
			default:
			}
		}
	}
}

// broadcastToTenant delivers data to every local session of the given tenant.
// An empty tenant matches no one: a session without a tenant has no
// colleagues, and treating "" as a tenant would pool every such session.
func (h *Hub) broadcastToTenant(tenantID string, data []byte) {
	if tenantID == "" {
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, clients := range h.users {
		for client := range clients {
			if client.tenantID != tenantID {
				continue
			}
			select {
			case client.send <- data:
			default:
			}
		}
	}
}

// notifyKeyPrefix namespaces the Redis channels that carry a notification to one
// user's sessions inside one workspace: notify:<workspace>:<user>.
const notifyKeyPrefix = "notify:"

// SendNotification pushes a notification to the recipient's open sessions in the
// notification's workspace, and to no other session of theirs: a person open in
// two workspaces is not told, in one, about the other. An empty workspace is a
// personal notification (an invitation) and reaches every session of the user.
func (h *Hub) SendNotification(workspaceID, userID string, notif *pb.Notification) {
	env := &pb.ServerEnvelope{
		Payload: &pb.ServerEnvelope_Notification{
			Notification: &pb.NotificationEvent{
				Id:          notif.Id,
				Type:        notif.Type,
				TargetType:  notif.TargetType,
				TargetId:    notif.TargetId,
				ActorUserId: notif.ActorUserId,
				ActorName:   notif.ActorName,
				TargetName:  notif.TargetName,
				WorkspaceId: notif.WorkspaceId,
				Params:      notif.Params,
				CreatedAt:   notif.CreatedAt,
			},
		},
	}
	data := marshalEnvelope(env)
	if data == nil || userID == "" {
		return
	}

	if workspaceID == "" {
		if h.rdb != nil {
			if err := h.rdb.Publish(h.ctx, "user:"+userID, data).Err(); err != nil {
				slog.Warn("redis notification publish failed", "error", err)
			}
			return
		}
		h.sendToUser(userID, data)
		return
	}
	if h.rdb != nil {
		if err := h.rdb.Publish(h.ctx, notifyKeyPrefix+workspaceID+":"+userID, data).Err(); err != nil {
			slog.Warn("redis notification publish failed", "error", err)
		}
		return
	}
	h.sendToUserInTenant(workspaceID, userID, data)
}

// sendToUserInTenant delivers data to the user's sessions that belong to the tenant.
func (h *Hub) sendToUserInTenant(tenantID, userID string, data []byte) {
	if tenantID == "" || userID == "" {
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for client := range h.users[userID] {
		if client.tenantID == tenantID {
			select {
			case client.send <- data:
			default:
			}
		}
	}
}

// SendUnreadCount pushes an unread count update to a user.
func (h *Hub) SendUnreadCount(userID string, count int32) {
	env := &pb.ServerEnvelope{
		Payload: &pb.ServerEnvelope_UnreadCount{
			UnreadCount: &pb.UnreadCountEvent{Count: count},
		},
	}
	data := marshalEnvelope(env)
	if data == nil {
		return
	}

	if h.rdb != nil {
		if err := h.rdb.Publish(h.ctx, "user:"+userID, data).Err(); err != nil {
			slog.Warn("redis unread count publish failed", "error", err)
		}
		return
	}

	h.sendToUser(userID, data)
}

// BroadcastThreadReply sends a thread reply event to channel subscribers.
func (h *Hub) BroadcastThreadReply(channelID string, msg *pb.Message) {
	chatMsg := messageToChatMessage(msg)
	env := &pb.ServerEnvelope{
		Payload: &pb.ServerEnvelope_ThreadReply{
			ThreadReply: &pb.ThreadReplyEvent{
				Message:         chatMsg,
				ParentMessageId: msg.ParentMessageId,
			},
		},
	}
	data := marshalEnvelope(env)
	if data == nil {
		return
	}

	if h.rdb != nil {
		if err := h.rdb.Publish(h.ctx, redisChanKey(channelID), data).Err(); err != nil {
			slog.Warn("redis thread reply publish failed", "error", err)
			h.broadcastLocal(channelID, data)
		}
		return
	}
	h.broadcastLocal(channelID, data)
}

// BroadcastApprovalEvent sends an approval status change to the users the
// notice names, and to no one else.
//
// It used to go to every connected session of every tenant, which handed the
// request ID, template name and actor of one tenant's approvals to everyone on
// the platform. Delivery is now by recipient: a session receives the event
// only if its NGAC user node is in RecipientNodeIDs and, when the notice
// carries a tenant, only if the session belongs to that tenant. A notice that
// names nobody is dropped.
func (h *Hub) BroadcastApprovalEvent(n events.ApprovalNotice) {
	recipients := make(map[string]bool, len(n.RecipientNodeIDs))
	for _, id := range n.RecipientNodeIDs {
		if id != "" {
			recipients[id] = true
		}
	}
	if len(recipients) == 0 {
		return
	}

	env := &pb.ServerEnvelope{
		Payload: &pb.ServerEnvelope_ApprovalEvent{
			ApprovalEvent: &pb.ApprovalEvent{
				RequestId:    n.RequestID,
				Status:       n.Status,
				Action:       n.Action,
				ActorNodeId:  n.ActorNodeID,
				TemplateName: n.TemplateName,
				TenantId:     n.TenantID,
				WorkspaceId:  n.WorkspaceID,
			},
		},
	}
	data := marshalEnvelope(env)
	if data == nil {
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, clients := range h.users {
		for client := range clients {
			if !recipients[client.ngacNodeID] {
				continue
			}
			if n.TenantID != "" && client.tenantID != n.TenantID {
				continue
			}
			select {
			case client.send <- data:
			default:
			}
		}
	}
}

// BroadcastPresence sends a presence event (online/offline) to the sessions of
// the given tenant. Presence says who is around; outside the tenant it is a
// directory of another organisation's people. A session with no tenant
// announces itself to no one.
func (h *Hub) BroadcastPresence(tenantID, userID, username, status string) {
	if tenantID == "" {
		return
	}
	env := &pb.ServerEnvelope{
		Payload: &pb.ServerEnvelope_PresenceEvent{
			PresenceEvent: &pb.PresenceEvent{
				UserId:   userID,
				Username: username,
				Status:   status,
			},
		},
	}
	data := marshalEnvelope(env)
	if data == nil {
		return
	}

	if h.rdb != nil {
		// Publish to all instances, on the tenant's own presence channel.
		if err := h.rdb.Publish(h.ctx, presenceKeyPrefix+tenantID, data).Err(); err != nil {
			slog.Warn("redis presence publish failed", "error", err)
		}
		return
	}

	h.broadcastToTenant(tenantID, data)
}
