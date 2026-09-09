package rpcerr

import (
	"errors"
	"testing"

	"github.com/menli02/QR-menu/services/gateway/internal/errs"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// coded builds the kind of error the order service produces: a status
// carrying a google.rpc.ErrorInfo with the domain code.
func coded(c codes.Code, reason, msg string) error {
	st := status.New(c, msg)
	withInfo, err := st.WithDetails(&errdetails.ErrorInfo{Reason: reason, Domain: "order.qrmenu"})
	if err != nil {
		panic(err)
	}
	return withInfo.Err()
}

// TestErrorInfoWins is the whole reason this package exists: several §8.1
// codes share one gRPC status, so the reason string has to take priority
// over the status code.
func TestErrorInfoWins(t *testing.T) {
	cases := []struct {
		reason string
		status codes.Code
		want   errs.Code
	}{
		{"ITEMS_UNAVAILABLE", codes.FailedPrecondition, errs.CodeItemsUnavailable},
		{"INVALID_TRANSITION", codes.FailedPrecondition, errs.CodeInvalidTransition},
		{"SESSION_CLOSED", codes.FailedPrecondition, errs.CodeSessionClosed},
		{"IDEMPOTENCY_KEY_REUSED", codes.AlreadyExists, errs.CodeIdempotencyKeyReused},
		{"REQUEST_IN_PROGRESS", codes.Aborted, errs.CodeRequestInProgress},
		{"CATALOG_UNAVAILABLE", codes.Unavailable, errs.CodeCatalogUnavailable},
		{"VALIDATION_FAILED", codes.InvalidArgument, errs.CodeValidationFailed},
		{"NOT_FOUND", codes.NotFound, errs.CodeNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.reason, func(t *testing.T) {
			got := From(coded(tc.status, tc.reason, "something happened"))
			if got.Code != tc.want {
				t.Errorf("code = %q, want %q", got.Code, tc.want)
			}
			if got.Message != "something happened" {
				t.Errorf("message = %q, want it preserved", got.Message)
			}
		})
	}

	// Three FailedPrecondition errors that must NOT collapse together —
	// the failure this package prevents.
	a := From(coded(codes.FailedPrecondition, "ITEMS_UNAVAILABLE", "x"))
	b := From(coded(codes.FailedPrecondition, "INVALID_TRANSITION", "x"))
	c := From(coded(codes.FailedPrecondition, "SESSION_CLOSED", "x"))
	if a.Code == b.Code || b.Code == c.Code || a.Code == c.Code {
		t.Errorf("distinct reasons collapsed to the same code: %q %q %q", a.Code, b.Code, c.Code)
	}
}

// TestUnknownReasonFallsBackToStatus covers the closed-set rule: a reason
// nobody documented must not become a code clients have never seen.
func TestUnknownReasonFallsBackToStatus(t *testing.T) {
	got := From(coded(codes.NotFound, "SOMETHING_NEW", "nope"))
	if got.Code != errs.CodeNotFound {
		t.Errorf("code = %q, want NOT_FOUND from the status", got.Code)
	}
}

func TestBareStatusMapping(t *testing.T) {
	cases := []struct {
		in   codes.Code
		want errs.Code
	}{
		{codes.NotFound, errs.CodeNotFound},
		{codes.InvalidArgument, errs.CodeValidationFailed},
		{codes.OutOfRange, errs.CodeValidationFailed},
		{codes.FailedPrecondition, errs.CodeValidationFailed},
		{codes.AlreadyExists, errs.CodeValidationFailed},
		{codes.Unauthenticated, errs.CodeUnauthenticated},
		{codes.PermissionDenied, errs.CodeForbidden},
		{codes.ResourceExhausted, errs.CodeRateLimited},
		{codes.Internal, errs.CodeInternal},
		{codes.Unknown, errs.CodeInternal},
		{codes.DataLoss, errs.CodeInternal},
	}
	for _, tc := range cases {
		t.Run(tc.in.String(), func(t *testing.T) {
			if got := From(status.Error(tc.in, "msg")); got.Code != tc.want {
				t.Errorf("From(%v) = %q, want %q", tc.in, got.Code, tc.want)
			}
		})
	}
}

// TestUnavailableDependsOnCallee is why there are two entry points: an
// unreachable catalog has its own documented code (§7.4), an unreachable
// anything-else does not.
func TestUnavailableDependsOnCallee(t *testing.T) {
	for _, c := range []codes.Code{codes.Unavailable, codes.DeadlineExceeded} {
		if got := FromCatalog(status.Error(c, "down")); got.Code != errs.CodeCatalogUnavailable {
			t.Errorf("FromCatalog(%v) = %q, want CATALOG_UNAVAILABLE", c, got.Code)
		}
		if got := From(status.Error(c, "down")); got.Code != errs.CodeInternal {
			t.Errorf("From(%v) = %q, want INTERNAL", c, got.Code)
		}
	}
}

// TestInternalMessageIsNotLeaked guards a real disclosure risk: an
// Internal error's text is whatever went wrong inside a service — a query,
// a hostname, a driver string — and none of it belongs in a public
// response.
func TestInternalMessageIsNotLeaked(t *testing.T) {
	secret := `pq: password authentication failed for user "qrmenu" at 10.0.3.14`
	got := From(status.Error(codes.Internal, secret))
	if got.Code != errs.CodeInternal {
		t.Fatalf("code = %q, want INTERNAL", got.Code)
	}
	if got.Message == secret {
		t.Error("the internal error text was passed through to the client")
	}
	if got.Message != "internal error" {
		t.Errorf("message = %q, want a fixed placeholder", got.Message)
	}
}

// TestPreconditionFailureBecomesDetails is FR-O6: the guest UI needs every
// unavailable item named so it can strike them from the cart in one pass.
func TestPreconditionFailureBecomesDetails(t *testing.T) {
	st := status.New(codes.FailedPrecondition, "some items are no longer available")
	withDetails, err := st.WithDetails(
		&errdetails.ErrorInfo{Reason: "ITEMS_UNAVAILABLE", Domain: "order.qrmenu"},
		&errdetails.PreconditionFailure{Violations: []*errdetails.PreconditionFailure_Violation{
			{Type: "OUT_OF_STOCK", Subject: "item-1", Description: "Flat White"},
			{Type: "INACTIVE", Subject: "item-2", Description: "Croissant"},
		}},
	)
	if err != nil {
		t.Fatalf("build status: %v", err)
	}

	got := From(withDetails.Err())
	if got.Code != errs.CodeItemsUnavailable {
		t.Fatalf("code = %q, want ITEMS_UNAVAILABLE", got.Code)
	}
	if len(got.Details) != 2 {
		t.Fatalf("details = %d, want 2", len(got.Details))
	}
	if got.Details[0].ItemID != "item-1" || got.Details[0].Name != "Flat White" {
		t.Errorf("details[0] = %+v", got.Details[0])
	}
	if got.Details[1].ItemID != "item-2" || got.Details[1].Name != "Croissant" {
		t.Errorf("details[1] = %+v", got.Details[1])
	}
}

func TestNonGRPCError(t *testing.T) {
	got := From(errors.New("not a grpc error"))
	// status.FromError treats a plain error as Unknown, which maps to
	// INTERNAL — and the text must not survive.
	if got.Code != errs.CodeInternal || got.Message != "internal error" {
		t.Errorf("got %+v, want an opaque internal error", got)
	}
}

func TestNilIsNil(t *testing.T) {
	if got := From(nil); got != nil {
		t.Errorf("From(nil) = %v, want nil", got)
	}
}

// TestEveryReasonMapsToAKnownCode keeps the two closed sets in step: a
// reason string with no errs.Code, or one whose code has no HTTP status,
// would silently degrade to 500 at runtime.
func TestEveryReasonMapsToAKnownCode(t *testing.T) {
	for reason, code := range reasonToCode {
		if code == "" {
			t.Errorf("reason %q maps to an empty code", reason)
		}
		if got := errs.HTTPStatus(code); got == 500 && code != errs.CodeInternal {
			t.Errorf("code %q (from reason %q) has no HTTP mapping", code, reason)
		}
	}
}

// TestPriceChangedCarriesTheNewTotals is §8.1's requirement that
// PRICE_CHANGED fails "plus new totals". Without them a guest UI can only
// say "the price changed" and force a full reload, which is precisely the
// re-confirmation flow FR-O7 exists to avoid.
func TestPriceChangedCarriesTheNewTotals(t *testing.T) {
	st := status.New(codes.FailedPrecondition, "prices changed")
	withInfo, err := st.WithDetails(&errdetails.ErrorInfo{
		Reason: "PRICE_CHANGED",
		Domain: "order.qrmenu",
		Metadata: map[string]string{
			"expected_total_minor": "800",
			"actual_total_minor":   "900",
			"currency":             "USD",
		},
	})
	if err != nil {
		t.Fatalf("build status: %v", err)
	}

	got := From(withInfo.Err())
	if got.Code != errs.CodePriceChanged {
		t.Fatalf("code = %q, want PRICE_CHANGED", got.Code)
	}
	if len(got.Details) != 1 {
		t.Fatalf("details = %d, want 1", len(got.Details))
	}
	d := got.Details[0]
	if d.ExpectedTotalMinor == nil || *d.ExpectedTotalMinor != 800 {
		t.Errorf("expected total = %v, want 800", d.ExpectedTotalMinor)
	}
	if d.ActualTotalMinor == nil || *d.ActualTotalMinor != 900 {
		t.Errorf("actual total = %v, want 900", d.ActualTotalMinor)
	}
	if d.Currency != "USD" {
		t.Errorf("currency = %q, want USD", d.Currency)
	}
}

// TestPriceChangedWithoutUsableMetadata: a missing or malformed number
// must produce no detail rather than a confident zero. "The price changed,
// new total 0" is worse than saying nothing.
func TestPriceChangedWithoutUsableMetadata(t *testing.T) {
	for _, meta := range []map[string]string{
		nil,
		{},
		{"currency": "USD"},
		{"expected_total_minor": "not-a-number", "actual_total_minor": "also-not"},
	} {
		st := status.New(codes.FailedPrecondition, "prices changed")
		withInfo, err := st.WithDetails(&errdetails.ErrorInfo{
			Reason: "PRICE_CHANGED", Domain: "order.qrmenu", Metadata: meta,
		})
		if err != nil {
			t.Fatalf("build status: %v", err)
		}
		got := From(withInfo.Err())
		if got.Code != errs.CodePriceChanged {
			t.Errorf("code = %q, want PRICE_CHANGED even without usable metadata", got.Code)
		}
		if len(got.Details) != 0 {
			t.Errorf("metadata %v produced details %+v, want none", meta, got.Details)
		}
	}
}

// TestPriceChangedZeroTotalIsCarried is why the totals are pointers: a
// fully-discounted cart really can total zero, and omitempty on a plain
// int64 would silently drop it.
func TestPriceChangedZeroTotalIsCarried(t *testing.T) {
	st := status.New(codes.FailedPrecondition, "prices changed")
	withInfo, _ := st.WithDetails(&errdetails.ErrorInfo{
		Reason: "PRICE_CHANGED", Domain: "order.qrmenu",
		Metadata: map[string]string{"expected_total_minor": "500", "actual_total_minor": "0", "currency": "USD"},
	})
	got := From(withInfo.Err())
	if len(got.Details) != 1 || got.Details[0].ActualTotalMinor == nil {
		t.Fatalf("a zero total was dropped: %+v", got.Details)
	}
	if *got.Details[0].ActualTotalMinor != 0 {
		t.Errorf("actual total = %d, want 0", *got.Details[0].ActualTotalMinor)
	}
}
