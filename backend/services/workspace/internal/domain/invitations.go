package domain

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"ngac-platform/ngac"
	"ngac-platform/pkg/realtime"
	policypb "ngac-platform/proto/policy"
	"ngac-platform/services/workspace/internal/store"
)

// ErrRateLimited means the caller has invited too many addresses for now.
var ErrRateLimited = errors.New("too many invitations, try again later")

// InvitationTTL is how long an invitation stays open.
const InvitationTTL = 7 * 24 * time.Hour

// Default invite budget per caller: enough for onboarding a team, too little to
// mail the internet from a stolen session.
const (
	defaultInviteMax    = 40
	defaultInviteWindow = time.Hour
)

// InvitationStore is where invitations and the address on an account live.
type InvitationStore interface {
	UpsertInvitation(ctx context.Context, inv *store.Invitation) error
	GetInvitation(ctx context.Context, id string) (*store.Invitation, error)
	ListPendingForWorkspace(ctx context.Context, wsID string, now time.Time) ([]*store.Invitation, error)
	ListPendingForEmail(ctx context.Context, email string, now time.Time) ([]*store.Invitation, error)
	RespondInvitation(ctx context.Context, id, email, status string, now time.Time) (*store.Invitation, error)
	ReopenInvitation(ctx context.Context, id string) error
	RevokeInvitation(ctx context.Context, wsID, id string, now time.Time) (bool, error)
	// UserEmail is the account's PROVED address, or empty: a signup address is only a claim.
	UserEmail(ctx context.Context, userID string) (string, error)
	// VerifiedEmailByNode is the same for the account behind a user node.
	VerifiedEmailByNode(ctx context.Context, nodeID string) (string, error)
	// RevokePendingForEmail withdraws every open offer to an address in a workspace.
	RevokePendingForEmail(ctx context.Context, wsID, email string, now time.Time) (int, error)
}

// WithInvitations wires in the invitation store.
func (s *Service) WithInvitations(inv InvitationStore) *Service {
	s.invitations = inv
	return s
}

// WithClock sets the time source (tests move it to expire an invitation).
func (s *Service) WithClock(now func() time.Time) *Service {
	s.now = now
	return s
}

// WithInviteLimit sets how many invitations one caller may send per window.
func (s *Service) WithInviteLimit(max int, window time.Duration) *Service {
	s.inviteLimiter = newWindowLimiter(max, window, s.clock)
	return s
}

func (s *Service) clock() time.Time {
	if s.now != nil {
		return s.now()
	}
	return time.Now()
}

// windowLimiter is a fixed-window counter per key, in this process. The auth
// service's OTP limit is the same shape on Redis (INCR with EXPIRE); this
// service has no Redis, and a per-replica budget is enough to stop a runaway
// loop or a stolen session from sweeping addresses.
type windowLimiter struct {
	mu     sync.Mutex
	max    int
	window time.Duration
	now    func() time.Time
	hits   map[string]*windowCount
}

type windowCount struct {
	start time.Time
	n     int
}

func newWindowLimiter(max int, window time.Duration, now func() time.Time) *windowLimiter {
	return &windowLimiter{max: max, window: window, now: now, hits: map[string]*windowCount{}}
}

// allow counts one use of key and reports whether it is within the budget.
func (l *windowLimiter) allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	c := l.hits[key]
	if c == nil || now.Sub(c.start) >= l.window {
		// Forget windows that have ended, so the map does not grow without bound.
		for k, v := range l.hits {
			if now.Sub(v.start) >= l.window {
				delete(l.hits, k)
			}
		}
		c = &windowCount{start: now}
		l.hits[key] = c
	}
	c.n++
	return c.n <= l.max
}

// InviteInput is one invitation as asked for: an address, and optionally a role
// and a department the person gets on accepting.
type InviteInput struct {
	Email        string
	RoleID       string
	DepartmentID string
}

