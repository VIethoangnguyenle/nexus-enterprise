// Package domain — group management: owner/manager roles, settings, transfers,
// disbanding, and the messaging-enforced restrictions (restricted posting,
// @Tất cả, system lines).
//
// Manager authority is "the caller is assigned to Ch_{id}_Managers", read from
// the graph (policy write client). It is never derived from a `manage` decision,
// because workspace owners hold manage on every group through the Channels OA
// and must not gain group powers from it.
package domain

import (
	"context"
	"fmt"
	"log/slog"
	"slices"

	"ngac-platform/ngac"
	"ngac-platform/pkg/policyclient"
	"ngac-platform/pkg/realtime"
	drivepb "ngac-platform/proto/drive"
	policypb "ngac-platform/proto/policy"
	"ngac-platform/services/messaging/internal/store"
)

// --- Manager check ---

// isGroupManager reports whether the user is assigned to the group's Managers UA.
// It reads from the policy writer (authoritative, not cached). Any failure to
// answer is a no: manager authority is never granted on an unknown.
func (s *Service) isGroupManager(ctx context.Context, ch *store.Channel, userNodeID string) bool {
	managersUA, err := s.findChannelManagersUA(ctx, ch)
	if err != nil {
		slog.Warn("managers UA lookup failed, treating as non-manager", "channel", ch.ID, "error", err)
		return false
	}
	if managersUA == "" {
		return false
	}
	res, err := s.policyWrite.IsAssigned(ctx, &policypb.IsAssignedRequest{
		ChildId: userNodeID, ParentId: managersUA,
	})
	if err != nil {
		slog.Warn("manager check failed, treating as non-manager", "channel", ch.ID, "error", err)
		return false
	}
	return res.GetValue()
}

// isGroupOwner reports whether userNodeID is the channel's owner (created_by).
// It resolves the owner's NGAC node from the user ID stored in created_by, so
// it works correctly even when the caller is a different person (e.g. a manager
// checking whether the target is the owner before demoting them).
func (s *Service) isGroupOwner(ctx context.Context, ch *store.Channel, userNodeID string) bool {
	if ch.CreatedBy == "" || userNodeID == "" {
		return false
	}
	ownerNode, err := s.store.UserNode(ctx, ch.CreatedBy)
	if err != nil {
		slog.Warn("owner lookup failed, treating as non-owner", "channel", ch.ID, "error", err)
		return false
	}
	return ownerNode != "" && ownerNode == userNodeID
}

// isGroupOwnerOrManager reports whether the user is the owner or a manager.
func (s *Service) isGroupOwnerOrManager(ctx context.Context, ch *store.Channel, userNodeID string) bool {
	return s.isGroupOwner(ctx, ch, userNodeID) || s.isGroupManager(ctx, ch, userNodeID)
}

// findChannelManagersUA looks the group's Managers UA up by its name on the
// policy writer, so a group made a moment ago is found. The name is keyed by the
// channel's ID, which no other channel shares. It answers "" when the group has
// none (a DM, the workspace channel, a group from before roles existed and not
// yet backfilled) and an error when the question could not be answered.
func (s *Service) findChannelManagersUA(ctx context.Context, ch *store.Channel) (string, error) {
	if ch.WorkspaceID == "" || ch.ChannelType != ChannelTypePrivate {
		return "", nil
	}
	n, err := s.policyWrite.FindNodeByName(ctx, &policypb.FindNodeByNameRequest{
		Name: ngac.ChannelManagersUAName(ngac.ChannelID(ch.ID)), NodeType: ngac.TypeUA,
	})
	if err != nil {
		if policyclient.IsNotFound(err) {
			return "", nil
		}
		return "", fmt.Errorf("find managers UA: %w", err)
	}
	return n.GetId(), nil
}

// requireManagersUA is findChannelManagersUA for an action that cannot proceed
// without it.
func (s *Service) requireManagersUA(ctx context.Context, ch *store.Channel) (string, error) {
	id, err := s.findChannelManagersUA(ctx, ch)
	if err != nil {
		return "", err
	}
	if id == "" {
		return "", fmt.Errorf("%w: group configuration unavailable", ErrNotFound)
	}
	return id, nil
}

// --- Group settings ---

