// Package policyclient asks the policy service whether a user may perform an
// operation on an OA. It is the one place a service turns a PDP answer into a
// yes or a no, and it fails closed: a transport error or an unrecognised
// decision is never an allow. Who the user is, and whether the object exists,
// are the PDP's to judge: an empty ID is sent as it is and denied there.
//
// Access is checked on the OA, never on the object (see CLAUDE.md): pass the
// OA's node ID as the object.
package policyclient

import (
	"context"
	"log/slog"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"ngac-platform/ngac"
	"ngac-platform/pkg/grpcauth"
	policypb "ngac-platform/proto/policy"
)

// Reader is the part of policypb.PolicyReadServiceClient this package uses, so
// a test can stand in for the policy service.
type Reader interface {
	CheckAccess(ctx context.Context, in *policypb.CheckAccessRequest, opts ...grpc.CallOption) (*policypb.AccessDecision, error)
	BatchCheckAccess(ctx context.Context, in *policypb.BatchCheckAccessRequest, opts ...grpc.CallOption) (*policypb.BatchAccessResult, error)
}

// Client wraps a Reader.
type Client struct{ r Reader }

// New returns a Client over r.
func New(r Reader) *Client { return &Client{r: r} }

// Check reports whether userNodeID holds op on objectNodeID. allowed is true
// only on an explicit ALLOW with a nil err. A non-nil err means the answer is
// unknown; callers that must tell an outage from a refusal read it, the rest
// treat it as a denial.
func (c *Client) Check(ctx context.Context, userNodeID, objectNodeID, op string) (allowed bool, err error) {
	resp, err := c.r.CheckAccess(ctx, &policypb.CheckAccessRequest{
		UserNodeId: userNodeID, ObjectNodeId: objectNodeID, Operation: op,
	})
	if err != nil {
		// Callers that only need a yes or no drop the error and deny; the log is
		// where an outage of the policy service shows.
		slog.Warn("policy check failed; denying", "operation", op, "object", objectNodeID, "error", err)
		return false, err
	}
	return ngac.Allowed(resp.GetDecision(), nil), nil
}

// CheckCaller is Check for the verified caller on ctx (see package grpcauth).
func (c *Client) CheckCaller(ctx context.Context, objectNodeID, op string) (bool, error) {
	return c.Check(ctx, grpcauth.CallerFrom(ctx).NGACNodeID, objectNodeID, op)
}

// Permissions is the answer to a batch check: object ID -> operation -> held.
// The zero value, and anything not asked about, holds nothing.
type Permissions map[string]map[string]bool

// Has reports whether op is held on the object.
func (p Permissions) Has(objectID, op string) bool { return p[objectID][op] }

// BatchCheck asks, in one round-trip, which of ops userNodeID holds on each
// object. With no object or operation there is nothing to ask and the answer is
// empty. On error the Permissions is nil, which holds nothing: a caller that
// ignores err still fails closed.
func (c *Client) BatchCheck(ctx context.Context, userNodeID string, objectNodeIDs, ops []string) (Permissions, error) {
	if len(objectNodeIDs) == 0 || len(ops) == 0 {
		return Permissions{}, nil
	}
	resp, err := c.r.BatchCheckAccess(ctx, &policypb.BatchCheckAccessRequest{
		UserNodeId: userNodeID, ObjectIds: objectNodeIDs, Operations: ops,
	})
	if err != nil {
		return nil, err
	}
	out := make(Permissions, len(resp.GetResults()))
	for id, perms := range resp.GetResults() {
		out[id] = perms.GetPermissions()
	}
	return out, nil
}

// BatchCheckCaller is BatchCheck for the verified caller on ctx.
func (c *Client) BatchCheckCaller(ctx context.Context, objectNodeIDs, ops []string) (Permissions, error) {
	return c.BatchCheck(ctx, grpcauth.CallerFrom(ctx).NGACNodeID, objectNodeIDs, ops)
}

// IsNotFound reports whether err is the policy service saying a node or edge
// does not exist, as opposed to the call failing. It lets a domain tell "absent"
// from "unreachable" without knowing the transport.
func IsNotFound(err error) bool { return status.Code(err) == codes.NotFound }
