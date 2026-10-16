package grpcauth

import (
	"context"
	"fmt"
	"log/slog"
	"runtime/debug"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

// InternalMessage is the whole of what a caller learns about a failure of the
// server's own. The cause goes to the log.
const InternalMessage = "internal error"

// Internal wraps err as an Internal status whose wire message is generic: a
// database error or a failed downstream call can name tables, hosts and
// queries. The returned error's own text is err's and err stays reachable with
// errors.Is, so Logging, which sees the error before it is serialised, writes
// the detail to the log. Do not wrap the result again: a wrapper's text would
// become the wire message.
func Internal(err error) error {
	return &internalError{cause: err}
}

type internalError struct{ cause error }

func (e *internalError) Error() string { return e.cause.Error() }
func (e *internalError) Unwrap() error { return e.cause }

// GRPCStatus is what the gRPC server serialises.
func (e *internalError) GRPCStatus() *status.Status {
	return status.New(codes.Internal, InternalMessage)
}

// Logging logs every unary call with its method, duration and status code. A
// failed call is logged at Warn with the error text: the log is where the
// detail of an internal failure belongs, because the caller only ever sees a
// generic message (see grpcutil.Status).
func Logging(ctx context.Context, req any, info *grpc.UnaryServerInfo, h grpc.UnaryHandler) (any, error) {
	start := time.Now()
	resp, err := h(ctx, req)
	attrs := []any{
		"method", info.FullMethod,
		"duration_ms", time.Since(start).Milliseconds(),
		"code", status.Code(err).String(),
	}
	if err != nil {
		slog.Warn("grpc call failed", append(attrs, "error", err.Error())...)
	} else {
		slog.Debug("grpc call", attrs...)
	}
	return resp, err
}

// Recovery turns a panic in a handler into an Internal error for that one
// call. Without it a single bad request takes the whole process down, and with
// it every in-flight request of every other tenant. The panic and its stack go
// to the log; the caller gets a generic message.
func Recovery(ctx context.Context, req any, info *grpc.UnaryServerInfo, h grpc.UnaryHandler) (resp any, err error) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("panic recovered in grpc handler",
				"method", info.FullMethod,
				"panic", fmt.Sprintf("%v", r),
				"stack", string(debug.Stack()))
			resp, err = nil, status.Error(codes.Internal, InternalMessage)
		}
	}()
	return h(ctx, req)
}

// dialIdleTimeout is how long a channel opened by Dial may sit without calls
// before it idles; zero means never. An idle channel rebuilds its resolver and
// connection on the next call, and in production that first call after half an
// hour without traffic waited about 8 s, with every request queued behind it.
var dialIdleTimeout time.Duration = 0

// slowConnect is how long a channel may take to become ready before the wait
// is logged as a warning.
var slowConnect = time.Second

// Dial opens a client connection to addr that forwards the caller on the
// request context (ClientInterceptor, StreamClientInterceptor) and names the
// dialling process as service. The transport is plaintext: the network between
// services is internal (see the package comment). The channel connects at once
// and never idles, so no call pays for setting it up; a slow (re)connect is
// logged. Extra options are applied last.
func Dial(addr, service string, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
	base := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithChainUnaryInterceptor(ClientInterceptor(service)),
		grpc.WithChainStreamInterceptor(StreamClientInterceptor(service)),
		grpc.WithIdleTimeout(dialIdleTimeout),
	}
	conn, err := grpc.NewClient(addr, append(base, opts...)...)
	if err != nil {
		return nil, err
	}
	go watchChannel(conn, addr, service, slowConnect)
	conn.Connect()
	return conn, nil
}

// watchChannel keeps the channel connected and logs its trouble until it is
// closed. A dropped connection leaves the channel idle until the next call,
// which would then pay for reconnecting, so an idle channel is reconnected at
// once. A wait longer than slow between starting to connect and being ready,
// and every failure to connect, are logged.
func watchChannel(conn *grpc.ClientConn, target, service string, slow time.Duration) {
	state := conn.GetState()
	var since time.Time
	for conn.WaitForStateChange(context.Background(), state) {
		next := conn.GetState()
		switch next {
		case connectivity.Idle:
			conn.Connect()
		case connectivity.Connecting:
			if since.IsZero() {
				since = time.Now()
			}
		case connectivity.TransientFailure:
			slog.Warn("grpc channel cannot connect", "target", target, "service", service)
		case connectivity.Ready:
			if waited := time.Since(since); !since.IsZero() && waited > slow {
				slog.Warn("grpc channel slow to become ready",
					"target", target, "service", service, "waited_ms", waited.Milliseconds())
			}
			since = time.Time{}
		case connectivity.Shutdown:
			return
		}
		state = next
	}
}
