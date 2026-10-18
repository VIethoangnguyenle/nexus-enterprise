package domain

import (
	"context"
	"fmt"
)

// NotifyPrefsStore reads people's approval notification switches.
type NotifyPrefsStore interface {
	// NotifyOffByNode returns, among the given user nodes, those whose saved
	// settings turn "đề nghị chờ bạn" (notify_pending) off and those that turn
	// the result of their own requests (notify_result) off. Someone who never
	// saved settings has both on.
	NotifyOffByNode(ctx context.Context, nodeIDs []string) (pendingOff, resultOff map[string]bool, err error)
}

// NotifyOptOuts is who among an event's audience asked not to be notified.
type NotifyOptOuts struct {
	// ResultOff: the requester turned off notifications about their requests' results.
	ResultOff bool
	// PendingOff: the approvers, among those the request now waits on, who turned
	// off "đề nghị chờ bạn".
	PendingOff []string
}

// NotifyOptOuts reads the switches of the event's requester and of the
// approvers it now waits on. Each person decides only their own notification.
// A store that keeps no settings has nobody opted out.
func (s *Service) NotifyOptOuts(ctx context.Context, aud *EventAudience) (NotifyOptOuts, error) {
	var out NotifyOptOuts
	nps, ok := s.store.(NotifyPrefsStore)
	if !ok || aud == nil || aud.Request == nil {
		return out, nil
	}
	nodes := append([]string{aud.Request.CreatedBy}, aud.AssigneeNodeIDs...)
	pendingOff, resultOff, err := nps.NotifyOffByNode(ctx, nodes)
	if err != nil {
		return out, fmt.Errorf("notification settings: %w", err)
	}
	out.ResultOff = resultOff[aud.Request.CreatedBy]
	for _, n := range aud.AssigneeNodeIDs {
		if pendingOff[n] {
			out.PendingOff = append(out.PendingOff, n)
		}
	}
	return out, nil
}
