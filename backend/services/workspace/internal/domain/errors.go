package domain

import (
	"fmt"

	"ngac-platform/pkg/httputil"
)

// These alias the shared sentinels rather than declaring new ones. errors.Is
// compares identity, not message text, so a local errors.New with the same
// string would be a different value and httputil.MapDomainError would never
// match it — every failure would surface as 500, including denials.
var (
	ErrNotFound      = httputil.ErrNotFound
	ErrAccessDenied  = httputil.ErrAccessDenied
	ErrAlreadyExists = httputil.ErrAlreadyExists
	ErrInvalidInput  = httputil.ErrInvalidInput
	ErrUnavailable   = httputil.ErrUnavailable
)

// ErrHiddenDepartmentHasPeople refuses to delete a department hidden from Danh bạ while people
// sit directly in it: they would land in a visible parent and be published. It is an invalid
// input like any other, but carries its own identity so the REST layer can name the reason.
var ErrHiddenDepartmentHasPeople = fmt.Errorf("%w: Chuyển những người trong phòng này sang phòng khác trước khi xoá", ErrInvalidInput)
