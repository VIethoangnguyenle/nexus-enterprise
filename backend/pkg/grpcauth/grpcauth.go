// Package grpcauth carries the verified caller across service-to-service gRPC
// calls.
//
// The REST edge verifies the JWT and puts the caller on the request context
// (httputil.SetClaims calls WithCaller). ClientInterceptor copies that caller
// into outgoing gRPC metadata and ServerInterceptor reads it back into the
// context, so a handler asks CallerFrom(ctx) instead of trusting a user id in
// the request body.
//
// This is defense in depth for an internal-only network. The metadata is not
// signed: anything that can reach a gRPC port can claim any identity. Real
// service-to-service authentication (mTLS or an internal token) is out of
// scope here.
package grpcauth

import (
	"context"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// Metadata keys. gRPC lower-cases keys on the wire.
const (
	KeyUserID      = "x-caller-user-id"
	KeyNGACNodeID  = "x-caller-ngac-node-id"
	KeyTenantID    = "x-caller-tenant-id"
	KeyServiceName = "x-service-name"
)

// Caller is the authenticated end user on whose behalf a request runs.
type Caller struct {
	UserID     string
	NGACNodeID string
	TenantID   string
}

// Authenticated reports whether c names a user. Both ids are required: the
// user id attributes data the caller writes, the node id is what the policy
// service decides on. A caller with only one is treated as nobody.
func (c Caller) Authenticated() bool {
	return c.UserID != "" && c.NGACNodeID != ""
}

type callerKey struct{}
type serviceKey struct{}

// WithCaller returns a context carrying c.
func WithCaller(ctx context.Context, c Caller) context.Context {
	return context.WithValue(ctx, callerKey{}, c)
}

// CallerFrom returns the caller on ctx, or the zero Caller if there is none.
// Check Authenticated before trusting it.
func CallerFrom(ctx context.Context) Caller {
	c, _ := ctx.Value(callerKey{}).(Caller)
	return c
}

// ServiceFrom returns the calling service's name when a request was admitted
// on a service identity rather than a user, or "".
func ServiceFrom(ctx context.Context) string {
	s, _ := ctx.Value(serviceKey{}).(string)
	return s
}

// ClientInterceptor forwards the caller on ctx as outgoing metadata. Any
// caller metadata already on the outgoing context is dropped first, so only a
// caller carried by WithCaller can reach the wire.
//
// A call made with no caller (a registration flow, a startup step, an event
// consumer) is sent with service as its identity. service is the name of the
// process dialling; pass "" to send none.
func ClientInterceptor(service string) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		md, _ := metadata.FromOutgoingContext(ctx)
		md = md.Copy()
		for _, k := range []string{KeyUserID, KeyNGACNodeID, KeyTenantID, KeyServiceName} {
			md.Delete(k)
		}
		c := CallerFrom(ctx)
		if c != (Caller{}) && !c.Authenticated() {
			// A half-built caller is a bug upstream. Sending it as the service
			// identity instead would quietly swap a user's rights for the
			// service's, so refuse the call.
			return status.Error(codes.Unauthenticated, "incomplete caller on context")
		}
		if c.Authenticated() {
			md.Set(KeyUserID, c.UserID)
			md.Set(KeyNGACNodeID, c.NGACNodeID)
			if c.TenantID != "" {
				md.Set(KeyTenantID, c.TenantID)
			}
		} else if service != "" {
			md.Set(KeyServiceName, service)
		}
		return invoker(metadata.NewOutgoingContext(ctx, md), method, req, reply, cc, opts...)
	}
}

// ServerPolicy lists the full gRPC methods (for example
// "/policy.PolicyWrite/CreateNode") that may run without a user. Every other
// method requires a caller. Each entry carries the reason it is exempt.
type ServerPolicy struct {
	// Exempt methods need no identity at all (health checks).
	Exempt map[string]string
	// ServiceOK methods are also accepted from a named service with no user:
	// startup steps, registration flows, and event consumers that have no
	// end user to forward.
	ServiceOK map[string]string
}

// ServerInterceptor admits a request when it carries a complete caller, or
// when its method is exempt under p. Anything else is Unauthenticated. The
// caller is placed on the handler's context; body fields never decide it.
func ServerInterceptor(p ServerPolicy) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, h grpc.UnaryHandler) (any, error) {
		ctx, err := admit(ctx, p, info.FullMethod)
		if err != nil {
			return nil, err
		}
		return h(ctx, req)
	}
}

// StreamServerInterceptor applies the same rule to streaming methods, so a
// streaming RPC added later is closed by default rather than skipping the
// unary check.
func StreamServerInterceptor(p ServerPolicy) grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, h grpc.StreamHandler) error {
		ctx, err := admit(ss.Context(), p, info.FullMethod)
		if err != nil {
			return err
		}
		return h(srv, &callerStream{ServerStream: ss, ctx: ctx})
	}
}

// ServerOptions returns the interceptor chains every service's gRPC server
// uses. The unary chain is Logging, Recovery, extra, then the caller check, so
// every server logs every call and survives a panic whether or not its main
// remembers to ask. Building both chains in one place keeps a server from
// getting one without the other.
func ServerOptions(p ServerPolicy, extra ...grpc.UnaryServerInterceptor) []grpc.ServerOption {
	chain := append([]grpc.UnaryServerInterceptor{Logging, Recovery}, extra...)
	chain = append(chain, ServerInterceptor(p))
	return []grpc.ServerOption{
		grpc.ChainUnaryInterceptor(chain...),
		grpc.ChainStreamInterceptor(StreamServerInterceptor(p)),
	}
}

// admit returns ctx carrying the caller (or the admitting service), or
// Unauthenticated when method may not run without one.
func admit(ctx context.Context, p ServerPolicy, method string) (context.Context, error) {
	md, _ := metadata.FromIncomingContext(ctx)
	c := Caller{
		UserID:     first(md, KeyUserID),
		NGACNodeID: first(md, KeyNGACNodeID),
		TenantID:   first(md, KeyTenantID),
	}
	if c.Authenticated() {
		return WithCaller(ctx, c), nil
	}
	if _, ok := p.Exempt[method]; ok {
		return ctx, nil
	}
	if _, ok := p.ServiceOK[method]; ok {
		if svc := first(md, KeyServiceName); svc != "" {
			return context.WithValue(ctx, serviceKey{}, svc), nil
		}
	}
	return nil, status.Error(codes.Unauthenticated, "caller identity required")
}

// callerStream overrides the stream's context with the admitted one.
type callerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (s *callerStream) Context() context.Context { return s.ctx }

// HealthExempt returns the Exempt entry every server needs so liveness and
// readiness probes keep working.
func HealthExempt() map[string]string {
	return map[string]string{
		"/grpc.health.v1.Health/Check": "liveness and readiness probes carry no user",
		"/grpc.health.v1.Health/Watch": "health watchers carry no user",
	}
}

func first(md metadata.MD, key string) string {
	v := md.Get(key)
	if len(v) == 0 {
		return ""
	}
	return strings.TrimSpace(v[0])
}
