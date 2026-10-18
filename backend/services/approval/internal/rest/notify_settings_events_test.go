package rest

import (
	"context"
	"errors"
	"testing"

	"ngac-platform/services/approval/internal/domain"
)

// prefsStore is the lifecycle store plus saved notification switches.
type prefsStore struct {
	*memStore
	pendingOff, resultOff map[string]bool
	err                   error
}

func (p *prefsStore) NotifyOffByNode(_ context.Context, nodes []string) (map[string]bool, map[string]bool, error) {
	if p.err != nil {
		return nil, nil, p.err
	}
	pending, result := map[string]bool{}, map[string]bool{}
	for _, n := range nodes {
		pending[n], result[n] = p.pendingOff[n], p.resultOff[n]
	}
	return pending, result, nil
}

func newPrefsLifecycle(t *testing.T, ps *prefsStore) *lifecycle {
	t.Helper()
	base := newLifecycle(t)
	ps.memStore = base.store
	base.h = NewHandler(domain.NewService(ps, allowPolicy{}), nil, base.pub)
	return base
}

// An approver who turned "đề nghị chờ bạn" off is named in the event so the
// notifier skips them; one who kept it on is not.
func TestEvents_CarryTheApproversWhoTurnedPendingOff(t *testing.T) {
	l := newPrefsLifecycle(t, &prefsStore{pendingOff: map[string]bool{"n-ap1": true}})
	l.create(t)

	evt := l.last(t)
	if len(evt.PendingNotifyOff) != 1 || evt.PendingNotifyOff[0] != "n-ap1" {
		t.Fatalf("pending_notify_off = %v, want [n-ap1]", evt.PendingNotifyOff)
	}
	if evt.NotifyResult == nil || !*evt.NotifyResult {
		t.Errorf("notify_result = %v, want true: the requester kept it on", evt.NotifyResult)
	}
}

func TestEvents_ApproverWhoKeptPendingOnIsNotListed(t *testing.T) {
	l := newPrefsLifecycle(t, &prefsStore{pendingOff: map[string]bool{"n-ap1": true}})
	id := l.create(t)

	if err := l.call(t, l.h.ApproveAction, "n-ap1", "tenant-a", body(map[string]any{"request_id": id})); err != nil {
		t.Fatalf("approve: %v", err)
	}

	evt := l.last(t)
	if len(evt.PendingNotifyOff) != 0 {
		t.Errorf("pending_notify_off = %v, want none: n-ap2, now pending, kept it on", evt.PendingNotifyOff)
	}
}

func TestEvents_CarryTheRequesterResultSwitch(t *testing.T) {
	l := newPrefsLifecycle(t, &prefsStore{resultOff: map[string]bool{"n-req": true}})
	id := l.create(t)

	if err := l.call(t, l.h.RejectAction, "n-ap1", "tenant-a", body(map[string]any{"request_id": id, "comment": "no"})); err != nil {
		t.Fatalf("reject: %v", err)
	}

	evt := l.last(t)
	if evt.NotifyResult == nil || *evt.NotifyResult {
		t.Fatalf("notify_result = %v, want false: the requester turned it off", evt.NotifyResult)
	}
}

// Settings that cannot be read leave the event as it was before the switches:
// everyone is notified, and the event still goes out.
func TestEvents_UnreadableSettingsNotifyEveryone(t *testing.T) {
	l := newPrefsLifecycle(t, &prefsStore{err: errors.New("tenant db down")})
	l.create(t)

	evt := l.last(t)
	if evt.NotifyResult != nil || len(evt.PendingNotifyOff) != 0 {
		t.Errorf("notify_result = %v pending_notify_off = %v, want both absent", evt.NotifyResult, evt.PendingNotifyOff)
	}
}
