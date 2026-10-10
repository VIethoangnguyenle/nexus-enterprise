package domain

import (
	"context"
	"fmt"

	"ngac-platform/ngac"
	policypb "ngac-platform/proto/policy"
)

// PermissionArea is a resource area a role can be granted rights on, with the
// operations that mean something there.
type PermissionArea struct {
	Area       ngac.Area
	Operations []string
}

// AreaGrant is what a role holds on one area.
type AreaGrant struct {
	Area       ngac.Area
	Operations []string
}

// areaOAs resolves, among the direct children of the workspace's PC, the OA of
// every area this workspace has. An area whose OA has not been created (a
// workspace that never used assets has no Assets OA) is simply absent.
func (s *Service) areaOAs(ctx context.Context, ws *WorkspaceResult) (map[ngac.Area]string, error) {
	children, err := s.policyWrite.GetChildren(ctx, &policypb.GetChildrenRequest{NodeId: ws.PcNodeID})
	if err != nil {
		return nil, fmt.Errorf("resolve workspace areas: %w", err)
	}
	byName := make(map[string]string, len(children.GetNodes()))
	for _, n := range children.GetNodes() {
		if n.NodeType == ngac.TypeOA {
			byName[n.Name] = n.Id
		}
	}
	out := make(map[ngac.Area]string, len(ngac.Areas()))
	for _, a := range ngac.Areas() {
		name, _ := ngac.AreaOAName(a, ngac.WorkspaceID(ws.ID))
		if id, ok := byName[name]; ok {
			out[a] = id
		}
	}
	return out, nil
}

// ListPermissionAreas lists the areas of a workspace a role can be granted on,
// each with the operations valid there. The answer is the ngac package's, so a
// screen never carries its own idea of which operation fits where. The caller
// must belong to the workspace.
func (s *Service) ListPermissionAreas(ctx context.Context, callerNodeID, wsID string) ([]PermissionArea, error) {
	ws, err := s.authorizeMember(ctx, callerNodeID, wsID)
	if err != nil {
		return nil, err
	}
	oas, err := s.areaOAs(ctx, ws)
	if err != nil {
		return nil, err
	}
	out := make([]PermissionArea, 0, len(oas))
	for _, a := range ngac.Areas() {
		if _, ok := oas[a]; ok {
			out = append(out, PermissionArea{Area: a, Operations: ngac.AreaOps(a)})
		}
	}
	return out, nil
}

// grantsByArea turns a UA's associations into per-area grants: only those that
// land on an area's OA, and only the operations that area offers. A grant on
// some other OA (a folder, a share) is not an area and is not shown as one.
func grantsByArea(assocs []*policypb.Association, oas map[ngac.Area]string) []AreaGrant {
	var out []AreaGrant
	for _, a := range ngac.Areas() {
		oa, ok := oas[a]
		if !ok {
			continue
		}
		held := map[string]bool{}
		for _, as := range assocs {
			if as.OaId != oa {
				continue
			}
			for _, op := range as.Operations {
				held[op] = true
			}
		}
		var ops []string
		for _, op := range ngac.AreaOps(a) {
			if held[op] {
				ops = append(ops, op)
			}
		}
		if len(ops) > 0 {
			out = append(out, AreaGrant{Area: a, Operations: ops})
		}
	}
	return out
}

// guardDelegation refuses to hand a person everything a UA confers unless the
// caller holds all of it themselves. Assigning a role or a department is a
// delegation, just like granting a permission: without this a caller who may
// administer the workspace could give themselves, or anyone, a role that holds
// operations they lack.
//
// What a UA confers includes what it inherits: a person under a sub-department
// also holds every grant of the departments above it. So the check covers the
// UA and every UA among its ancestors. A grant, or an ancestry, that cannot be
// read is not assumed to be empty.
func (s *Service) guardDelegation(ctx context.Context, callerNodeID, uaID string) error {
	anc, err := s.policyWrite.GetAncestors(ctx, &policypb.GetAncestorsRequest{NodeId: uaID})
	if err != nil {
		return fmt.Errorf("%w: cannot read what this attribute inherits", ErrAccessDenied)
	}
	chain := []string{uaID}
	for _, n := range anc.GetNodes() {
		if n.GetNodeType() == ngac.TypeUA && n.Id != uaID {
			chain = append(chain, n.Id)
		}
	}
	held := map[[2]string]bool{}
	for _, ua := range chain {
		resp, err := s.policyWrite.GetAssociations(ctx, &policypb.GetAssociationsRequest{UaId: ua})
		if err != nil {
			return fmt.Errorf("%w: cannot read what this attribute confers", ErrAccessDenied)
		}
		for _, a := range resp.GetAssociations() {
			for _, op := range a.Operations {
				if held[[2]string{a.OaId, op}] {
					continue
				}
				if err := s.checkAccess(ctx, callerNodeID, a.OaId, op); err != nil {
					return fmt.Errorf("cannot hand out an operation you do not hold: %w", err)
				}
				held[[2]string{a.OaId, op}] = true
			}
		}
	}
	return nil
}