// GroupSettings holds the management state of a group. People are named by
// user ID, the ID the roster (ListChannelMembers) gives each member, and the
// ID the appoint, remove-manager and transfer endpoints take back.
type GroupSettings struct {
	PostPolicy       string   `json:"post_policy"`
	InvitePolicy     string   `json:"invite_policy"`
	MentionAllPolicy string   `json:"mention_all_policy"`
	OwnerID          string   `json:"owner_id"`
	ManagerIDs       []string `json:"manager_ids"`
}

// GetGroupManagement returns a group's settings, its owner and its managers.
// Anyone who reads the group may see them: they decide what each member may do.
func (s *Service) GetGroupManagement(ctx context.Context, channelID, userNodeID string) (*GroupSettings, error) {
	ch, err := s.authorizeChannel(ctx, channelID, userNodeID, ngac.OpRead)
	if err != nil {
		return nil, err
	}
	if ch.ChannelType == ChannelTypeDM || ch.ChannelType == ChannelTypeWorkspace {
		return nil, fmt.Errorf("%w: direct messages and the workspace channel have no management", ErrInvalidInput)
	}
	managers, err := s.groupManagerIDs(ctx, ch)
	if err != nil {
		return nil, err
	}
	return &GroupSettings{
		PostPolicy:       ch.PostPolicy,
		InvitePolicy:     s.enforcedInviteRule(ctx, ch),
		MentionAllPolicy: ch.MentionAllPolicy,
		OwnerID:          ch.CreatedBy,
		ManagerIDs:       managers,
	}, nil
}

// enforcedInviteRule is the group's "Ai được thêm người" as the policy enforces
// it: read from the Members → Content grant (ngac.InviteRule), so the screen
// never promises what the graph refuses. The row's value stands in only when
// the grant cannot be read or is missing, and that is logged.
func (s *Service) enforcedInviteRule(ctx context.Context, ch *store.Channel) string {
	if ch.NGACUaID == "" || ch.NGACOaID == "" {
		return ch.InvitePolicy
	}
	list, err := s.policyWrite.GetAssociations(ctx, &policypb.GetAssociationsRequest{UaId: ch.NGACUaID})
	if err != nil {
		slog.Warn("members grant unreadable, showing the stored invite rule", "channel", ch.ID, "error", err)
		return ch.InvitePolicy
	}
	for _, a := range list.GetAssociations() {
		if a.GetOaId() == ch.NGACOaID {
			rule := ngac.InviteRule(a.GetOperations())
			if rule != ch.InvitePolicy {
				slog.Warn("stored invite rule differs from the graph; saving the rule repairs it",
					"channel", ch.ID, "stored", ch.InvitePolicy, "graph", rule)
			}
			return rule
		}
	}
	slog.Warn("group has no members grant on its content", "channel", ch.ID)
	return ch.InvitePolicy
}

// groupManagerIDs lists the user IDs of the people directly under the group's
// Managers UA, read from the writer. A node with no user behind it is skipped.
func (s *Service) groupManagerIDs(ctx context.Context, ch *store.Channel) ([]string, error) {
	managersUA, err := s.findChannelManagersUA(ctx, ch)
	if err != nil {
		return nil, err
	}
	out := []string{}
	if managersUA == "" {
		return out, nil
	}
	children, err := s.policyWrite.GetChildren(ctx, &policypb.GetChildrenRequest{NodeId: managersUA})
	if err != nil {
		return nil, fmt.Errorf("list managers: %w", err)
	}
	for _, n := range children.GetNodes() {
		if n.GetNodeType() != ngac.TypeU {
			continue
		}
		id, err := s.store.UserIDByNode(ctx, n.GetId())
		if err != nil {
			return nil, fmt.Errorf("list managers: %w", err)
		}
		if id != "" {
			out = append(out, id)
		}
	}
	return out, nil
}

