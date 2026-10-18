package domain

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"ngac-platform/ngac"
	"ngac-platform/pkg/realtime"
	policypb "ngac-platform/proto/policy"
)

// ErrGroupCreationLocked refuses to grant create_channel on Channels while group
// creation is locked to administrators. It is a conflict with the workspace's
// current rule, not a malformed request, so the REST edge answers it with 409.
var ErrGroupCreationLocked = errors.New("group creation is locked to administrators")

// The lock "Ai được tạo nhóm = chỉ quản trị viên" is implemented by removing the
// right, never by a prohibition (docs/specs/user-settings): create_channel leaves
// the Members → Channels association and every custom role's Channels grant, and
// the Owners keep their own association. There is no lock row and no actor; the
// state is whether the Members association still holds create_channel.

// channelsLockState is what the graph says about group creation in one workspace.
type channelsLockState struct {
	channelsOA string
	membersUA  string
	// membersOps is what Members hold on the Channels OA now.
	membersOps []string
}

func (st *channelsLockState) rule() string { return ngac.GroupCreationRule(st.membersOps) }

// readChannelsLock reads the lock state from the writer's graph: a replica that is
// behind would report a lock that was just lifted, or the reverse.
func (s *Service) readChannelsLock(ctx context.Context, ws *WorkspaceResult) (*channelsLockState, error) {
	oas, err := s.areaOAs(ctx, ws)
	if err != nil {
		return nil, err
	}
	channelsOA, ok := oas[ngac.AreaChannels]
	if !ok {
		return nil, fmt.Errorf("%w: this workspace has no %s area", ErrNotFound, ngac.AreaChannels)
	}
	membersUA, err := s.FindUAByName(ctx, ws.ID, ngac.MembersUAName(ngac.WorkspaceID(ws.ID)))
	if err != nil {
		return nil, err
	}
	assocs, err := s.policyWrite.GetAssociations(ctx, &policypb.GetAssociationsRequest{UaId: membersUA})
	if err != nil {
		return nil, fmt.Errorf("read members grants: %w", err)
	}
	st := &channelsLockState{channelsOA: channelsOA, membersUA: membersUA}
	for _, a := range assocs.GetAssociations() {
		if a.OaId == channelsOA {
			st.membersOps = append(st.membersOps, a.Operations...)
		}
	}
	return st, nil
}

// groupCreationRuleOf is the rule of a workspace for any reader: a personal
// workspace has no locks, so its rule is always everyone.
func (s *Service) groupCreationRuleOf(ctx context.Context, ws *WorkspaceResult) (string, error) {
	if ws.Type == TypePersonal {
		return ngac.GroupCreationEveryone, nil
	}
	st, err := s.readChannelsLock(ctx, ws)
	if err != nil {
		return "", err
	}
	return st.rule(), nil
}

// GroupCreationRule tells a member who may create groups in the workspace:
// ngac.GroupCreationEveryone or ngac.GroupCreationAdmins.
func (s *Service) GroupCreationRule(ctx context.Context, callerNodeID, wsID string) (string, error) {
	ws, err := s.authorizeMember(ctx, callerNodeID, wsID)
	if err != nil {
		return "", err
	}
	return s.groupCreationRuleOf(ctx, ws)
}

