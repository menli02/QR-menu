// Package rpcerr translates a gRPC error from a backing service into the
// gateway's public error envelope (docs/TZ.md §8.1).
//
// There are two sources of truth, in priority order:
//
//  1. A google.rpc.ErrorInfo detail. The order service attaches one to
//     every error it raises (see its internal/apierr), because
//     ITEMS_UNAVAILABLE, INVALID_TRANSITION, SESSION_CLOSED,
//     IDEMPOTENCY_KEY_REUSED and REQUEST_IN_PROGRESS all share the same
//     gRPC status and are only distinguishable by the reason string.
//  2. Failing that, the bare gRPC status code, mapped conservatively.
//
// The Reason strings are duplicated here rather than imported: a service's
// internal/ package can't cross a service boundary, and the contract
// between them is the ErrorInfo wire shape, not shared Go code.
package rpcerr

import (
	"github.com/menli02/QR-menu/services/gateway/internal/errs"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// reasonToCode maps a google.rpc.ErrorInfo reason onto §8.1's closed set.
// An unrecognised reason is deliberately not passed through as a code —
// the set is closed, and inventing a code a client has never seen is
// worse than reporting the generic one.
var reasonToCode = map[string]errs.Code{
	"VALIDATION_FAILED":      errs.CodeValidationFailed,
	"UNAUTHENTICATED":        errs.CodeUnauthenticated,
	"FORBIDDEN":              errs.CodeForbidden,
	"NOT_FOUND":              errs.CodeNotFound,
	"ITEMS_UNAVAILABLE":      errs.CodeItemsUnavailable,
	"PRICE_CHANGED":          errs.CodePriceChanged,
	"INVALID_TRANSITION":     errs.CodeInvalidTransition,
	"IDEMPOTENCY_KEY_REUSED": errs.CodeIdempotencyKeyReused,
	"REQUEST_IN_PROGRESS":    errs.CodeRequestInProgress,
	"RATE_LIMITED":           errs.CodeRateLimited,
	"TABLE_INACTIVE":         errs.CodeTableInactive,
	"SESSION_CLOSED":         errs.CodeSessionClosed,
	"CATALOG_UNAVAILABLE":    errs.CodeCatalogUnavailable,
	"INTERNAL":               errs.CodeInternal,
}

// From converts a gRPC error into the public error. Use it for calls to
// order and identity.
func From(err error) *errs.Error {
	return convert(err, errs.CodeInternal)
}

// FromCatalog is From for calls to the catalog service, where an
// unreachable dependency has its own documented code: §7.4 says an order
// submit fails fast with CATALOG_UNAVAILABLE rather than a generic 500,
// so the KDS can keep working on orders that were already placed.
func FromCatalog(err error) *errs.Error {
	return convert(err, errs.CodeCatalogUnavailable)
}

func convert(err error, unavailable errs.Code) *errs.Error {
	if err == nil {
		return nil
	}
	st, ok := status.FromError(err)
	if !ok {
		return errs.New(errs.CodeInternal, "internal error")
	}

	code, details := fromDetails(st)
	if code == "" {
		code = fromStatusCode(st.Code(), unavailable)
	}

	// The message is safe to surface only for errors a service raised
	// deliberately. An Internal one is whatever went wrong inside — it
	// can carry a query, a hostname, a driver string — so it is replaced
	// with a fixed message and the real text stays in the logs.
	message := st.Message()
	if code == errs.CodeInternal {
		message = "internal error"
	}

	return errs.New(code, message, details...)
}

// fromDetails reads the structured details a service attached: ErrorInfo
// for the code, PreconditionFailure for the per-item list that
// ITEMS_UNAVAILABLE carries (FR-O6: the guest UI needs the whole list to
// strike those items from the cart in one pass).
func fromDetails(st *status.Status) (errs.Code, []errs.Detail) {
	var code errs.Code
	var details []errs.Detail

	for _, d := range st.Details() {
		switch info := d.(type) {
		case *errdetails.ErrorInfo:
			if mapped, ok := reasonToCode[info.GetReason()]; ok {
				code = mapped
			}
		case *errdetails.PreconditionFailure:
			for _, v := range info.GetViolations() {
				details = append(details, errs.Detail{
					ItemID: v.GetSubject(),
					Name:   v.GetDescription(),
				})
			}
		}
	}
	return code, details
}

// fromStatusCode is the fallback for a service that returns a bare
// status — catalog and identity mostly do.
//
// FailedPrecondition and AlreadyExists both land on VALIDATION_FAILED
// rather than being guessed at: §8.1's set has no generic conflict code,
// and labelling an arbitrary precondition failure as INVALID_TRANSITION
// would tell a client something untrue. A service that needs a specific
// 409 attaches an ErrorInfo, which fromDetails picks up first.
func fromStatusCode(c codes.Code, unavailable errs.Code) errs.Code {
	switch c {
	case codes.NotFound:
		return errs.CodeNotFound
	case codes.InvalidArgument, codes.OutOfRange, codes.FailedPrecondition, codes.AlreadyExists:
		return errs.CodeValidationFailed
	case codes.Unauthenticated:
		return errs.CodeUnauthenticated
	case codes.PermissionDenied:
		return errs.CodeForbidden
	case codes.ResourceExhausted:
		return errs.CodeRateLimited
	case codes.Unavailable, codes.DeadlineExceeded:
		return unavailable
	default:
		return errs.CodeInternal
	}
}
