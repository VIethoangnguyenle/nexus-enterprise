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
)

// ReasonQuotedMessageUnavailable is the machine-readable reason of a send refused
// because the message it quotes is gone, unknown, or in another channel. A client
// tells it apart from every other bad request without reading the prose.
const ReasonQuotedMessageUnavailable = "quoted_message_unavailable"

// ErrQuoteUnavailable is an invalid-input refusal of a quote that names no live
// message of the channel. Whether it is gone, never was, or lives elsewhere is
// not told: the answer must not reveal what another channel holds.
var ErrQuoteUnavailable = fmt.Errorf("%w: a quote names a message of this channel", ErrInvalidInput)