// UpdateGroupSettings changes a group's management policies.
// Requires manager authority.
func (s *Service) UpdateGroupSettings(ctx context.Context, channelID, userNodeID string, postPolicy, invitePolicy, mentionAllPolicy string) error {
	ch, err := s.authorizeChannel(ctx, channelID, userNodeID, ngac.OpRead)
	if err != nil {
		return err
	}
	if ch.ChannelType == ChannelTypeDM || ch.ChannelType == ChannelTypeWorkspace {
		return fmt.Errorf("%w: direct messages and the workspace channel have no management", ErrInvalidInput)
	}
	if !s.isGroupOwnerOrManager(ctx, ch, userNodeID) {
		return fmt.Errorf("%w: only the owner or a manager may change settings", ErrAccessDenied)
	}
	// Validate setting values.
	for _, v := range []string{postPolicy, invitePolicy, mentionAllPolicy} {
		if v != "" && !ngac.IsGroupSetting(v) {
			return fmt.Errorf("%w: invalid setting value %q", ErrInvalidInput, v)
		}
	}

	// The grant is written whenever the rule is saved, even when the row already
	// holds that value: the row and the graph are separate writes, so they can
	// drift (a row write that failed after the grant, two managers saving at
	// once), and comparing with the row would leave a drifted pair stuck.
	// CreateAssociation is an upsert, so a grant already right costs nothing.
	if invitePolicy != "" {
		if err := s.syncInviteGrant(ctx, ch, invitePolicy); err != nil {
			return err
		}
	}

	if err := s.store.UpdateChannelSettings(ctx, channelID, postPolicy, invitePolicy, mentionAllPolicy); err != nil {
		return err
	}

	s.emitGroupManagementChanged(ctx, ch)
	return nil
}

// syncInviteGrant adjusts the Members → Content association: when restricted,
// removes invite; when unrestricted, adds it back. Goes through the policy
// writer (EPP invalidation).
func (s *Service) syncInviteGrant(ctx context.Context, ch *store.Channel, invitePolicy string) error {
	var ops []string
	if invitePolicy == ngac.GroupSettingManagers {
		ops = ngac.ChannelMemberOpsRestricted()
	} else {
		ops = ngac.ChannelMemberOps()
	}
	if _, err := s.policyWrite.CreateAssociation(ctx, &policypb.CreateAssociationRequest{
		UaId: ch.NGACUaID, OaId: ch.NGACOaID, Operations: ops,
	}); err != nil {
		return fmt.Errorf("sync invite grant: %w", err)
	}
	return nil
}

// --- Manager appointments ---

// AppointManager assigns a member to the group's Managers UA.
func (s *Service) AppointManager(ctx context.Context, channelID, userNodeID, targetNodeID string) error {
	ch, err := s.authorizeChannel(ctx, channelID, userNodeID, ngac.OpRead)
	if err != nil {
		return err
	}
	if ch.ChannelType == ChannelTypeDM || ch.ChannelType == ChannelTypeWorkspace {
		return fmt.Errorf("%w: DMs and the workspace channel have no managers", ErrInvalidInput)
	}
	if !s.isGroupOwnerOrManager(ctx, ch, userNodeID) {
		return fmt.Errorf("%w: only the owner or a manager may appoint managers", ErrAccessDenied)
	}
	// Target must be a member.
	if err := s.requireInChannel(ctx, ch, targetNodeID); err != nil {
		return err
	}
	managersUA, err := s.requireManagersUA(ctx, ch)
	if err != nil {
		return err
	}
	if _, err := s.policyWrite.CreateAssignment(ctx, &policypb.CreateAssignmentRequest{
		ChildId: targetNodeID, ParentId: managersUA,
	}); err != nil {
		return fmt.Errorf("appoint manager: %w", err)
	}
	s.emitGroupManagementChanged(ctx, ch)
	return nil
}

// RemoveManager removes a member from the group's Managers UA but keeps them in Members.
func (s *Service) RemoveManager(ctx context.Context, channelID, userNodeID, targetNodeID string) error {
	ch, err := s.authorizeChannel(ctx, channelID, userNodeID, ngac.OpRead)
	if err != nil {
		return err
	}
	if ch.ChannelType == ChannelTypeDM || ch.ChannelType == ChannelTypeWorkspace {
		return fmt.Errorf("%w: DMs and the workspace channel have no managers", ErrInvalidInput)
	}
	if !s.isGroupOwnerOrManager(ctx, ch, userNodeID) {
		return fmt.Errorf("%w: only the owner or a manager may remove managers", ErrAccessDenied)
	}
	// Cannot demote the owner, whoever asks: the owner is resolved from
	// created_by, not from the caller.
	if s.isGroupOwner(ctx, ch, targetNodeID) {
		return fmt.Errorf("%w: the owner cannot be demoted; transfer ownership first", ErrInvalidInput)
	}
	managersUA, err := s.requireManagersUA(ctx, ch)
	if err != nil {
		return err
	}
	if _, err := s.policyWrite.RemoveAssignment(ctx, &policypb.RemoveAssignmentRequest{
		ChildId: targetNodeID, ParentId: managersUA,
	}); err != nil {
		return fmt.Errorf("remove manager: %w", err)
	}
	s.emitGroupManagementChanged(ctx, ch)
	return nil
}

