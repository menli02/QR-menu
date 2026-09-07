package orderservicelogic

import (
	"context"
	"errors"

	"github.com/menli02/QR-menu/services/order/internal/model"

	"go.opentelemetry.io/otel/trace"
)

// isNotFound is the logic layer's shorthand for "no such row". Every
// model returns sqlx's sentinel unchanged, so one predicate covers them
// all.
func isNotFound(err error) bool {
	return errors.Is(err, model.ErrNotFound)
}

// traceID copies the current trace id onto outbox rows so an event can be
// correlated back to the request that produced it (docs/TZ.md §8.3's
// envelope carries trace_id). Empty when the request arrived without a
// trace context, which is normal in local development.
func traceID(ctx context.Context) string {
	sc := trace.SpanContextFromContext(ctx)
	if !sc.HasTraceID() {
		return ""
	}
	return sc.TraceID().String()
}

// defaultPageSize and maxPageSize bound ListTickets. A KDS screen shows
// tens of tickets; the cap exists so a client asking for everything
// can't turn one request into a full-table scan.
const (
	defaultPageSize = 50
	maxPageSize     = 200
)

func clampPageSize(n int32) int {
	switch {
	case n <= 0:
		return defaultPageSize
	case int(n) > maxPageSize:
		return maxPageSize
	default:
		return int(n)
	}
}
