package provision

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"ngac-platform/ngac"
	policypb "ngac-platform/proto/policy"
)

// PersonalUAReader is the part of the policy read client the personal-UA lookup
// uses.
type PersonalUAReader interface {
	Finder
	GetAncestors(ctx context.Context, in *policypb.GetAncestorsRequest, opts ...grpc.CallOption) (*policypb.NodeList, error)
}

// personalUARetry is how long a lost creation race waits for the winner to
// finish assigning the user before looking again.
const personalUARetry = 50 * time.Millisecond

// FindPersonalUA returns the user attribute that contains exactly this user, or
// "" when none exists yet. A node is trusted as the person's UA only by what it
// is, never by its name: its properties must mark it as the personal UA of this
// user (ngac.IsPersonalUAOf). Role names are chosen by workspace administrators,
// so a role called "User_<id>" would otherwise receive every grant made to that
// person.
//
// A UA that was created but not yet assigned (a creation interrupted between
// its two writes) is adopted: the user is assigned to it, provided its
// properties say it is theirs.
func FindPersonalUA(ctx context.Context, r PersonalUAReader, w Writer, userNodeID string) (string, error) {
	anc, err := r.GetAncestors(ctx, &policypb.GetAncestorsRequest{NodeId: userNodeID})
	if err != nil {
		return "", fmt.Errorf("look up personal UA: %w", err)
	}
	for _, n := range anc.GetNodes() {
		if n.GetNodeType() == ngac.TypeUA && ngac.IsPersonalUAOf(n.GetProperties(), userNodeID) {
			return n.GetId(), nil
		}
	}

	byName, err := r.FindNodeByName(ctx, &policypb.FindNodeByNameRequest{
		Name: ngac.PersonalUAName(ngac.UserNodeID(userNodeID)), NodeType: ngac.TypeUA,
	})
	if err != nil && status.Code(err) != codes.NotFound {
		return "", fmt.Errorf("look up personal UA: %w", err)
	}
	if err == nil && byName.GetId() != "" && ngac.IsPersonalUAOf(byName.GetProperties(), userNodeID) {
		if err := assignToPersonalUA(ctx, w, userNodeID, byName.GetId()); err != nil {
			return "", err
		}
		return byName.GetId(), nil
	}
	return "", nil
}

// EnsurePersonalUA returns the user's personal UA, creating it on first use. An
// association can only start from a UA, never from a user node, so granting
// something to one person goes through this attribute. At most one such UA
// exists per user (unique index on the node name), which makes a lost creation
// race resolvable by looking again.
//
// The UA is the person's, not the caller's: it is never rolled back with
// whatever the caller goes on to provision.
func EnsurePersonalUA(ctx context.Context, r PersonalUAReader, w Writer, userNodeID string) (string, error) {
	if userNodeID == "" {
		return "", fmt.Errorf("personal UA: no user")
	}
	if id, err := FindPersonalUA(ctx, r, w, userNodeID); err != nil || id != "" {
		return id, err
	}

	c := NewCreator(w)
	ua, err := c.Node(ctx, &policypb.CreateNodeRequest{
		Name: ngac.PersonalUAName(ngac.UserNodeID(userNodeID)), NodeType: ngac.TypeUA,
		Properties: ngac.PersonalUAProperties(userNodeID),
	})
	if err != nil {
		// Most likely a concurrent first use created it between our lookup and
		// our create; give it a moment to finish assigning, then look again.
		for i := 0; i < 3; i++ {
			time.Sleep(personalUARetry)
			if id, ferr := FindPersonalUA(ctx, r, w, userNodeID); ferr == nil && id != "" {
				return id, nil
			}
		}
		return "", fmt.Errorf("create personal UA: %w", err)
	}
	if err := assignToPersonalUA(ctx, w, userNodeID, ua.GetId()); err != nil {
		return "", c.Fail(ctx, err)
	}
	c.Done()
	return ua.GetId(), nil
}

func assignToPersonalUA(ctx context.Context, w Writer, userNodeID, uaID string) error {
	if _, err := w.CreateAssignment(ctx, &policypb.CreateAssignmentRequest{ChildId: userNodeID, ParentId: uaID}); err != nil {
		return fmt.Errorf("assign user to personal UA: %w", err)
	}
	return nil
}
