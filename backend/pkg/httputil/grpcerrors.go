package httputil

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// MapGRPCError translates a gRPC status into the equivalent HTTP error.
//
// Use this — not MapDomainError — on anything returned by a gRPC client.
// MapDomainError matches sentinel errors with errors.Is, and a gRPC status is
// not one of those, so passing it there silently falls through to 500. A denial
// reported as "internal server error" tells the caller the server broke and the
// request might succeed on retry, when the answer is a settled no.
//
// A refusal keeps its message. When the service attached an ErrorInfo reason
// (to tell apart refusals that share a status code) the body also carries it as
// "reason". Every other code — Internal, Unavailable, Unknown — is a failure of
// ours and becomes a generic 500 (see Internal); its text only reaches the log.
func MapGRPCError(err error) *echo.HTTPError {
	st, ok := status.FromError(err)
	if !ok {
		return Internal(err)
	}
	switch st.Code() {
	case codes.NotFound:
		return refusal(http.StatusNotFound, st)
	case codes.PermissionDenied:
		return refusal(http.StatusForbidden, st)
	case codes.Unauthenticated:
		return refusal(http.StatusUnauthorized, st)
	case codes.InvalidArgument:
		return refusal(http.StatusBadRequest, st)
	case codes.AlreadyExists, codes.FailedPrecondition:
		return refusal(http.StatusConflict, st)
	default:
		return Internal(err)
	}
}

func refusal(code int, st *status.Status) *echo.HTTPError {
	for _, d := range st.Details() {
		if info, ok := d.(*errdetails.ErrorInfo); ok && info.Reason != "" {
			return echo.NewHTTPError(code, map[string]any{"message": st.Message(), "reason": info.Reason})
		}
	}
	return echo.NewHTTPError(code, st.Message())
}