// --- Transfer and disband ---

// TransferOwnership moves ownership from the current owner to another member of
// the group, who becomes a manager; the former owner stays a manager. The
// Managers edges are written before created_by moves, so a failure leaves the
// owner where they were and a retry finishes the job.
func (s *Service) TransferOwnership(ctx context.Context, channelID, userNodeID, targetNodeID string) error {
	ch, err := s.authorizeChannel(ctx, channelID, userNodeID, ngac.OpRead)
	if err != nil {
		return err
	}
	if ch.ChannelType == ChannelTypeDM || ch.ChannelType == ChannelTypeWorkspace {
		return fmt.Errorf("%w: DMs and the workspace channel have no ownership", ErrInvalidInput)
	}
	if !s.isGroupOwner(ctx, ch, userNodeID) {
		return fmt.Errorf("%w: only the owner may transfer ownership", ErrAccessDenied)
	}
	if targetNodeID == userNodeID {
		return fmt.Errorf("%w: you already own this group", ErrInvalidInput)
	}
	if err := s.requireInChannel(ctx, ch, targetNodeID); err != nil {
		return err
	}
	targetUserID, err := s.store.UserIDByNode(ctx, targetNodeID)
	if err != nil {
		return fmt.Errorf("transfer ownership: resolve target: %w", err)
	}
	if targetUserID == "" {
		return fmt.Errorf("%w: not a member of this channel", ErrNotFound)
	}
	managersUA, err := s.requireManagersUA(ctx, ch)
	if err != nil {
		return err
	}
	// Both are managers afterwards; CreateAssignment is idempotent.
	for _, node := range []string{targetNodeID, userNodeID} {
		if _, err := s.policyWrite.CreateAssignment(ctx, &policypb.CreateAssignmentRequest{
			ChildId: node, ParentId: managersUA,
		}); err != nil {
			return fmt.Errorf("transfer ownership: add manager: %w", err)
		}
	}
	if err := s.store.TransferChannelOwnership(ctx, channelID, targetUserID); err != nil {
		return err
	}
	s.emitGroupManagementChanged(ctx, ch)
	return nil
}

// DisbandGroup deletes a group. Owner only. The group's drive goes to the
// trash first (see trashChannelDrive); if it cannot, nothing is deleted.
func (s *Service) DisbandGroup(ctx context.Context, channelID, userNodeID string) error {
	ch, err := s.authorizeChannel(ctx, channelID, userNodeID, ngac.OpRead)
	if err != nil {
		return err
	}
	if ch.ChannelType != ChannelTypePrivate {
		return fmt.Errorf("%w: only private groups can be disbanded", ErrInvalidInput)
	}
	if !s.isGroupOwner(ctx, ch, userNodeID) {
		return fmt.Errorf("%w: only the owner may disband a group", ErrAccessDenied)
	}
	// Resolved before the row goes: afterwards nothing names it.
	managersUA, err := s.findChannelManagersUA(ctx, ch)
	if err != nil {
		return err
	}
	if err := s.trashChannelDrive(ctx, ch); err != nil {
		return err
	}
	// Remove the channel row (cascades to members cache).
	if err := s.store.DeleteChannel(ctx, channelID); err != nil {
		return err
	}
	// Clean up graph nodes.
	s.cleanupChannelGraph(ctx, ch, managersUA)

	disbanded := channelEvent(ctx, realtime.KindDeleted, ch)
	disbanded.ChannelID = ch.ID
	s.emit(disbanded)
	return nil
}

