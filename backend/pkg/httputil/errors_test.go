package httputil_test

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"ngac-platform/pkg/httputil"
)

// MapDomainError is only useful if it actually recognises the errors services
// produce. It previously could not: every service declared its own
// `errors.New("access denied")`, and errors.Is compares identity rather than
// message text, so nothing ever matched and every domain failure — including a
// denial — was returned to the client as 500.
func TestMapDomainError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want int
	}{
		{"not found", httputil.ErrNotFound, http.StatusNotFound},
		{"access denied", httputil.ErrAccessDenied, http.StatusForbidden},
		{"already exists", httputil.ErrAlreadyExists, http.StatusConflict},
		{"invalid input", httputil.ErrInvalidInput, http.StatusBadRequest},

		// Services wrap with %w to add context; the class must survive that.
		{"wrapped denial", fmt.Errorf("%w: read on oa-1", httputil.ErrAccessDenied), http.StatusForbidden},
		{"doubly wrapped", fmt.Errorf("get thread: %w", fmt.Errorf("%w: x", httputil.ErrAccessDenied)), http.StatusForbidden},

		{"unknown", errors.New("something broke"), http.StatusInternalServerError},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := httputil.MapDomainError(tc.err).Code; got != tc.want {
				t.Errorf("status = %d, want %d", got, tc.want)
			}
		})
	}
}

// A sentinel declared locally with the same text is a different value. This
// pins the reason services must alias the shared ones instead of redeclaring:
// the moment one drifts back to errors.New, its denials become 500s.
func TestMapDomainError_LookalikeSentinelDoesNotMatch(t *testing.T) {
	lookalike := errors.New("access denied") // same text, different value

	if got := httputil.MapDomainError(lookalike).Code; got != http.StatusInternalServerError {
		t.Fatalf("status = %d — a lookalike matched, which would hide the aliasing requirement", got)
	}
	if errors.Is(lookalike, httputil.ErrAccessDenied) {
		t.Error("errors.Is matched two distinct errors.New values; the premise of this test is wrong")
	}
}
