package grpc

import (
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"ngac-platform/services/messaging/internal/domain"
)

// domainError converts a domain error into a gRPC status that preserves what
// kind of failure it was.
//
// Every handler here used to wrap failures as codes.Internal regardless of
// cause. The REST layer then called httputil.MapDomainError on the result — but
// by that point the domain sentinel had been flattened into a status error, so
// errors.Is could not see it and everything fell through to 500.
//
// While these endpoints performed no authorization checks that was invisible:
// nothing ever denied, so nothing was ever misreported. Now that they do check,
// a denial reached the client as "500 Internal Server Error", which tells the
// caller the server broke and the request might be worth retrying, when in fact
// the answer is a settled no.
//
// context is the operation name, used only for the message.
func domainError(context string, err error) error {
	switch {
	case errors.Is(err, domain.ErrAccessDenied):
		return status.Errorf(codes.PermissionDenied, "%s: %v", context, err)
	case errors.Is(err, domain.ErrNotFound):
		return status.Errorf(codes.NotFound, "%s: %v", context, err)
	case errors.Is(err, domain.ErrInvalidInput):
		return status.Errorf(codes.InvalidArgument, "%s: %v", context, err)
	case errors.Is(err, domain.ErrAlreadyExists):
		return status.Errorf(codes.AlreadyExists, "%s: %v", context, err)
	default:
		return status.Errorf(codes.Internal, "%s: %v", context, err)
	}
}
