package grpcauth

import (
	"context"
	"fmt"
	"log/slog"
	"runtime/debug"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
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

// Dial opens a client connection to addr that forwards the caller on the
// request context (ClientInterceptor, StreamClientInterceptor) and names the
// dialling process as service. The transport is plaintext: the network between
// services is internal (see the package comment). Extra options are applied last.
func Dial(addr, service string, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
	base := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithChainUnaryInterceptor(ClientInterceptor(service)),
		grpc.WithChainStreamInterceptor(StreamClientInterceptor(service)),
	}
	return grpc.NewClient(addr, append(base, opts...)...)
}
