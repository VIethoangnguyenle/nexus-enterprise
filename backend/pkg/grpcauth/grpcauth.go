// Package grpcauth carries the verified caller across service-to-service gRPC
// calls, and proves to the receiving service that the caller was vouched for by
// a service of this platform.
//
// The REST edge verifies the JWT and puts the caller on the request context
// (httputil.SetClaims calls WithCaller). The client interceptors mint a signed,
// short-lived identity token for each call (HMAC-SHA256 under the shared
// INTERNAL_IDENTITY_SECRET) naming that caller, or the dialling service when
// there is none, and bind it to the full gRPC method. The server interceptors
// verify the signature, the expiry and the method before putting the caller
// on the handler's context, so a handler asks CallerFrom(ctx) instead of
// trusting a user id in the request body. Raw x-caller-* metadata is ignored.
//
// What this stops: anything that can reach a gRPC port but does not hold the
// secret can no longer claim an identity, and a token captured off the wire
// cannot be replayed on another method or after about a minute. What it does
// not stop: a holder of the secret can mint any identity, and a captured
// token can be replayed on the same method until it expires (there is no
// replay cache; the nonce is only for logging). The transport is plaintext, so
// the ports must still be unreachable from outside the platform's network.
package grpcauth

import (
	"context"
	"log/slog"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
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

// identityContext returns ctx with a freshly minted identity token for method
// in its outgoing metadata. Any token already there is dropped first, so only
// the caller carried by WithCaller (or the service name) can reach the wire.
//
// A call made with no caller (a registration flow, a startup step, an event
// consumer) is signed as service. With service "" it carries no token at all.
func identityContext(ctx context.Context, service, method string) (context.Context, error) {
	md, _ := metadata.FromOutgoingContext(ctx)
	md = md.Copy()
	md.Delete(KeyIdentity)

	var c claims
	switch caller := CallerFrom(ctx); {
	case caller.Authenticated():
		c = claims{UID: caller.UserID, NID: caller.NGACNodeID, TID: caller.TenantID}
	case caller != (Caller{}):
		// A half-built caller is a bug upstream. Sending it as the service
		// identity instead would quietly swap a user's rights for the
		// service's, so refuse the call.
		return nil, status.Error(codes.Unauthenticated, "incomplete caller on context")
	case service != "":
		c = claims{SVC: service}
	default:
		return metadata.NewOutgoingContext(ctx, md), nil
	}
	token, err := mint(active.Load(), c, method)
	if err != nil {
		return nil, status.Error(codes.Unauthenticated, "cannot sign identity: "+err.Error())
	}
	md.Set(KeyIdentity, token)
	return metadata.NewOutgoingContext(ctx, md), nil
}

// ClientInterceptor signs the caller on ctx into the outgoing metadata of each
// unary call. service is the name of the process dialling, used when ctx holds
// no caller; pass "" to send no identity then.
func ClientInterceptor(service string) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		ctx, err := identityContext(ctx, service, method)
		if err != nil {
			return err
		}
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

// StreamClientInterceptor is ClientInterceptor for streaming calls. The token
// is minted when the stream opens.
func StreamClientInterceptor(service string) grpc.StreamClientInterceptor {
	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		ctx, err := identityContext(ctx, service, method)
		if err != nil {
			return nil, err
		}
		return streamer(ctx, desc, cc, method, opts...)
	}
}

// ServiceRule admits one method to named services that carry no user.
type ServiceRule struct {
	// Reason says why the method may run without a user.
	Reason string
	// Services are the only service names that may call it.
	Services []string
}

// ServiceOnly builds a ServiceRule for the given services.
func ServiceOnly(reason string, services ...string) ServiceRule {
	return ServiceRule{Reason: reason, Services: services}
}

func (r ServiceRule) allows(service string) bool {
	for _, s := range r.Services {
		if s == service {
			return true
		}
	}
	return false
}

// ServerPolicy lists the full gRPC methods (for example
// "/policy.PolicyWrite/CreateNode") that may run without a user. Every other
// method requires a caller. Each entry carries the reason it is exempt.
type ServerPolicy struct {
	// Exempt methods need no identity at all (health checks).
	Exempt map[string]string
	// ServiceOK methods are also accepted from the named services, with no
	// user: startup steps, registration flows, and event consumers that have no
	// end user to forward. The service name comes from a verified token.
	ServiceOK map[string]ServiceRule
}

// ServerInterceptor admits a request when it carries a valid identity token
// for its method naming a complete caller, or a service its policy allows for
// that method, or when its method is exempt under p. Anything else is
// Unauthenticated. The caller is placed on the handler's context; body fields
// never decide it.
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

// maxHeaderListSize bounds the request headers a server accepts.
const maxHeaderListSize = 16 << 10

// ServerOptions returns the interceptor chains every service's gRPC server
// uses. The unary chain is Logging, Recovery, extra, then the caller check, so
// every server logs every call and survives a panic whether or not its main
// remembers to ask. Building both chains in one place keeps a server from
// getting one without the other.
func ServerOptions(p ServerPolicy, extra ...grpc.UnaryServerInterceptor) []grpc.ServerOption {
	chain := append([]grpc.UnaryServerInterceptor{Logging, Recovery}, extra...)
	chain = append(chain, ServerInterceptor(p))
	return []grpc.ServerOption{
		// The only caller-supplied metadata a request needs is the identity
		// token (about 400 bytes). Refuse oversized headers before they are
		// parsed and verified.
		grpc.MaxHeaderListSize(maxHeaderListSize),
		grpc.ChainUnaryInterceptor(chain...),
		grpc.ChainStreamInterceptor(StreamServerInterceptor(p)),
	}
}

// admit returns ctx carrying the caller (or the admitting service), or
// Unauthenticated when method may not run without one.
func admit(ctx context.Context, p ServerPolicy, method string) (context.Context, error) {
	if _, ok := p.Exempt[method]; ok {
		return ctx, nil
	}
	md, _ := metadata.FromIncomingContext(ctx)
	token := first(md, KeyIdentity)
	if token == "" {
		return nil, status.Error(codes.Unauthenticated, "caller identity required")
	}
	c, err := verify(active.Load(), token, method)
	if err != nil {
		// The reason stays in the log; the caller only learns it was refused.
		slog.Warn("identity token refused", "method", method, "reason", err.Error())
		return nil, status.Error(codes.Unauthenticated, "invalid caller identity")
	}
	if c.SVC != "" {
		if rule, ok := p.ServiceOK[method]; ok && rule.allows(c.SVC) {
			return context.WithValue(ctx, serviceKey{}, c.SVC), nil
		}
		slog.Warn("service identity not allowed on method", "method", method, "service", c.SVC)
		return nil, status.Error(codes.Unauthenticated, "caller identity required")
	}
	return WithCaller(ctx, Caller{UserID: c.UID, NGACNodeID: c.NID, TenantID: c.TID}), nil
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