// trashChannelDrive moves a group's drive (its root folder and everything in
// it) to the trash through the drive service, as the caller on ctx, whose write
// on the drive the drive service checks. Nothing is deleted for good: the
// items keep their rows, objects and quota share, and once the group's nodes
// are gone only an operator, in the database, can restore or purge them.
//
// It runs before the group is deleted. Afterwards the drive's OA hangs under
// no group and its files would stay counted against the workspace, reachable
// by nobody. A drive that cannot be looked up or trashed stops the disband
// (ErrUnavailable, 503): the group stays whole and a retry finishes. A drive
// already in the trash (an earlier attempt) is not trashed again. With no drive
// service wired there is no drive to trash.
func (s *Service) trashChannelDrive(ctx context.Context, ch *store.Channel) error {
	if s.driveClient == nil {
		return nil
	}
	root, err := s.driveClient.GetChannelDrive(ctx, &drivepb.GetChannelDriveRequest{ChannelId: ch.ID})
	if err != nil {
		slog.Error("disband refused: the group's drive could not be looked up", "channel", ch.ID, "error", err)
		return fmt.Errorf("%w: the group's drive could not be moved to the trash; try again", ErrUnavailable)
	}
	if root.GetId() == "" || root.GetStatus() == "trashed" {
		return nil
	}
	if _, err := s.driveClient.TrashItem(ctx, &drivepb.TrashItemRequest{ItemId: root.GetId()}); err != nil {
		slog.Error("disband refused: the group's drive could not be trashed", "channel", ch.ID, "drive", root.GetId(), "error", err)
		return fmt.Errorf("%w: the group's drive could not be moved to the trash; try again", ErrUnavailable)
	}
	return nil
}

// cleanupChannelGraph removes a disbanded group's NGAC nodes through the policy
// writer (so decisions are invalidated): the Managers UA, the Members UA and the
// Content OA. Deleting a node deletes its edges. The row is already gone, so a
// node that cannot be deleted is logged for an operator; it grants nothing to
// anyone once its channel no longer exists.
func (s *Service) cleanupChannelGraph(ctx context.Context, ch *store.Channel, managersUA string) {
	for _, id := range []string{managersUA, ch.NGACUaID, ch.NGACOaID} {
		if id == "" {
			continue
		}
		if _, err := s.policyWrite.DeleteNode(ctx, &policypb.DeleteNodeRequest{NodeId: id}); err != nil {
			slog.Warn("could not clean up channel graph node", "channel", ch.ID, "node", id, "error", err)
		}
	}
}

// emitGroupManagementChanged announces a settings or role change so clients
// refresh permissions.
func (s *Service) emitGroupManagementChanged(ctx context.Context, ch *store.Channel) {
	evt := channelEvent(ctx, "management_changed", ch)
	evt.ChannelID = ch.ID
	s.emit(evt)
}

// --- Restricted posting check ---

// The system's own message types. A person sending one of these is always refused
// (they cannot spoof a system line).
var systemMessageTypes = map[string]bool{
	"system":      true,
	"system_join": true,
	"system_left": true,
	"call_event":  true,
}

// MentionAll is the canonical mention-all string.
const MentionAll = "@Tất cả"

// containsMentionAll reports whether the mentions list contains the @all marker.
func containsMentionAll(mentions []string) bool {
	return slices.Contains(mentions, MentionAll)
}