// InviteByEmail records a pending invitation for the address. It never assigns
// anyone and never says whether an account, or a membership, exists for the
// address: any well-formed address is answered the same way. The person sees it
// when they sign in, and accepts or declines.
//
// The caller needs invite on the Mgmt OA. A role or a department riding along
// must belong to this workspace, and attaching one takes manage and holding
// everything it confers; both are checked again, against the inviter's rights at
// that time, when the invitation is accepted. Inviting is limited per caller.
func (s *Service) InviteByEmail(ctx context.Context, callerNodeID, wsID string, in InviteInput) error {
	ws, err := s.authorizeAdmin(ctx, callerNodeID, wsID, ngac.OpInvite)
	if err != nil {
		return err
	}
	if s.invitations == nil {
		return fmt.Errorf("invitations: store not configured")
	}
	addr, err := normalizeEmail(in.Email)
	if err != nil {
		return err
	}
	if in.RoleID != "" || in.DepartmentID != "" {
		if err := s.checkInviteExtras(ctx, ws, callerNodeID, in); err != nil {
			return err
		}
	}
	if s.inviteLimiter != nil && !s.inviteLimiter.allow(callerNodeID) {
		return ErrRateLimited
	}
	return s.invitations.UpsertInvitation(ctx, &store.Invitation{
		WorkspaceID: ws.ID, Email: addr, RoleID: in.RoleID, DepartmentID: in.DepartmentID,
		InvitedBy: callerNodeID, ExpiresAt: s.clock().Add(InvitationTTL),
	})
}

// checkInviteExtras validates the role and department of an invitation as the
// given inviter: they belong to this workspace, the inviter may assign (manage)
// and holds everything they confer.
func (s *Service) checkInviteExtras(ctx context.Context, ws *WorkspaceResult, inviterNodeID string, in InviteInput) error {
	if in.RoleID != "" {
		if err := s.requireRole(ctx, ws, in.RoleID); err != nil {
			return err
		}
	}
	var dept *store.Department
	if in.DepartmentID != "" {
		d, err := s.departmentInWorkspace(ctx, ws.ID, in.DepartmentID)
		if err != nil {
			return err
		}
		dept = d
	}
	mgmtID, err := s.mgmtOAID(ctx, ws)
	if err != nil {
		return err
	}
	if err := s.checkAccess(ctx, inviterNodeID, mgmtID, ngac.OpManage); err != nil {
		return err
	}
	if in.RoleID != "" {
		if err := s.guardDelegation(ctx, inviterNodeID, in.RoleID); err != nil {
			return err
		}
	}
	if dept != nil {
		if err := s.guardDelegation(ctx, inviterNodeID, dept.NGACUaID); err != nil {
			return err
		}
	}
	return nil
}

// InvitationView is an invitation as a screen shows it: names, never ids (the
// ID is only what a button sends back).
type InvitationView struct {
	ID             string
	Email          string
	WorkspaceName  string
	InviterName    string
	RoleName       string
	DepartmentName string
	CreatedAt      time.Time
	ExpiresAt      time.Time
}

// describe names an invitation's workspace, inviter, role and department. A
// name that cannot be found (a deleted role) is left empty.
func (s *Service) describe(ctx context.Context, inv *store.Invitation, workspaceName string) *InvitationView {
	v := &InvitationView{ID: inv.ID, Email: inv.Email, WorkspaceName: workspaceName, CreatedAt: inv.CreatedAt, ExpiresAt: inv.ExpiresAt}
	if inv.InvitedBy != "" {
		people := s.peopleFor(ctx, inv.WorkspaceID, []*policypb.NGACNode{{Id: inv.InvitedBy, NodeType: ngac.TypeU}})
		if len(people) == 1 {
			v.InviterName = people[0].DisplayName
		}
	}
	if inv.RoleID != "" {
		if n, err := s.policyWrite.GetNode(ctx, &policypb.GetNodeRequest{NodeId: inv.RoleID}); err == nil && isRole(n) {
			v.RoleName = ngac.DisplayName(n.Name, n.Properties)
		}
	}
	if inv.DepartmentID != "" {
		if d, err := s.deptStore.GetDepartment(ctx, inv.DepartmentID); err == nil && d != nil {
			v.DepartmentName = d.Name
		}
	}
	return v
}

// ListInvitations lists the offers still open in the workspace. The caller
// needs invite on the Mgmt OA.
func (s *Service) ListInvitations(ctx context.Context, callerNodeID, wsID string) ([]*InvitationView, error) {
	ws, err := s.authorizeAdmin(ctx, callerNodeID, wsID, ngac.OpInvite)
	if err != nil {
		return nil, err
	}
	if s.invitations == nil {
		return nil, fmt.Errorf("invitations: store not configured")
	}
	list, err := s.invitations.ListPendingForWorkspace(ctx, ws.ID, s.clock())
	if err != nil {
		return nil, err
	}
	out := make([]*InvitationView, 0, len(list))
	for _, inv := range list {
		out = append(out, s.describe(ctx, inv, ws.Name))
	}
	return out, nil
}

