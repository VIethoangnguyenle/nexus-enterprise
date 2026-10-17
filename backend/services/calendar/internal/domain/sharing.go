package domain

import (
	"context"
	"errors"
	"sort"

	"ngac-platform/ngac"
	"ngac-platform/pkg/grpcauth"
	"ngac-platform/pkg/provision"
	"ngac-platform/pkg/realtime"
	policypb "ngac-platform/proto/policy"
	"ngac-platform/services/calendar/internal/events"
	"ngac-platform/services/calendar/internal/store"
)

// formerMember is how a share is listed when its grantee has left the workspace.
const formerMember = "Thành viên cũ"

// authorizeSharing admits the caller to manage the shares of a personal
// calendar: share and manage on its OA, both, and a personal calendar at all.
// The checks come first, so a stranger learns nothing about the calendar's kind.
func (s *Service) authorizeSharing(ctx context.Context, wsID, calID string) (grpcauth.Caller, *store.Workspace, *store.Calendar, error) {
	who, ws, err := s.requireMember(ctx, wsID)
	if err != nil {
		return who, nil, nil, err
	}
	cal, err := s.loadCalendar(ctx, wsID, calID)
	if err != nil {
		return who, nil, nil, err
	}
	for _, op := range []string{ngac.OpShare, ngac.OpManage} {
		if err := s.authorize(ctx, who.NGACNodeID, cal, op); err != nil {
			return who, nil, nil, err
		}
	}
	if cal.Kind != store.KindPersonal {
		return who, nil, nil, invalid("only a personal calendar is shared")
	}
	return who, ws, cal, nil
}

// ListShares returns who a personal calendar is shared with, and at what level.
func (s *Service) ListShares(ctx context.Context, wsID, calID string) ([]*ShareView, error) {
	_, ws, cal, err := s.authorizeSharing(ctx, wsID, calID)
	if err != nil {
		return nil, err
	}
	shares, err := s.store.Shares(ctx, cal.ID)
	if err != nil {
		return nil, internal("list shares", err)
	}
	nodes := make([]string, len(shares))
	for i, sh := range shares {
		nodes[i] = sh.GranteeNodeID
	}
	people, err := s.store.People(ctx, ws.ID, nodes)
	if err != nil {
		return nil, internal("name grantees", err)
	}
	out := make([]*ShareView, 0, len(shares))
	for _, sh := range shares {
		who := refOf(people[sh.GranteeNodeID])
		if who == nil {
			who = formerMemberRef(sh.GranteeNodeID)
		}
		out = append(out, &ShareView{Grantee: who, Level: sh.Level})
	}
	return out, nil
}

