// Package caller carries the authenticated caller's identity on a context for
// the RPCs whose request message has no field for it.
//
// Most asset RPCs name the caller in the request (user_ngac_node_id). A few do
// not, and they cannot be authorized from the request alone. For those the
// server reads the caller from here instead. The identity is a Go context
// value, so it can only be set in-process (for example by a REST handler from
// verified JWT claims); a remote gRPC client has no way to supply it, and a
// guarded RPC called without it is denied.
//
// The proper fix is a caller field on those request messages. Until then this
// keeps the guard fail-closed without trusting anything off the wire.
package caller

import "context"

// Identity is the authenticated caller.
type Identity struct {
	UserID     string
	NGACNodeID string
}

type ctxKey struct{}

// WithIdentity returns a context carrying id.
func WithIdentity(ctx context.Context, id Identity) context.Context {
	return context.WithValue(ctx, ctxKey{}, id)
}

// FromContext returns the caller on ctx, or the zero Identity if none was set.
// A zero Identity names nobody and must be treated as unauthenticated.
func FromContext(ctx context.Context) Identity {
	id, _ := ctx.Value(ctxKey{}).(Identity)
	return id
}