// SetGroupCreationRule locks group creation to administrators or opens it to
// everyone again, and answers with the rule read back from the graph. The caller
// needs manage on the workspace's Mgmt OA, like every other change to the
// workspace as a whole; a personal workspace has no locks.
//
// Locking removes the right from every custom role first and from Members last,
// so a write that fails half-way leaves the graph still reading "everyone" and
// the same request finishes the job. Unlocking restores the Members association
// only: custom roles are granted again by an administrator.
func (s *Service) SetGroupCreationRule(ctx context.Context, callerNodeID, wsID, rule string) (string, error) {
	ws, err := s.authorizeAdmin(ctx, callerNodeID, wsID, ngac.OpManage)
	if err != nil {
		return "", err
	}
	if ws.Type == TypePersonal {
		return "", fmt.Errorf("%w: a personal workspace has no locks", ErrInvalidInput)
	}
	if rule != ngac.GroupCreationAdmins && rule != ngac.GroupCreationEveryone {
		return "", fmt.Errorf("%w: group_creation must be %q or %q", ErrInvalidInput, ngac.GroupCreationEveryone, ngac.GroupCreationAdmins)
	}
	var got string
	err = s.withChannelsLock(ctx, ws.ID, func(ctx context.Context) error {
		st, err := s.readChannelsLock(ctx, ws)
		if err != nil {
			return err
		}
		if rule == ngac.GroupCreationAdmins {
			err = s.lockGroupCreation(ctx, ws, st)
		} else {
			err = s.unlockGroupCreation(ctx, st)
		}
		if err != nil {
			return err
		}
		after, err := s.readChannelsLock(ctx, ws)
		if err != nil {
			return err
		}
		got = after.rule()
		return nil
	})
	if err != nil {
		return "", err
	}
	s.announce(ctx, realtime.KindUpdated, ws.ID, ws.ID)
	return got, nil
}

// lockGroupCreation takes create_channel from every custom role's Channels grant,
// then from Members. Each write is a policy-service call, which invalidates the
// decision caches (the EPP path).
func (s *Service) lockGroupCreation(ctx context.Context, ws *WorkspaceResult, st *channelsLockState) error {
	children, err := s.policyWrite.GetChildren(ctx, &policypb.GetChildrenRequest{NodeId: ws.PcNodeID})
	if err != nil {
		return fmt.Errorf("get workspace roles: %w", err)
	}
	for _, n := range children.GetNodes() {
		if !isRole(n) {
			continue
		}
		changed, err := s.withdrawCreateChannel(ctx, n.Id, st.channelsOA)
		if err != nil {
			return err
		}
		if changed {
			s.announce(ctx, realtime.KindRoleChanged, ws.ID, n.Id)
		}
	}
	_, err = s.withdrawCreateChannel(ctx, st.membersUA, st.channelsOA)
	return err
}

// unlockGroupCreation puts create_channel back on the Members association.
func (s *Service) unlockGroupCreation(ctx context.Context, st *channelsLockState) error {
	if slices.Contains(st.membersOps, ngac.OpCreateChannel) {
		return nil
	}
	_, err := s.policyWrite.CreateAssociation(ctx, &policypb.CreateAssociationRequest{
		UaId: st.membersUA, OaId: st.channelsOA, Operations: ngac.UnlockedMemberChannelOps(st.membersOps),
	})
	if err != nil {
		return fmt.Errorf("restore members grant: %w", err)
	}
	return nil
}

// withdrawCreateChannel removes create_channel from what a UA holds on the
// Channels OA, keeping anything else it holds there, and reports whether it wrote.
func (s *Service) withdrawCreateChannel(ctx context.Context, uaID, channelsOA string) (bool, error) {
	assocs, err := s.policyWrite.GetAssociations(ctx, &policypb.GetAssociationsRequest{UaId: uaID})
	if err != nil {
		return false, fmt.Errorf("read grants: %w", err)
	}
	for _, a := range assocs.GetAssociations() {
		if a.OaId != channelsOA || !slices.Contains(a.Operations, ngac.OpCreateChannel) {
			continue
		}
		if left := ngac.LockedChannelOps(a.Operations); len(left) > 0 {
			_, err = s.policyWrite.CreateAssociation(ctx, &policypb.CreateAssociationRequest{UaId: uaID, OaId: channelsOA, Operations: left})
		} else {
			_, err = s.policyWrite.RemoveAssociation(ctx, &policypb.RemoveAssociationRequest{UaId: uaID, OaId: channelsOA})
		}
		if err != nil {
			return false, fmt.Errorf("withdraw create_channel: %w", err)
		}
		return true, nil
	}
	return false, nil
}

// withChannelsLock serialises the writes that decide who holds create_channel in
// one workspace, so a role edited while the lock is being turned on cannot slip
// the right back in between the check and the write.
func (s *Service) withChannelsLock(ctx context.Context, wsID string, fn func(ctx context.Context) error) error {
	return s.store.WithOwnerLock(ctx, wsID, fn)
}