// RevokeInvitation withdraws an open offer of this workspace. The caller needs
// invite on the Mgmt OA; an invitation of another workspace is not found.
func (s *Service) RevokeInvitation(ctx context.Context, callerNodeID, wsID, invitationID string) error {
	ws, err := s.authorizeAdmin(ctx, callerNodeID, wsID, ngac.OpInvite)
	if err != nil {
		return err
	}
	if s.invitations == nil {
		return fmt.Errorf("invitations: store not configured")
	}
	ok, err := s.invitations.RevokeInvitation(ctx, ws.ID, invitationID, s.clock())
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("%w: invitation", ErrNotFound)
	}
	return nil
}

// myEmail is the address on the caller's own account: the only thing an
// invitation is matched to. It comes from the user record of the verified
// caller, never from a request.
func (s *Service) myEmail(ctx context.Context, userID string) (string, error) {
	if userID == "" {
		return "", fmt.Errorf("%w: requires an authenticated caller", ErrAccessDenied)
	}
	if s.invitations == nil {
		return "", fmt.Errorf("invitations: store not configured")
	}
	email, err := s.invitations.UserEmail(ctx, userID)
	if err != nil {
		return "", err
	}
	return email, nil
}

// ListMyInvitations lists the open offers addressed to the caller's account.
func (s *Service) ListMyInvitations(ctx context.Context, userID string) ([]*InvitationView, error) {
	email, err := s.myEmail(ctx, userID)
	if err != nil {
		return nil, err
	}
	if email == "" {
		return []*InvitationView{}, nil
	}
	list, err := s.invitations.ListPendingForEmail(ctx, email, s.clock())
	if err != nil {
		return nil, err
	}
	out := make([]*InvitationView, 0, len(list))
	for _, inv := range list {
		ws, err := s.GetWorkspace(ctx, inv.WorkspaceID)
		if err != nil {
			continue // the workspace is gone; so is the offer
		}
		out = append(out, s.describe(ctx, inv, ws.Name))
	}
	return out, nil
}

// ownInvitation loads an invitation and checks it is addressed to the caller's
// account. An invitation that is someone else's, or does not exist, is not
// found: the answer must not tell which.
func (s *Service) ownInvitation(ctx context.Context, userID, id string) (*store.Invitation, string, error) {
	email, err := s.myEmail(ctx, userID)
	if err != nil {
		return nil, "", err
	}
	inv, err := s.invitations.GetInvitation(ctx, id)
	if err != nil {
		return nil, "", err
	}
	if inv == nil || email == "" || inv.Email != email {
		return nil, "", fmt.Errorf("%w: invitation", ErrNotFound)
	}
	switch {
	case inv.Status != store.InvitationPending:
		return nil, "", fmt.Errorf("%w: invitation already answered", ErrAlreadyExists)
	case !inv.ExpiresAt.After(s.clock()):
		return nil, "", fmt.Errorf("%w: invitation expired", ErrInvalidInput)
	}
	return inv, email, nil
}

// DeclineInvitation turns down an open offer addressed to the caller.
func (s *Service) DeclineInvitation(ctx context.Context, userID, invitationID string) error {
	inv, email, err := s.ownInvitation(ctx, userID, invitationID)
	if err != nil {
		return err
	}
	got, err := s.invitations.RespondInvitation(ctx, inv.ID, email, store.InvitationDeclined, s.clock())
	if err != nil {
		return err
	}
	if got == nil {
		return fmt.Errorf("%w: invitation already answered", ErrAlreadyExists)
	}
	return nil
}

// AcceptResult says what accepting did.
type AcceptResult struct {
	WorkspaceID   string
	WorkspaceName string
	// RoleApplied and DepartmentApplied are false when the invitation named one
	// that was not given: the inviter could no longer give it, or it is gone.
	RoleApplied       bool
	DepartmentApplied bool
}

