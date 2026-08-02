package httputil

import (
	"net/http"

	"github.com/labstack/echo/v4"
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
func MapGRPCError(err error) *echo.HTTPError {
	st, ok := status.FromError(err)
	if !ok {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	switch st.Code() {
	case codes.NotFound:
		return echo.NewHTTPError(http.StatusNotFound, st.Message())
	case codes.PermissionDenied:
		return echo.NewHTTPError(http.StatusForbidden, st.Message())
	case codes.Unauthenticated:
		return echo.NewHTTPError(http.StatusUnauthorized, st.Message())
	case codes.InvalidArgument:
		return echo.NewHTTPError(http.StatusBadRequest, st.Message())
	case codes.AlreadyExists:
		return echo.NewHTTPError(http.StatusConflict, st.Message())
	case codes.FailedPrecondition:
		return echo.NewHTTPError(http.StatusConflict, st.Message())
	default:
		return echo.NewHTTPError(http.StatusInternalServerError, st.Message())
	}
}
