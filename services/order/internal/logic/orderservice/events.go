package orderservicelogic

// Outbox event payloads for the qrmenu.order.v1 and
// qrmenu.service_request.v1 topics (docs/TZ.md §8.3).
//
// These are a published contract, consumed by the gateway's WebSocket
// fan-out today and by analytics/audit consumers later, so the rules from
// §8.3 apply: additive fields only, never renumber or repurpose one, and
// a breaking change means a new topic suffix.
//
// Transition payloads deliberately carry both `from` and `to` plus the
// actor and reason. FR-A4's admin audit log has no table of its own —
// §7.3 puts the durable audit trail in these events — so anything an
// auditor would need to reconstruct "who changed what, when, and why"
// has to be in here at publish time.

// Event type names, one per §8.3's R1 list that this service produces.
const (
	eventOrderPlaced             = "order.placed"
	eventOrderTransitioned       = "order.transitioned"
	eventOrderItemTransitioned   = "order.item_transitioned"
	eventOrderCancelled          = "order.cancelled"
	eventTableSessionOpened      = "table_session.opened"
	eventTableSessionClosed      = "table_session.closed"
	eventServiceRequestCreated   = "service_request.created"
	eventServiceRequestTransited = "service_request.transitioned"
)

type orderItemPayload struct {
	ID             string `json:"id"`
	MenuItemID     string `json:"menu_item_id"`
	Name           string `json:"name"`
	Qty            int32  `json:"qty"`
	UnitPriceMinor int64  `json:"unit_price_minor"`
	LineTotalMinor int64  `json:"line_total_minor"`
	Comment        string `json:"comment,omitempty"`
}

type orderPlacedPayload struct {
	OrderID        string             `json:"order_id"`
	Number         string             `json:"number"`
	TableID        string             `json:"table_id"`
	TableSessionID string             `json:"table_session_id"`
	GuestSessionID string             `json:"guest_session_id"`
	TotalMinor     int64              `json:"total_minor"`
	Currency       string             `json:"currency"`
	MenuVersion    string             `json:"menu_version"`
	Items          []orderItemPayload `json:"items"`
}

type orderTransitionedPayload struct {
	OrderID        string `json:"order_id"`
	Number         string `json:"number"`
	TableID        string `json:"table_id"`
	TableSessionID string `json:"table_session_id"`
	From           string `json:"from"`
	To             string `json:"to"`
	Reason         string `json:"reason,omitempty"`
	ActorStaffID   string `json:"actor_staff_id,omitempty"` // empty = the guest acted (FR-O11 self-cancel)
	TotalMinor     int64  `json:"total_minor"`
	Currency       string `json:"currency"`
}

type orderItemTransitionedPayload struct {
	OrderID      string `json:"order_id"`
	OrderItemID  string `json:"order_item_id"`
	MenuItemID   string `json:"menu_item_id"`
	TableID      string `json:"table_id"`
	From         string `json:"from"`
	To           string `json:"to"`
	Reason       string `json:"reason,omitempty"`
	ActorStaffID string `json:"actor_staff_id,omitempty"`
	// OrderStatus is the order's status after any FR-K5 auto-transition
	// this item change triggered, so a consumer doesn't have to infer it.
	OrderStatus string `json:"order_status"`
}

type tableSessionOpenedPayload struct {
	TableSessionID string `json:"table_session_id"`
	TableID        string `json:"table_id"`
	Currency       string `json:"currency"`
}

type tableSessionClosedPayload struct {
	TableSessionID string `json:"table_session_id"`
	TableID        string `json:"table_id"`
	TotalMinor     int64  `json:"total_minor"`
	Currency       string `json:"currency"`
	PaymentMethod  string `json:"payment_method"`
	ActorStaffID   string `json:"actor_staff_id,omitempty"`
}

type serviceRequestCreatedPayload struct {
	ServiceRequestID string `json:"service_request_id"`
	TableID          string `json:"table_id"`
	TableSessionID   string `json:"table_session_id"`
	Type             string `json:"type"`
	Note             string `json:"note,omitempty"`
}

type serviceRequestTransitionedPayload struct {
	ServiceRequestID string `json:"service_request_id"`
	TableID          string `json:"table_id"`
	TableSessionID   string `json:"table_session_id"`
	Type             string `json:"type"`
	From             string `json:"from"`
	To               string `json:"to"`
	ActorStaffID     string `json:"actor_staff_id,omitempty"`
}
