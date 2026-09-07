package orderservicelogic

import (
	"context"
	"errors"

	"github.com/menli02/QR-menu/services/order/internal/apierr"
	"github.com/menli02/QR-menu/services/order/internal/model"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// Endpoint names for the idempotency key's scope (venue_id, endpoint,
// key). They are stored in the database, so treat them as a wire format:
// renaming one silently un-deduplicates every in-flight key.
const (
	endpointCreateOrder          = "order.CreateOrder"
	endpointTransitionOrder      = "order.TransitionOrder"
	endpointTransitionOrderItem  = "order.TransitionOrderItem"
	endpointCreateServiceRequest = "order.CreateServiceRequest"
)

// peekIdempotent is the read-only fast path for a retry: if this key
// already has a stored response, decode and return it without opening a
// transaction or calling any downstream service.
//
// It is an optimisation, never the authority — runIdempotent re-checks
// under the row lock, so a key that appears free here and is taken a
// microsecond later still resolves correctly. The one case it decides
// outright is a fingerprint mismatch, which is a client bug that will not
// become valid by looking again.
func peekIdempotent[T proto.Message](
	ctx context.Context,
	db sqlx.SqlConn,
	endpoint, venueID, key, fingerprint string,
	empty func() T,
) (T, bool, error) {
	var zero T
	if key == "" {
		return zero, false, nil // runIdempotent reports the missing key
	}

	prior, err := model.NewIdempotencyModel(db).Find(ctx, venueID, endpoint, key)
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return zero, false, nil
		}
		logx.WithContext(ctx).Errorf("idempotency peek failed: %v", err)
		return zero, false, nil // fall through to the authoritative path
	}
	if prior.Fingerprint != fingerprint {
		return zero, false, apierr.IdempotencyKeyReused("idempotency key was used with a different request body")
	}
	if prior.InProgress {
		return zero, false, nil
	}

	replay := empty()
	if err := protojson.Unmarshal(prior.ResponseBody, replay); err != nil {
		logx.WithContext(ctx).Errorf("idempotency replay decode failed for %s/%s: %v", endpoint, key, err)
		return zero, false, nil
	}
	return replay, true, nil
}

// runIdempotent executes work exactly once per (venueID, endpoint, key),
// implementing docs/TZ.md §7.4.
//
// Everything — the key claim, the domain writes, the outbox row and the
// stored response — happens in one transaction. Three properties follow,
// and they are the reason the helper exists rather than each RPC rolling
// its own:
//
//   - A key is only ever recorded for a request that *succeeded*. A
//     handler that fails rolls its claim back too, so the same key is
//     immediately retryable and a transient error can't poison it.
//     Trade-off, stated because it is a real one: a 409 is therefore not
//     replayed from storage, it is recomputed. That is fine for the
//     conflicts this service raises (they are all deterministic given the
//     same state) and it keeps a failed write from occupying a key.
//
//   - A concurrent duplicate blocks on the claim INSERT until
//     lock_timeout, then surfaces as REQUEST_IN_PROGRESS rather than
//     racing the first request to a second order.
//
//   - The response a replay returns is byte-identical to the original,
//     because it is the original — stored as protojson at commit time.
//
// empty supplies a fresh message to decode a replayed response into;
// generics keep that type-safe without reflection at the call site.
func runIdempotent[T proto.Message](
	ctx context.Context,
	db sqlx.SqlConn,
	endpoint, venueID, key, fingerprint string,
	empty func() T,
	work func(ctx context.Context, s sqlx.Session) (T, error),
) (T, error) {
	var zero T
	if key == "" {
		return zero, apierr.Validation("idempotency_key is required")
	}

	var out T
	err := db.TransactCtx(ctx, func(ctx context.Context, s sqlx.Session) error {
		idem := model.NewIdempotencyModel(s)

		prior, owned, err := idem.Claim(ctx, venueID, endpoint, key, fingerprint)
		if err != nil {
			if model.IsLockUnavailable(err) {
				return apierr.RequestInProgress("another request with this idempotency key is in flight")
			}
			logx.WithContext(ctx).Errorf("idempotency claim failed: %v", err)
			return apierr.Internal("claim idempotency key")
		}

		if !owned {
			if prior.Fingerprint != fingerprint {
				return apierr.IdempotencyKeyReused("idempotency key was used with a different request body")
			}
			if prior.InProgress {
				// Only reachable if a previous deployment wrote the row in
				// its own transaction; with the single-transaction scheme
				// above, a committed row is always complete.
				return apierr.RequestInProgress("another request with this idempotency key is in flight")
			}
			replay := empty()
			if err := protojson.Unmarshal(prior.ResponseBody, replay); err != nil {
				logx.WithContext(ctx).Errorf("idempotency replay decode failed for %s/%s: %v", endpoint, key, err)
				return apierr.Internal("decode stored response")
			}
			out = replay
			return nil
		}

		result, err := work(ctx, s)
		if err != nil {
			return err
		}

		body, err := protojson.Marshal(result)
		if err != nil {
			logx.WithContext(ctx).Errorf("idempotency response encode failed: %v", err)
			return apierr.Internal("encode response")
		}
		if err := idem.Complete(ctx, venueID, endpoint, key, 200, body); err != nil {
			logx.WithContext(ctx).Errorf("idempotency complete failed: %v", err)
			return apierr.Internal("store idempotent response")
		}
		out = result
		return nil
	})
	if err != nil {
		return zero, err
	}
	return out, nil
}
