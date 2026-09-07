package orderservicelogic

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/menli02/QR-menu/proto/catalog/v1"
	"github.com/menli02/QR-menu/proto/order/v1"
	"github.com/menli02/QR-menu/services/order/internal/apierr"
	"github.com/menli02/QR-menu/services/order/internal/model"
	"github.com/menli02/QR-menu/services/order/internal/svc"
	"github.com/menli02/QR-menu/services/order/internal/venue"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type CreateOrderLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateOrderLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateOrderLogic {
	return &CreateOrderLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// CreateOrder is the transactional heart of the system (FR-O3..O9,
// docs/TZ.md §7.2).
//
// The three phases below follow §7.2's sequence diagram exactly, and the
// order matters:
//
//  1. Replay check and validation — cheap, no side effects.
//  2. catalog.ResolveOrderItems — a network call, made *before* any
//     transaction is open so no database locks are held across it.
//  3. One transaction: claim the idempotency key, open-or-join the table
//     session, register the guest session, reserve the order number,
//     insert the order, update the running total, write the outbox rows.
//
// Nothing about the client-supplied cart is trusted for money: prices,
// names and modifier validity all come from catalog in phase 2, and what
// is written in phase 3 is that snapshot (FR-O3).
func (l *CreateOrderLogic) CreateOrder(in *v1_orderpb.CreateOrderRequest) (*v1_orderpb.Order, error) {
	if err := validateCreateOrder(in); err != nil {
		return nil, err
	}

	settings, err := l.svcCtx.Venue.Get(l.ctx, in.GetVenueId())
	if err != nil {
		return nil, catalogError(err, "load venue settings")
	}
	if err := validateComments(in, settings.CommentMaxLen); err != nil {
		return nil, err
	}

	fingerprint := createOrderFingerprint(in)

	// Fast path: a retry of a request that already succeeded returns the
	// stored response without troubling catalog. Correctness doesn't
	// depend on this — runIdempotent below re-checks under the row lock —
	// but retries are the normal case on a flaky phone connection, and
	// this keeps them off the pricing path.
	if replay, found, err := peekIdempotent(l.ctx, l.svcCtx.DB, endpointCreateOrder,
		in.GetVenueId(), in.GetIdempotencyKey(), fingerprint, func() *v1_orderpb.Order { return &v1_orderpb.Order{} }); err != nil {
		return nil, err
	} else if found {
		return replay, nil
	}

	resolved, err := l.resolve(in, settings)
	if err != nil {
		return nil, err
	}
	if resolved.GetTotalMinor() > settings.OrderTotalLimitMinor && settings.OrderTotalLimitMinor > 0 {
		return nil, apierr.Validation(fmt.Sprintf("order total %d exceeds the venue limit of %d",
			resolved.GetTotalMinor(), settings.OrderTotalLimitMinor))
	}

	businessDate := settings.BusinessDate(time.Now())

	return runIdempotent(l.ctx, l.svcCtx.DB, endpointCreateOrder,
		in.GetVenueId(), in.GetIdempotencyKey(), fingerprint,
		func() *v1_orderpb.Order { return &v1_orderpb.Order{} },
		func(ctx context.Context, s sqlx.Session) (*v1_orderpb.Order, error) {
			return l.place(ctx, s, in, settings, resolved, businessDate)
		})
}

// resolve re-prices the cart against catalog and rejects the whole submit
// if anything is unavailable (FR-O6: partial acceptance is not allowed).
func (l *CreateOrderLogic) resolve(in *v1_orderpb.CreateOrderRequest, settings *venue.Settings) (*v1_catalogpb.ResolveOrderItemsResponse, error) {
	req := &v1_catalogpb.ResolveOrderItemsRequest{
		VenueId: in.GetVenueId(),
		Locale:  in.GetLocale(),
		Items:   make([]*v1_catalogpb.RequestedItem, 0, len(in.GetItems())),
	}
	for _, it := range in.GetItems() {
		req.Items = append(req.Items, &v1_catalogpb.RequestedItem{
			ItemId:            it.GetItemId(),
			Qty:               it.GetQty(),
			ModifierOptionIds: it.GetModifierOptionIds(),
			Comment:           it.GetComment(),
		})
	}

	resp, err := l.svcCtx.CatalogRpc.ResolveOrderItems(l.ctx, req)
	if err != nil {
		return nil, catalogError(err, "resolve order items")
	}

	if len(resp.GetUnavailable()) > 0 {
		items := make([]apierr.UnavailableItem, 0, len(resp.GetUnavailable()))
		for _, u := range resp.GetUnavailable() {
			items = append(items, apierr.UnavailableItem{
				ItemID: u.GetItemId(), Name: u.GetName(), Reason: u.GetReason(),
			})
		}
		return nil, apierr.ItemsUnavailable("some items are no longer available", items)
	}
	if len(resp.GetItems()) == 0 {
		return nil, apierr.Validation("order resolved to no items")
	}
	// Belt and braces: catalog owns the currency, but an order whose
	// currency disagrees with the table session's would corrupt the bill
	// silently, so the mismatch is caught here rather than at the CHECK.
	if resp.GetCurrency() != "" && settings.Currency != "" && resp.GetCurrency() != settings.Currency {
		l.Errorf("currency mismatch for venue %s: catalog resolved %q, settings say %q",
			in.GetVenueId(), resp.GetCurrency(), settings.Currency)
		return nil, apierr.Internal("venue currency is inconsistent")
	}
	return resp, nil
}

// place performs every write of the order, inside the caller's
// transaction.
func (l *CreateOrderLogic) place(
	ctx context.Context,
	s sqlx.Session,
	in *v1_orderpb.CreateOrderRequest,
	settings *venue.Settings,
	resolved *v1_catalogpb.ResolveOrderItemsResponse,
	businessDate time.Time,
) (*v1_orderpb.Order, error) {
	sessions := model.NewTableSessionModel(s)
	orders := model.NewOrderModel(s)
	outbox := model.NewOutboxModel(s)

	currency := resolved.GetCurrency()
	if currency == "" {
		currency = settings.Currency
	}

	// FR-O9: first order at a free table opens a session; later orders
	// join it. Whether we opened it decides if a table_session.opened
	// event is due.
	before, err := sessions.FindOpenByTable(ctx, in.GetVenueId(), in.GetTableId())
	opened := isNotFound(err)
	if err != nil && !opened {
		return nil, apierr.Internal("look up table session")
	}
	session, err := sessions.OpenOrJoin(ctx, in.GetVenueId(), in.GetTableId(), currency)
	if err != nil {
		l.Errorf("open or join table session: %v", err)
		return nil, apierr.Internal("open table session")
	}
	if !opened && before != nil && before.ID != session.ID {
		// The session we read was closed and a new one opened between the
		// two statements. Harmless — the order lands on the new one — but
		// worth a log line, because it means a guest ordered across a
		// table turnover.
		l.Infof("table %s turned over between read and join: %s -> %s", in.GetTableId(), before.ID, session.ID)
	}

	guest, err := model.NewGuestSessionModel(s).EnsureExists(ctx,
		in.GetGuestSessionId(), in.GetVenueId(), in.GetTableId(), session.ID, l.svcCtx.GuestSessionTTL)
	if err != nil {
		l.Errorf("register guest session: %v", err)
		return nil, apierr.Internal("register guest session")
	}
	// A guest whose table session was paid and closed must not be able to
	// keep ordering onto the next party's bill, even though their JWT has
	// not expired yet. Both checks matter: revoked_at catches the guest
	// who was at the table when it was settled, and the table_session_id
	// mismatch catches a token from an *earlier* session at the same
	// table (EnsureExists deliberately never re-points an existing row at
	// a new session — see its comment). Either way the fix is the same:
	// rescan the QR code for a fresh token.
	if guest.RevokedAt.Valid || guest.TableSessionID != session.ID {
		return nil, apierr.SessionClosed("this guest session has ended; rescan the QR code to start a new one")
	}

	seq, err := orders.NewOrderNumberSeq(ctx, in.GetVenueId(), businessDate)
	if err != nil {
		l.Errorf("reserve order number: %v", err)
		return nil, apierr.Internal("reserve order number")
	}

	params := model.InsertOrderParams{
		VenueID:        in.GetVenueId(),
		TableID:        in.GetTableId(),
		TableSessionID: session.ID,
		GuestSessionID: in.GetGuestSessionId(),
		BusinessDate:   businessDate,
		Number:         formatOrderNumber(seq),
		TotalMinor:     resolved.GetTotalMinor(),
		Currency:       currency,
		MenuVersion:    resolved.GetMenuVersion(),
		Items:          make([]model.InsertOrderItemParams, 0, len(resolved.GetItems())),
	}
	for _, ri := range resolved.GetItems() {
		item := model.InsertOrderItemParams{
			MenuItemID:     ri.GetItemId(),
			Name:           ri.GetName(),
			UnitPriceMinor: ri.GetUnitPrice().GetAmountMinor(),
			Qty:            ri.GetQty(),
			Comment:        ri.GetComment(),
			LineTotalMinor: ri.GetLineTotalMinor(),
		}
		for _, rm := range ri.GetModifiers() {
			item.Modifiers = append(item.Modifiers, model.InsertOrderItemModifierParams{
				OptionID:        rm.GetOptionId(),
				Name:            rm.GetName(),
				PriceDeltaMinor: rm.GetPriceDelta().GetAmountMinor(),
			})
		}
		params.Items = append(params.Items, item)
	}

	order, err := orders.Insert(ctx, params)
	if err != nil {
		l.Errorf("insert order: %v", err)
		return nil, apierr.Internal("insert order")
	}

	if err := sessions.AddToTotal(ctx, session.ID, order.TotalMinor); err != nil {
		l.Errorf("update table session total: %v", err)
		return nil, apierr.Internal("update table session total")
	}

	if opened {
		if err := outbox.Insert(ctx, eventTableSessionOpened, in.GetVenueId(), model.TopicOrder, session.TableID,
			tableSessionOpenedPayload{TableSessionID: session.ID, TableID: session.TableID, Currency: session.Currency},
			traceID(ctx)); err != nil {
			l.Errorf("write table_session.opened outbox row: %v", err)
			return nil, apierr.Internal("write outbox row")
		}
	}

	payload := orderPlacedPayload{
		OrderID:        order.ID,
		Number:         order.Number,
		TableID:        order.TableID,
		TableSessionID: order.TableSessionID,
		GuestSessionID: order.GuestSessionID,
		TotalMinor:     order.TotalMinor,
		Currency:       order.Currency,
		MenuVersion:    order.MenuVersion,
		Items:          make([]orderItemPayload, 0, len(order.Items)),
	}
	for _, it := range order.Items {
		payload.Items = append(payload.Items, orderItemPayload{
			ID: it.ID, MenuItemID: it.MenuItemID, Name: it.Name, Qty: it.Qty,
			UnitPriceMinor: it.UnitPriceMinor, LineTotalMinor: it.LineTotalMinor,
			Comment: it.Comment.String,
		})
	}
	if err := outbox.Insert(ctx, eventOrderPlaced, in.GetVenueId(), model.TopicOrder, order.ID, payload, traceID(ctx)); err != nil {
		l.Errorf("write order.placed outbox row: %v", err)
		return nil, apierr.Internal("write outbox row")
	}

	return orderToProto(order), nil
}

// ---------------------------------------------------------------------
// Validation (FR-O5)
// ---------------------------------------------------------------------

func validateCreateOrder(in *v1_orderpb.CreateOrderRequest) error {
	switch {
	case in.GetVenueId() == "":
		return apierr.Validation("venue_id is required")
	case in.GetTableId() == "":
		return apierr.Validation("table_id is required")
	case in.GetGuestSessionId() == "":
		return apierr.Validation("guest_session_id is required")
	case len(in.GetItems()) == 0:
		return apierr.Validation("an order must contain at least one item")
	case len(in.GetItems()) > venue.MaxItemsPerOrder:
		return apierr.Validation(fmt.Sprintf("an order may contain at most %d lines", venue.MaxItemsPerOrder))
	}
	for i, it := range in.GetItems() {
		if it.GetItemId() == "" {
			return apierr.Validation(fmt.Sprintf("items[%d].item_id is required", i))
		}
		if it.GetQty() < 1 || it.GetQty() > 99 {
			return apierr.Validation(fmt.Sprintf("items[%d].qty must be between 1 and 99", i))
		}
	}
	return nil
}

// validateComments is separate from validateCreateOrder because the limit
// is a venue setting, so it can only run once settings are loaded.
// Length is counted in runes: a 200-character limit that rejects 70
// Japanese characters would be a bug, not a policy.
func validateComments(in *v1_orderpb.CreateOrderRequest, maxLen int) error {
	for i, it := range in.GetItems() {
		if n := utf8.RuneCountInString(it.GetComment()); n > maxLen {
			return apierr.Validation(fmt.Sprintf("items[%d].comment is %d characters, the limit is %d", i, n, maxLen))
		}
	}
	return nil
}

// createOrderFingerprint hashes the parts of the request that define what
// was ordered (§7.4: "SHA-256 of the canonical request body").
//
// Modifier ids are sorted so that two clients sending the same selection
// in a different order share a fingerprint — they *are* the same request,
// and rejecting the retry as IDEMPOTENCY_KEY_REUSED would be wrong.
// Item order is preserved: two lines of the same item with different
// comments are distinguishable, and reordering lines is a different cart.
func createOrderFingerprint(in *v1_orderpb.CreateOrderRequest) string {
	parts := []string{in.GetVenueId(), in.GetTableId(), in.GetGuestSessionId(), in.GetLocale()}
	for _, it := range in.GetItems() {
		mods := append([]string(nil), it.GetModifierOptionIds()...)
		sort.Strings(mods)
		parts = append(parts,
			it.GetItemId(),
			strconv.FormatInt(int64(it.GetQty()), 10),
			strings.Join(mods, ","),
			it.GetComment(),
		)
	}
	return model.Fingerprint(parts...)
}

// formatOrderNumber renders FR-O8's human ticket number: "A-001" through
// "A-999", then "B-001", and so on. The letter is what staff actually
// call across a kitchen, so it advances slowly on purpose.
//
// A venue that somehow exceeds 26 * 999 orders in one business day falls
// back to a plain sequence rather than wrapping around to "A-001" and
// colliding with a ticket from that morning.
func formatOrderNumber(seq int) string {
	if seq < 1 {
		seq = 1
	}
	const perLetter = 999
	idx := (seq - 1) / perLetter
	if idx > 25 {
		return strconv.Itoa(seq)
	}
	return fmt.Sprintf("%c-%03d", 'A'+rune(idx), (seq-1)%perLetter+1)
}

// catalogError maps a failed catalog call onto §7.4's fail-fast rule:
// catalog down means the order can't be priced, so the submit fails with
// CATALOG_UNAVAILABLE rather than a generic 500, and KDS carries on with
// the orders it already has.
func catalogError(err error, what string) error {
	if st, ok := status.FromError(err); ok {
		switch st.Code() {
		case codes.NotFound:
			return apierr.NotFound(st.Message())
		case codes.InvalidArgument:
			return apierr.Validation(st.Message())
		case codes.Unavailable, codes.DeadlineExceeded, codes.ResourceExhausted:
			return apierr.CatalogUnavailable(what + ": catalog is unavailable")
		}
	}
	logx.Errorf("%s: %v", what, err)
	return apierr.Internal(what)
}
