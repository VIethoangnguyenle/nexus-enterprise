package domain

import (
	"context"
	"errors"
	"fmt"
	"time"

	"ngac-platform/ngac"
	"ngac-platform/pkg/realtime"
	"ngac-platform/services/workspace/internal/store"
)

// Conversion is the record of a personal workspace becoming an organization.
type Conversion struct {
	ConvertedByName string
	ConvertedAt     time.Time
}

// ConvertToOrganization turns a personal workspace into an organization. It is
// one way and happens once.
//
// Who may: the caller must hold manage on the workspace's management OA, like
// any change to the workspace as a whole, and be an owner of it — a member of
// its Owners group, the same test every owner change uses. Holding manage is not
// enough: a manager who is not an owner is refused. A caller who may not is told so even when the workspace is
// already an organization; ErrAlreadyOrganization is only for the owner.
//
// What changes: only the type, with who and when. An organization needs no graph
// nodes a personal workspace lacks (assets get their OA on first use; approvals
// use the tenant schema, which the approval service creates on first use), so
// nothing is written to the graph.
func (s *Service) ConvertToOrganization(ctx context.Context, callerNodeID, wsID string) (*Conversion, error) {
	ws, err := s.authorizeAdmin(ctx, callerNodeID, wsID, ngac.OpManage)
	if err != nil {
		return nil, err
	}
	// Who the owners are, the type and the write happen under the workspace's owner
	// lock, like every other owner rule, so a conversion cannot land after the
	// caller was removed as an owner: it is one way and cannot be undone. The
	// owners are read from the graph as the writer sees it; if they cannot be
	// read nobody is an owner.
	var c *store.Conversion
	convert := func(ctx context.Context) error {
		_, owners, err := s.ownersOf(ctx, ws)
		if err != nil {
			return fmt.Errorf("%w: cannot resolve owners", ErrAccessDenied)
		}
		if err := requireOwner(owners, callerNodeID); err != nil {
			return fmt.Errorf("%w: only an owner converts a workspace", ErrAccessDenied)
		}
		// Read again under the lock: the row may have been converted since authorizeAdmin.
		row, err := s.store.GetByID(ctx, ws.ID)
		if err != nil {
			return fmt.Errorf("%w: workspace", ErrNotFound)
		}
		if row.Type == TypeOrganization {
			return ErrAlreadyOrganization
		}
		c, err = s.store.ConvertToOrganization(ctx, ws.ID, callerNodeID)
		switch {
		case errors.Is(err, store.ErrNotPersonal):
			return ErrAlreadyOrganization // converted by someone else since the read
		case errors.Is(err, store.ErrWorkspaceNotFound):
			return fmt.Errorf("%w: workspace", ErrNotFound)
		}
		return err
	}
	if err := s.store.WithOwnerLock(ctx, ws.ID, convert); err != nil {
		return nil, err
	}
	s.announce(ctx, realtime.KindUpdated, ws.ID, ws.ID)
	return &Conversion{ConvertedByName: readableName(c.ByName, neutralPerson), ConvertedAt: c.At}, nil
}