// checkGroupPostingRestrictions enforces:
// - system message types from a person → always DENY
// - restricted posting: top-level messages (no parent) from a non-manager → DENY
// - restricted @Tất cả: from a non-manager → DENY
// - DMs refuse @Tất cả
// - workspace-wide channel: @Tất cả only for workspace admins
//
// This must be called after the basic write authorization.
func (s *Service) checkGroupPostingRestrictions(ctx context.Context, ch *store.Channel, senderNodeID string, in SendMessageInput) error {
	msgType := in.MessageType
	if msgType == "" {
		msgType = "user"
	}

	// A person sending a system-type message is always refused.
	if systemMessageTypes[msgType] && msgType != "user" {
		return fmt.Errorf("%w: system message types cannot be sent by a person", ErrAccessDenied)
	}

	// DMs refuse @Tất cả.
	if ch.ChannelType == ChannelTypeDM && containsMentionAll(in.Mentions) {
		return fmt.Errorf("%w: @Tất cả is not available in direct messages", ErrAccessDenied)
	}

	// Workspace-wide channel: @Tất cả only for workspace admins.
	if ch.ChannelType == ChannelTypeWorkspace && containsMentionAll(in.Mentions) {
		if !s.isWorkspaceAdmin(ctx, ch, senderNodeID) {
			return fmt.Errorf("%w: only administrators may use @Tất cả in the workspace channel", ErrAccessDenied)
		}
	}

	// Private groups only.
	if ch.ChannelType != ChannelTypePrivate {
		return nil
	}

	// Restricted posting: a top-level message from a non-manager is refused. A
	// card (approval, poll, task) is a post like any other: refused in the main
	// stream, a reply inside a thread.
	restrictedPost := ch.PostPolicy == ngac.GroupSettingManagers && in.ParentMessageID == ""
	restrictedMention := ch.MentionAllPolicy == ngac.GroupSettingManagers && containsMentionAll(in.Mentions)
	if !restrictedPost && !restrictedMention {
		return nil
	}
	if s.isGroupOwnerOrManager(ctx, ch, senderNodeID) {
		return nil
	}
	if restrictedPost {
		return fmt.Errorf("%w: only the owner and managers may post top-level messages", ErrAccessDenied)
	}
	return fmt.Errorf("%w: only the owner and managers may mention everyone", ErrAccessDenied)
}

// checkTopLevelPost refuses a non-manager starting a poll or a task in the
// group's main stream while posting is restricted; inside a thread it is a reply.
func (s *Service) checkTopLevelPost(ctx context.Context, ch *store.Channel, userNodeID, parentMessageID string) error {
	if ch.ChannelType != ChannelTypePrivate || ch.PostPolicy != ngac.GroupSettingManagers || parentMessageID != "" {
		return nil
	}
	if s.isGroupOwnerOrManager(ctx, ch, userNodeID) {
		return nil
	}
	return fmt.Errorf("%w: only the owner and managers may post top-level messages", ErrAccessDenied)
}

// checkEditMentionAllRestriction enforces the @Tất cả restriction on edits:
// an edit that adds @Tất cả is refused when restricted.
func (s *Service) checkEditMentionAllRestriction(ctx context.Context, ch *store.Channel, senderNodeID string, oldMentions, newMentions []string) error {
	if ch.ChannelType == ChannelTypeDM && containsMentionAll(newMentions) {
		return fmt.Errorf("%w: @Tất cả is not available in direct messages", ErrAccessDenied)
	}
	if ch.ChannelType == ChannelTypeWorkspace && containsMentionAll(newMentions) {
		if !s.isWorkspaceAdmin(ctx, ch, senderNodeID) {
			return fmt.Errorf("%w: only administrators may use @Tất cả in the workspace channel", ErrAccessDenied)
		}
	}
	if ch.ChannelType != ChannelTypePrivate {
		return nil
	}
	// If the edit adds @Tất cả and it's restricted, check manager status.
	if ch.MentionAllPolicy == ngac.GroupSettingManagers &&
		containsMentionAll(newMentions) && !containsMentionAll(oldMentions) {
		if !s.isGroupOwnerOrManager(ctx, ch, senderNodeID) {
			return fmt.Errorf("%w: only the owner and managers may mention everyone", ErrAccessDenied)
		}
	}
	return nil
}

// isWorkspaceAdmin checks if the user holds manage on the workspace's Mgmt OA.
func (s *Service) isWorkspaceAdmin(ctx context.Context, ch *store.Channel, userNodeID string) bool {
	if ch.WorkspaceID == "" {
		return false
	}
	ws, err := s.store.GetWorkspaceByID(ctx, ch.WorkspaceID)
	if err != nil || ws == nil {
		return false
	}
	mgmtOA := s.findChildByName(ctx, ws.PCNodeID, ngac.MgmtOAName(ngac.WorkspaceID(ws.ID)), ngac.TypeOA)
	if mgmtOA == "" {
		return false
	}
	ok, _ := policyclient.New(s.policyRead).Check(ctx, userNodeID, mgmtOA, ngac.OpManage)
	return ok
}
