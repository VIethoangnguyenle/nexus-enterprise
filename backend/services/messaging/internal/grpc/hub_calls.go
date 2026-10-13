package grpc

import (
	"context"
	"log/slog"

	pb "ngac-platform/proto/messaging"
	"ngac-platform/services/messaging/internal/domain"
)

// CallChanged tells live sessions about a committed call change: the
// conversation's subscribers get its state (and the card, when its text
// changed); the people the change names get it on every device, which is how a
// direct call rings and stops ringing wherever they are signed in.
func (h *Hub) CallChanged(_ context.Context, c domain.CallChange) {
	if c.Call == nil {
		return
	}
	ev := callEvent(c)
	env := &pb.ServerEnvelope{Payload: &pb.ServerEnvelope_CallEvent{CallEvent: ev}}
	if c.Card != nil {
		h.BroadcastMessageEdited(c.Call.ChannelId, c.Card)
	}
	if len(c.Users) == 0 {
		h.publishToChannel(c.Call.ChannelId, env)
		return
	}
	data := marshalEnvelope(env)
	if data == nil {
		return
	}
	for _, userID := range c.Users {
		if userID == "" {
			continue
		}
		if h.rdb != nil {
			err := h.rdb.Publish(h.ctx, "user:"+userID, data).Err()
			if err == nil {
				continue
			}
			slog.Warn("redis call event publish failed", "error", err)
		}
		h.sendToUser(userID, data)
	}
	// Whoever follows the conversation sees the state too (the card's button).
	state := callEvent(c)
	state.Type = domain.CallChangeState
	h.publishToChannel(c.Call.ChannelId, &pb.ServerEnvelope{Payload: &pb.ServerEnvelope_CallEvent{CallEvent: state}})
}

func callEvent(c domain.CallChange) *pb.CallEvent {
	call := c.Call
	ids := make([]string, 0, len(call.Participants))
	for _, p := range call.Participants {
		ids = append(ids, p.UserId)
	}
	return &pb.CallEvent{
		Type: c.Type, CallId: call.Id, ChannelId: call.ChannelId, Kind: call.Kind, Status: call.Status,
		CallerId: call.StartedBy, CallerName: call.StartedByName, CallerAvatarUrl: call.StartedByAvatarUrl,
		ParticipantCount: int32(len(call.Participants)), ParticipantIds: ids,
		StartedAt: call.StartedAt, EndedAt: call.EndedAt,
	}
}
