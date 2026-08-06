package anchorsdk

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	nanoclient "github.com/nanostack-dev/anchor/clients/go"
)

// Sentinel errors for classifying a failed call with [errors.Is].
//
// ErrPermanent matches any error that retrying cannot fix — every 4xx, and by
// extension each of the more specific sentinels below. Retry loops should stop
// on it; the SDK's own retry already does.
var (
	ErrPermanent    = errors.New("anchor: permanent failure")
	ErrInvalid      = errors.New("anchor: invalid request")
	ErrUnauthorized = errors.New("anchor: unauthorized")
	ErrForbidden    = errors.New("anchor: forbidden")
	ErrNotFound     = errors.New("anchor: not found")
	ErrConflict     = errors.New("anchor: conflict")
)

// Detail is a single structured error returned by Anchor, mirroring the
// ApiError schema. Field is set when the error is attributable to one input.
type Detail struct {
	Code     string
	Message  string
	Field    string
	Metadata map[string]any
}

// Error is a failed Anchor call. It carries the HTTP status, the operation that
// failed, and any structured details Anchor returned in the body.
//
// Match it with [errors.As] to read Details, or classify it with [errors.Is]
// against the package sentinels:
//
//	if errors.Is(err, anchorsdk.ErrNotFound) { ... }
//
//	var apiErr *anchorsdk.Error
//	if errors.As(err, &apiErr) {
//	    for _, d := range apiErr.Details { log.Print(d.Code, d.Field, d.Message) }
//	}
type Error struct {
	// Op is the SDK operation that failed, e.g. "Organizations.Create".
	Op string
	// StatusCode is the HTTP status. Zero when the request never completed.
	StatusCode int
	// Details holds Anchor's structured errors, empty if the body carried none.
	Details []Detail
	// Body is the raw response body, retained for diagnostics when Details is empty.
	Body []byte

	cause error
	// forcePermanent marks a failure permanent regardless of status code, for
	// outcomes where a 2xx still means "retrying cannot help".
	forcePermanent bool
}

func (e *Error) Error() string {
	var b strings.Builder
	b.WriteString("anchor: ")
	b.WriteString(e.Op)

	if e.StatusCode != 0 {
		fmt.Fprintf(&b, ": status %d", e.StatusCode)
	}

	switch {
	case len(e.Details) > 0:
		parts := make([]string, 0, len(e.Details))
		for _, d := range e.Details {
			if d.Field != "" {
				parts = append(parts, fmt.Sprintf("%s (%s): %s", d.Code, d.Field, d.Message))
				continue
			}
			parts = append(parts, fmt.Sprintf("%s: %s", d.Code, d.Message))
		}
		b.WriteString(": ")
		b.WriteString(strings.Join(parts, "; "))
	case e.cause != nil:
		b.WriteString(": ")
		b.WriteString(e.cause.Error())
	}

	return b.String()
}

// Unwrap exposes the transport error for a request that never completed.
func (e *Error) Unwrap() error { return e.cause }

// Is supports [errors.Is] against the package sentinels. A 4xx matches
// ErrPermanent in addition to its specific sentinel.
func (e *Error) Is(target error) bool {
	switch target {
	case ErrPermanent:
		return e.Permanent()
	case ErrInvalid:
		return e.StatusCode == http.StatusBadRequest || e.StatusCode == http.StatusUnprocessableEntity
	case ErrUnauthorized:
		return e.StatusCode == http.StatusUnauthorized
	case ErrForbidden:
		return e.StatusCode == http.StatusForbidden
	case ErrNotFound:
		return e.StatusCode == http.StatusNotFound
	case ErrConflict:
		return e.StatusCode == http.StatusConflict
	default:
		return false
	}
}

// Permanent reports whether retrying is futile. Every 4xx is permanent; 5xx and
// transport failures are not.
//
// 429 is treated as retryable despite being a 4xx: it is the one client error
// that a later attempt can succeed at.
func (e *Error) Permanent() bool {
	if e.forcePermanent {
		return true
	}
	if e.StatusCode == http.StatusTooManyRequests {
		return false
	}
	return e.StatusCode >= 400 && e.StatusCode < 500
}

// permanentError builds an Error that retrying cannot fix, for a call that
// returned a success status but a terminal outcome in its body.
func permanentError(op string, code int, message string) *Error {
	return &Error{
		Op:             op,
		StatusCode:     code,
		Details:        []Detail{{Code: "TERMINAL_OUTCOME", Message: message}},
		forcePermanent: true,
	}
}

// transportError builds an Error for a request that never reached Anchor. It is
// always retryable.
func transportError(op string, err error) *Error {
	return &Error{Op: op, cause: err}
}

// statusError builds an Error from a completed non-2xx response, decoding
// Anchor's ApiErrorResponse body when present.
func statusError(op string, code int, body []byte) *Error {
	e := &Error{Op: op, StatusCode: code, Body: body}

	var payload nanoclient.ApiErrorResponse
	if len(body) == 0 || json.Unmarshal(body, &payload) != nil {
		return e
	}

	e.Details = make([]Detail, 0, len(payload.Errors))
	for _, apiErr := range payload.Errors {
		d := Detail{Code: apiErr.Code, Message: apiErr.Message}
		if apiErr.Field != nil {
			d.Field = *apiErr.Field
		}
		if apiErr.Metadata != nil {
			d.Metadata = *apiErr.Metadata
		}
		e.Details = append(e.Details, d)
	}

	return e
}

// decode returns payload when the response carried a decoded 2xx body, and a
// *Error otherwise. It is the single place every facade converts a generated
// response into a Go result.
func decode[T any](op string, code int, body []byte, payload *T) (*T, error) {
	if payload != nil && code >= 200 && code < 300 {
		return payload, nil
	}
	return nil, statusError(op, code, body)
}

// expectSuccess returns nil for any 2xx and a *Error otherwise. Used by
// operations with no response body, such as DELETE.
func expectSuccess(op string, code int, body []byte) error {
	if code >= 200 && code < 300 {
		return nil
	}
	return statusError(op, code, body)
}
