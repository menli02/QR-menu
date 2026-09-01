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

// Detail is one entry of the optional "details" array, e.g. one
// unavailable item on a 409 ITEMS_UNAVAILABLE response.
type Detail struct {
	ItemID string `json:"item_id,omitempty"`
	Name   string `json:"name,omitempty"`
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
