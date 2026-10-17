package domain

import (
	"context"

	"ngac-platform/ngac"
	"ngac-platform/pkg/policyclient"
	"ngac-platform/services/calendar/internal/store"
)

// authorize requires op for the user on a calendar's OA. It is fail-closed: no
// user, no OA, or an error from the policy service denies.
func (s *Service) authorize(ctx context.Context, userNodeID string, cal *store.Calendar, op string) error {
	if userNodeID == "" || cal == nil || cal.NgacOAID == "" {
		return denied("no %s access", op)
	}
	if ok, _ := policyclient.New(s.policyRead).Check(ctx, userNodeID, cal.NgacOAID, op); !ok {
		return denied("no %s access", op)
	}
	return nil
}

// calendarOps are the operations a calendar's access level is read from.
var calendarOps = []string{ngac.OpRead, ngac.OpWrite, ngac.OpShare, ngac.OpManage}

// accessLevels reports, from one batch call, how much of each calendar the user
// holds: manage, write, read, or "" for none. A calendar the user holds nothing
// on is absent from the map. An error from the policy service is returned,
// never read as "nothing held", so a caller can refuse to list anything.
func (s *Service) accessLevels(ctx context.Context, userNodeID string, cals []*store.Calendar) (map[string]string, error) {
	out := map[string]string{}
	var oaIDs []string
	seen := map[string]bool{}
	for _, c := range cals {
		if c.NgacOAID != "" && !seen[c.NgacOAID] {
			seen[c.NgacOAID] = true
			oaIDs = append(oaIDs, c.NgacOAID)
		}
	}
	if userNodeID == "" || len(oaIDs) == 0 {
		return out, nil
	}
	perms, err := policyclient.New(s.policyRead).BatchCheck(ctx, userNodeID, oaIDs, calendarOps)
	if err != nil {
		return nil, internal("check calendar access", err)
	}
	for _, c := range cals {
		switch {
		case perms.Has(c.NgacOAID, ngac.OpManage):
			out[c.ID] = AccessManage
		case perms.Has(c.NgacOAID, ngac.OpWrite):
			out[c.ID] = AccessWrite
		case perms.Has(c.NgacOAID, ngac.OpRead):
			out[c.ID] = AccessRead
		}
	}
	return out, nil
}

// accessTo is the access the caller has to a calendar of their workspace: what
// the policy grants, else free/busy for a colleague's personal calendar, else
// "" (nothing: the calendar is not theirs to see).
func accessTo(held map[string]string, c *store.Calendar) string {
	if a := held[c.ID]; a != "" {
		return a
	}
	if c.Kind == store.KindPersonal {
		return AccessFreeBusy
	}
	return ""
}
