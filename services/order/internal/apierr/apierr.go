// Package apierr builds the gRPC errors the order service returns, so
// that the gateway can reproduce docs/TZ.md §8.1's closed error-code set
// faithfully instead of guessing a code from a gRPC status.
//
// A bare gRPC code is not enough: ITEMS_UNAVAILABLE, PRICE_CHANGED,
// INVALID_TRANSITION, IDEMPOTENCY_KEY_REUSED, REQUEST_IN_PROGRESS and
// SESSION_CLOSED are all "409 Conflict" and all map to FailedPrecondition
// or Aborted, so the domain code travels alongside the status as a
// google.rpc.ErrorInfo detail. Per-item detail (the item list on
// ITEMS_UNAVAILABLE) travels as a google.rpc.PreconditionFailure.
//
// The gateway has its own copy of the Reason constants — internal
// packages can't cross a service boundary, and the wire contract here is
// the ErrorInfo shape, not shared Go code.
package apierr

import (
	"strconv"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Domain identifies this service as the producer of the ErrorInfo, per
// the google.rpc.ErrorInfo convention.
const Domain = "order.qrmenu"

// Reason values, one per docs/TZ.md §8.1 code that this service can
// actually produce. The gateway maps these to HTTP verbatim.
const (
	ReasonValidationFailed     = "VALIDATION_FAILED"
	ReasonNotFound             = "NOT_FOUND"
	ReasonItemsUnavailable     = "ITEMS_UNAVAILABLE"
	ReasonPriceChanged         = "PRICE_CHANGED"
	ReasonInvalidTransition    = "INVALID_TRANSITION"
	ReasonIdempotencyKeyReused = "IDEMPOTENCY_KEY_REUSED"
	ReasonRequestInProgress    = "REQUEST_IN_PROGRESS"
	ReasonSessionClosed        = "SESSION_CLOSED"
	ReasonCatalogUnavailable   = "CATALOG_UNAVAILABLE"
	ReasonInternal             = "INTERNAL"
)

// UnavailableItem is one entry of an ITEMS_UNAVAILABLE detail list.
type UnavailableItem struct {
	ItemID string
	Name   string
	Reason string // catalog's reason: OUT_OF_STOCK, INACTIVE, NOT_FOUND, …
}

func coded(c codes.Code, reason, msg string) error {
	st := status.New(c, msg)
	withInfo, err := st.WithDetails(&errdetails.ErrorInfo{Reason: reason, Domain: Domain})
	if err != nil {
		// WithDetails only fails if the status code is OK, which none of
		// the constructors below use. Fall back to the bare status rather
		// than losing the error entirely.
		return st.Err()
	}
	return withInfo.Err()
}

// Validation is a malformed or out-of-range request (FR-O5's numeric and
// length rules land here).
func Validation(msg string) error {
	return coded(codes.InvalidArgument, ReasonValidationFailed, msg)
}

// NotFound is a missing venue, order, session or request.
func NotFound(msg string) error {
	return coded(codes.NotFound, ReasonNotFound, msg)
}

// Internal is an unexpected failure; the message is for logs and should
// not be shown to a guest verbatim.
func Internal(msg string) error {
	return coded(codes.Internal, ReasonInternal, msg)
}

// InvalidTransition is a state-machine rejection (FR-K3).
func InvalidTransition(msg string) error {
	return coded(codes.FailedPrecondition, ReasonInvalidTransition, msg)
}

// SessionClosed is an attempt to write to a table session that has been
// paid and archived.
func SessionClosed(msg string) error {
	return coded(codes.FailedPrecondition, ReasonSessionClosed, msg)
}

// IdempotencyKeyReused is §7.4's "same key, different fingerprint".
func IdempotencyKeyReused(msg string) error {
	return coded(codes.AlreadyExists, ReasonIdempotencyKeyReused, msg)
}

// RequestInProgress is §7.4's "a second request while the first is
// running". Aborted is the code gRPC reserves for "retry at a higher
// level", which is exactly what the client should do here.
func RequestInProgress(msg string) error {
	return coded(codes.Aborted, ReasonRequestInProgress, msg)
}

// CatalogUnavailable is §7.4's fail-fast when the catalog dependency is
// down: the order can't be priced, and KDS keeps working on orders that
// were already placed.
func CatalogUnavailable(msg string) error {
	return coded(codes.Unavailable, ReasonCatalogUnavailable, msg)
}

// PriceChanged is FR-O7: the cart re-priced to something other than what
// the guest was shown, so the submit fails and they re-confirm rather than
// being charged a price they never saw.
//
// The new total rides along in the ErrorInfo metadata. Without it the
// guest UI can only say "the price changed" and force a full reload; with
// it, it can show the new number and a confirm button.
func PriceChanged(msg string, expectedMinor, actualMinor int64, currency string) error {
	st := status.New(codes.FailedPrecondition, msg)
	withInfo, err := st.WithDetails(&errdetails.ErrorInfo{
		Reason: ReasonPriceChanged,
		Domain: Domain,
		Metadata: map[string]string{
			"expected_total_minor": strconv.FormatInt(expectedMinor, 10),
			"actual_total_minor":   strconv.FormatInt(actualMinor, 10),
			"currency":             currency,
		},
	})
	if err != nil {
		return st.Err()
	}
	return withInfo.Err()
}

// ItemsUnavailable is FR-O6. Partial acceptance is not allowed in R1, so
// the whole submit fails and every offending item is named — the guest UI
// needs the full list to strike them from the cart in one pass.
func ItemsUnavailable(msg string, items []UnavailableItem) error {
	st := status.New(codes.FailedPrecondition, msg)

	violations := make([]*errdetails.PreconditionFailure_Violation, 0, len(items))
	for _, it := range items {
		violations = append(violations, &errdetails.PreconditionFailure_Violation{
			Type:        it.Reason,
			Subject:     it.ItemID,
			Description: it.Name,
		})
	}

	withDetails, err := st.WithDetails(
		&errdetails.ErrorInfo{Reason: ReasonItemsUnavailable, Domain: Domain},
		&errdetails.PreconditionFailure{Violations: violations},
	)
	if err != nil {
		return st.Err()
	}
	return withDetails.Err()
}
