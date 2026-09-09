// Package errs implements the gateway's public error envelope
// (docs/TZ.md §8.1): a closed, documented set of error codes, each mapped
// to one HTTP status, always returned as
//
//	{"error": {"code", "message", "details", "trace_id"}}
package errs

import "net/http"

// Code is a closed set — do not add ad-hoc values at call sites. Extend
// this list first, matching docs/TZ.md §8.1.
type Code string

const (
	CodeValidationFailed     Code = "VALIDATION_FAILED"
	CodeUnauthenticated      Code = "UNAUTHENTICATED"
	CodeForbidden            Code = "FORBIDDEN"
	CodeNotFound             Code = "NOT_FOUND"
	CodeItemsUnavailable     Code = "ITEMS_UNAVAILABLE"
	CodePriceChanged         Code = "PRICE_CHANGED"
	CodeInvalidTransition    Code = "INVALID_TRANSITION"
	CodeIdempotencyKeyReused Code = "IDEMPOTENCY_KEY_REUSED"
	CodeRequestInProgress    Code = "REQUEST_IN_PROGRESS"
	CodeRateLimited          Code = "RATE_LIMITED"
	CodeTableInactive        Code = "TABLE_INACTIVE"
	CodeSessionClosed        Code = "SESSION_CLOSED"
	CodeCatalogUnavailable   Code = "CATALOG_UNAVAILABLE"
	CodeInternal             Code = "INTERNAL"
)

var httpStatus = map[Code]int{
	CodeValidationFailed:     http.StatusBadRequest,
	CodeUnauthenticated:      http.StatusUnauthorized,
	CodeForbidden:            http.StatusForbidden,
	CodeNotFound:             http.StatusNotFound,
	CodeItemsUnavailable:     http.StatusConflict,
	CodePriceChanged:         http.StatusConflict,
	CodeInvalidTransition:    http.StatusConflict,
	CodeIdempotencyKeyReused: http.StatusConflict,
	CodeRequestInProgress:    http.StatusConflict,
	CodeRateLimited:          http.StatusTooManyRequests,
	CodeTableInactive:        http.StatusConflict,
	CodeSessionClosed:        http.StatusConflict,
	CodeCatalogUnavailable:   http.StatusServiceUnavailable,
	CodeInternal:             http.StatusInternalServerError,
}

// Detail is one entry of the optional "details" array. docs/TZ.md §8.1
// shows the ITEMS_UNAVAILABLE shape (item_id + name); PRICE_CHANGED needs
// a different one, because §8.1 specifies that code as failing "plus new
// totals" and a guest cannot re-confirm a number they were not given.
//
// One struct with two shapes rather than two types: `details` is a JSON
// array in the contract, every field is omitempty, and a client reads
// whichever fields the code it received implies. Splitting it would mean
// changing the envelope itself.
//
// The totals are pointers so that a genuine zero is distinguishable from
// absent — omitempty on a plain int64 would silently drop a total of 0,
// which is a real value for a fully-discounted cart.
type Detail struct {
	ItemID string `json:"item_id,omitempty"`
	Name   string `json:"name,omitempty"`

	// PRICE_CHANGED (FR-O7).
	ExpectedTotalMinor *int64 `json:"expected_total_minor,omitempty"`
	ActualTotalMinor   *int64 `json:"actual_total_minor,omitempty"`
	Currency           string `json:"currency,omitempty"`
}

// Error is the typed error every handler should return for an
// application-level failure; anything else is reported as CodeInternal.
type Error struct {
	Code    Code
	Message string
	Details []Detail
}

func (e *Error) Error() string { return string(e.Code) + ": " + e.Message }

func New(code Code, message string, details ...Detail) *Error {
	return &Error{Code: code, Message: message, Details: details}
}

// HTTPStatus returns the status code for a given Code, defaulting to 500
// for anything outside the closed set (should not happen if New is used
// consistently).
func HTTPStatus(code Code) int {
	if status, ok := httpStatus[code]; ok {
		return status
	}
	return http.StatusInternalServerError
}

// Body is the JSON shape written on the wire, matching docs/TZ.md §8.1.
type Body struct {
	Error struct {
		Code    Code     `json:"code"`
		Message string   `json:"message"`
		Details []Detail `json:"details,omitempty"`
		TraceID string   `json:"trace_id,omitempty"`
	} `json:"error"`
}