// PutShare shares a personal calendar with a person of the workspace at one of
// three levels, or changes the level of an existing share. The level is a name;
// the operations come from package ngac, never from the request. The grant is an
// association from the grantee's personal UA to the calendar's OA, and a second
// grant to the same pair replaces the first in one step, so a change of level is
// never a moment without the share.
func (s *Service) PutShare(ctx context.Context, wsID, calID, granteeNode, level string) (*ShareView, error) {
	who, ws, cal, err := s.authorizeSharing(ctx, wsID, calID)
	if err != nil {
		return nil, err
	}
	ops, ok := ngac.CalendarShareOps(level)
	if !ok {
		return nil, invalid("level must be view, edit or manage")
	}
	switch granteeNode {
	case "":
		return nil, invalid("a person is required")
	case who.NGACNodeID, cal.OwnerNodeID:
		return nil, invalid("a calendar is not shared with its owner")
	}
	person, err := s.requirePerson(ctx, ws, granteeNode)
	if err != nil {
		return nil, err
	}

	previous, err := s.store.ShareOf(ctx, cal.ID, granteeNode)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return nil, internal("load share", err)
	}
	ua, err := provision.EnsurePersonalUA(ctx, s.policyRead, s.policyWrite, granteeNode)
	if err != nil {
		return nil, internal("find the grantee's attribute", err)
	}
	if _, err := s.policyWrite.CreateAssociation(ctx, &policypb.CreateAssociationRequest{UaId: ua, OaId: cal.NgacOAID, Operations: ops}); err != nil {
		return nil, internal("grant the share", err)
	}
	rec := &store.Share{CalendarID: cal.ID, GranteeNodeID: granteeNode, Level: level, GrantedBy: who.NGACNodeID}
	if err := s.store.SaveShare(ctx, rec); err != nil {
		s.restoreGrant(ctx, ua, cal, previous)
		return nil, internal("record the share", err)
	}
	// The grantee finds the calendar on their list; an existing choice of colour
	// or visibility is kept.
	if err := s.store.SaveSubscription(ctx, granteeNode, cal.ID, nil, nil); err != nil {
		return nil, internal("subscribe the grantee", err)
	}

	e := realtime.For(ctx, realtime.DomainCalendar, realtime.KindShareCreated, ws.ID, cal.ID)
	e.UserNodeIDs = distinct(cal.OwnerNodeID, granteeNode, who.NGACNodeID)
	s.emit(e)
	s.notify(ctx, events.TypeShared, ws, cal, nil, []string{granteeNode}, noticeExtras{actor: who})

	saved, err := s.store.ShareOf(ctx, cal.ID, granteeNode)
	if err != nil {
		return nil, internal("load share", err)
	}
	return &ShareView{Grantee: refOf(person), Level: saved.Level}, nil
}

// restoreGrant puts the graph back as it was before a grant whose record could
// not be stored: the previous level, or none.
func (s *Service) restoreGrant(ctx context.Context, ua string, cal *store.Calendar, previous *store.Share) {
	if previous != nil {
		if ops, ok := ngac.CalendarShareOps(previous.Level); ok {
			_, err := s.policyWrite.CreateAssociation(ctx, &policypb.CreateAssociationRequest{UaId: ua, OaId: cal.NgacOAID, Operations: ops})
			if err != nil {
				logRevertFailure("restore previous level", err)
			}
			return
		}
	}
	if _, err := s.policyWrite.RemoveAssociation(ctx, &policypb.RemoveAssociationRequest{UaId: ua, OaId: cal.NgacOAID}); err != nil {
		logRevertFailure("remove unrecorded grant", err)
	}
}

// DeleteShare ends a share. The next decision for the former grantee is DENY.
func (s *Service) DeleteShare(ctx context.Context, wsID, calID, granteeNode string) error {
	who, ws, cal, err := s.authorizeSharing(ctx, wsID, calID)
	if err != nil {
		return err
	}
	if granteeNode == "" || granteeNode == cal.OwnerNodeID {
		return invalid("the owner's access is not a share")
	}
	ua, err := provision.FindPersonalUA(ctx, s.policyRead, s.policyWrite, granteeNode)
	if err != nil {
		return internal("find the grantee's attribute", err)
	}
	if ua != "" {
		if _, err := s.policyWrite.RemoveAssociation(ctx, &policypb.RemoveAssociationRequest{UaId: ua, OaId: cal.NgacOAID}); err != nil {
			return internal("revoke the share", err)
		}
	}
	if _, err := s.store.DeleteShare(ctx, cal.ID, granteeNode); err != nil {
		return internal("forget the share", err)
	}

	e := realtime.For(ctx, realtime.DomainCalendar, realtime.KindShareRevoked, ws.ID, cal.ID)
	e.UserNodeIDs = distinct(cal.OwnerNodeID, granteeNode, who.NGACNodeID)
	s.emit(e)
	return nil
}

// distinct returns the non-empty values once each, in a stable order.
func distinct(values ...string) []string {
	seen := map[string]bool{}
	var out []string
	for _, v := range values {
		if v != "" && !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	sort.Strings(out)
	return out
}