// AcceptInvitation makes the caller a member of the workspace that invited them.
//
// The invitation must be open, unexpired and addressed to the caller's account,
// and the inviter must still hold invite on the Mgmt OA: an offer from someone
// who has since lost the right does not add anyone. The member is added the way
// an invite always was (Members UA, listed in the workspace). A role or
// department riding along is given only if the inviter, now, still holds manage
// and everything it confers; otherwise the person joins without it. Answering is
// one conditional update, so two accepts at once add the person once.
func (s *Service) AcceptInvitation(ctx context.Context, userID, callerNodeID, invitationID string) (*AcceptResult, error) {
	if callerNodeID == "" {
		return nil, fmt.Errorf("%w: requires an authenticated caller", ErrAccessDenied)
	}
	inv, email, err := s.ownInvitation(ctx, userID, invitationID)
	if err != nil {
		return nil, err
	}
	ws, err := s.authorizeAdmin(ctx, inv.InvitedBy, inv.WorkspaceID, ngac.OpInvite)
	if err != nil {
		return nil, err
	}
	claimed, err := s.invitations.RespondInvitation(ctx, inv.ID, email, store.InvitationAccepted, s.clock())
	if err != nil {
		return nil, err
	}
	if claimed == nil {
		return nil, fmt.Errorf("%w: invitation already answered", ErrAlreadyExists)
	}
	result, err := s.admit(ctx, ws, claimed, userID, callerNodeID)
	if err != nil {
		if rerr := s.invitations.ReopenInvitation(ctx, claimed.ID); rerr != nil {
			slog.Error("could not reopen an invitation that failed to apply", "error", rerr)
		}
		return nil, err
	}
	s.announce(ctx, realtime.KindInvitationAccepted, ws.ID, callerNodeID)
	return result, nil
}

// admit adds the person to the workspace and gives them what the inviter can
// still give.
func (s *Service) admit(ctx context.Context, ws *WorkspaceResult, inv *store.Invitation, userID, nodeID string) (*AcceptResult, error) {
	res := &AcceptResult{WorkspaceID: ws.ID, WorkspaceName: ws.Name}
	already, err := s.reachesPC(ctx, ws, nodeID)
	if err != nil {
		return nil, err
	}
	if !already {
		membersUAID, err := s.FindUAByName(ctx, ws.ID, ngac.MembersUAName(ngac.WorkspaceID(ws.ID)))
		if err != nil {
			return nil, err
		}
		if _, err := s.policyWrite.CreateAssignment(ctx, &policypb.CreateAssignmentRequest{ChildId: nodeID, ParentId: membersUAID}); err != nil {
			return nil, fmt.Errorf("assign member: %w", err)
		}
		if s.directory != nil {
			if err := s.directory.EnsureTenantUser(ctx, ws.ID, userID, nodeID); err != nil {
				if _, undo := s.policyWrite.RemoveAssignment(ctx, &policypb.RemoveAssignmentRequest{ChildId: nodeID, ParentId: membersUAID}); undo != nil {
					slog.Error("could not undo an accepted invitation", "workspace", ws.ID, "error", undo)
				}
				return nil, err
			}
		}
	}

	// What rode along, as the inviter's rights stand now.
	extras := InviteInput{RoleID: inv.RoleID, DepartmentID: inv.DepartmentID}
	if extras.RoleID == "" && extras.DepartmentID == "" {
		return res, nil
	}
	if inv.RoleID != "" && s.checkInviteExtras(ctx, ws, inv.InvitedBy, InviteInput{RoleID: inv.RoleID}) == nil {
		if _, err := s.policyWrite.CreateAssignment(ctx, &policypb.CreateAssignmentRequest{ChildId: nodeID, ParentId: inv.RoleID}); err != nil {
			slog.Warn("invitation role not applied", "workspace", ws.ID, "error", err)
		} else {
			res.RoleApplied = true
		}
	}
	if inv.DepartmentID != "" && s.checkInviteExtras(ctx, ws, inv.InvitedBy, InviteInput{DepartmentID: inv.DepartmentID}) == nil {
		dept, err := s.departmentInWorkspace(ctx, ws.ID, inv.DepartmentID)
		if err == nil {
			if err := s.applyDepartment(ctx, ws, nodeID, dept); err != nil {
				slog.Warn("invitation department not applied", "workspace", ws.ID, "error", err)
			} else {
				res.DepartmentApplied = true
			}
		}
	}
	return res, nil
}
