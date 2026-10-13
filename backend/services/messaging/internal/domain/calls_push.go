// Package domain — the notifications calls send to people's devices.
package domain

import (
	"context"
	"fmt"
	"slices"
	"time"

	"ngac-platform/ngac"
	"ngac-platform/pkg/push"
	pb "ngac-platform/proto/messaging"
	"ngac-platform/services/messaging/internal/store"
)

// callMentionInviteFor is how long after a group call starts a person
// @mentioned in its conversation is invited to it rather than just mentioned.
const callMentionInviteFor = 2 * time.Minute

// pushCallChange tells a direct call's other person's devices that it rings,
// and — when it stops ringing unanswered (missed, or cancelled by the caller) —
// replaces that notification with a missed call. Declined and answered calls
// were the person's own doing: nothing more is pushed. The person's open app
// rings by itself, so a device showing the conversation is not pushed.
func (s *Service) pushCallChange(ctx context.Context, c CallChange) {
	if s.push == nil || c.Call == nil {
		return
	}
	if c.Type != CallChangeRing && c.Type != CallChangeMissed && c.Type != CallChangeCancel {
		return
	}
	call := c.Call
	s.later(ctx, func(ctx context.Context) {
		for _, userID := range c.Users {
			if userID == "" || userID == call.StartedBy || !s.wantsCallPush(ctx, userID, call.ChannelId) {
				continue
			}
			m := push.Message{ExternalIDs: []string{userID}, CollapseID: "call-" + call.Id, Title: call.StartedByName}
			if c.Type == CallChangeRing {
				m.Body = "đang gọi thoại cho bạn"
				if call.Kind == "video" {
					m.Body = "đang gọi video cho bạn"
				}
				m.URL = "/calls/" + call.Id + "?answer=" + call.Kind
			} else {
				m.Body = CallCardText(call.Kind, callMissed, nil, nil)
				m.URL = "/channels/" + call.ChannelId
			}
			s.send(ctx, m)
		}
	})
}

// wantsCallPush reports whether a person takes incoming-call notifications
// now: the kind is on, it is not their quiet hours, and they do not have the
// conversation open (where the app rings).
func (s *Service) wantsCallPush(ctx context.Context, userID, channelID string) bool {
	prefs, err := s.store.GetPushPreferences(ctx, userID)
	if err != nil || !prefs.IncomingCall || inQuietHours(prefs, s.pushNow()) {
		return false
	}
	return s.push.viewing == nil || !s.push.viewing.IsViewing(userID, channelID)
}

// InviteToCall asks people to join an open call: those of them who may write
// in its conversation (and so may join) are told on their devices and in their
// open app. The inviter needs write too. At most MaxMentionPushes are invited at
// once; anyone else named is skipped without a word.
func (s *Service) InviteToCall(ctx context.Context, callID, userID, userNodeID string, invitees []string) error {
	call, err := s.authorizeCall(ctx, callID, userNodeID, ngac.OpWrite)
	if err != nil {
		return err
	}
	if !slices.Contains(openCallStatuses, call.Status) {
		return ErrCallEnded
	}
	ch, err := s.loadChannel(ctx, call.ChannelID)
	if err != nil {
		return err
	}
	if len(invitees) > MaxMentionPushes {
		invitees = invitees[:MaxMentionPushes]
	}
	nodes, err := s.store.WorkspaceNodes(ctx, ch.WorkspaceID, invitees)
	if err != nil {
		return err
	}
	var invited []string
	for _, id := range invitees {
		node := nodes[id]
		if id == userID || node == "" || slices.Contains(invited, id) || s.checkAccess(ctx, node, ch.NGACOaID, ngac.OpWrite) != nil {
			continue
		}
		invited = append(invited, id)
	}
	if len(invited) == 0 {
		return nil
	}
	out, err := s.callProto(ctx, call)
	if err != nil {
		return err
	}
	s.announceCall(ctx, CallChange{Type: CallChangeInvite, Call: out, Users: invited})
	if s.push == nil {
		return nil
	}
	inviter, _ := s.store.DisplayName(ctx, userID)
	s.later(ctx, func(ctx context.Context) {
		for _, id := range invited {
			prefs, err := s.store.GetPushPreferences(ctx, id)
			if err != nil || !prefs.Mention || inQuietHours(prefs, s.pushNow()) {
				continue
			}
			s.send(ctx, callInvitePush(ch, call.ID, inviter, id))
		}
	})
	return nil
}

func callInvitePush(ch *store.Channel, callID, inviter, userID string) push.Message {
	return push.Message{
		ExternalIDs: []string{userID}, CollapseID: "call-" + callID,
		Title: ch.Name, Body: fmt.Sprintf("%s mời bạn vào cuộc gọi", inviter), URL: "/calls/" + callID,
	}
}

// mentionInvite returns the group call a mention in msg invites to: a call open
// in the conversation for less than two minutes, mentioned in its main stream.
func (s *Service) mentionInvite(ctx context.Context, ch *store.Channel, msg *pb.Message) *store.Call {
	if ch.ChannelType == "dm" || msg.ParentMessageId != "" || len(msg.Mentions) == 0 {
		return nil
	}
	call, err := s.store.OpenCall(ctx, ch.ID)
	if err != nil || call == nil || call.Status != callActive || s.pushNow().Sub(call.CreatedAt) > callMentionInviteFor {
		return nil
	}
	return call
}
