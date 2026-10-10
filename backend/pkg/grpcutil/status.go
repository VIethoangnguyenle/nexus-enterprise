// Package grpcutil holds the mapping from a domain error to a gRPC status. (The
// server interceptors, Dial and Internal live in grpcauth, which installs them,
// so that one chain exists and a service cannot start a server without it; a
// service that needs only Internal does not link the HTTP packages this one
// reaches for the shared sentinels.)
package grpcutil

import (
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"ngac-platform/pkg/grpcauth"
	"ngac-platform/pkg/httputil"
)

// Mapping adds a service-specific sentinel to the ones every service shares.
type Mapping struct {
	Is   error
	Code codes.Code
}

// Status converts a domain error into the gRPC status a caller should see.
//
// A shared sentinel (httputil.Err*) and any extra Mapping keep their class and
// their message: those are the domain's own refusals, written for the caller.
// Anything else is a failure of ours — a database error, a failed downstream
// call — and its text can carry SQL, hostnames or other internals. The caller
// gets Internal "internal error" (see grpcauth.Internal); the original error
// stays reachable so the logging interceptor writes the detail to the log.
//
// The first matching extra wins, before the shared sentinels, so a service can
// refine one (for example mapping its own "conflict" to FailedPrecondition).
func Status(err error, extra ...Mapping) error {
	if err == nil {
		return nil
	}
	for _, m := range extra {
		if errors.Is(err, m.Is) {
			return status.Error(m.Code, err.Error())
		}
	}
	switch {
	case errors.Is(err, httputil.ErrNotFound):
		return status.Error(codes.NotFound, err.Error())
	case errors.Is(err, httputil.ErrAccessDenied):
		return status.Error(codes.PermissionDenied, err.Error())
	case errors.Is(err, httputil.ErrAlreadyExists):
		return status.Error(codes.AlreadyExists, err.Error())
	case errors.Is(err, httputil.ErrInvalidInput):
		return status.Error(codes.InvalidArgument, err.Error())
	}
	return grpcauth.Internal(err)
}

// IsFailedPrecondition reports whether err is a downstream service refusing
// because of the state of things (for example the document store saying a file
// was never uploaded), as opposed to the call failing. It lets a domain tell the
// two apart without knowing the transport.
func IsFailedPrecondition(err error) bool { return status.Code(err) == codes.FailedPrecondition }
