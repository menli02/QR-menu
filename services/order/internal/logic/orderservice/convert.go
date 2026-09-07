package orderservicelogic

import (
	"github.com/menli02/QR-menu/proto/order/v1"
	"github.com/menli02/QR-menu/services/order/internal/model"

	"database/sql"

	"google.golang.org/protobuf/types/known/timestamppb"
)

// This file is the single boundary between order_db's TEXT status columns
// and the proto enums. Every mapping is spelled out in both directions
// rather than derived from the enum names: the DB values are a storage
// contract that must not change if a proto enum is ever renamed, and an
// unrecognised value maps to _UNSPECIFIED instead of panicking, so a row
// written by a newer deployment can still be read by an older one.

var orderStatusToProto = map[string]v1_orderpb.OrderStatus{
	model.OrderPlaced:     v1_orderpb.OrderStatus_ORDER_STATUS_PLACED,
	model.OrderAccepted:   v1_orderpb.OrderStatus_ORDER_STATUS_ACCEPTED,
	model.OrderInProgress: v1_orderpb.OrderStatus_ORDER_STATUS_IN_PROGRESS,
	model.OrderReady:      v1_orderpb.OrderStatus_ORDER_STATUS_READY,
	model.OrderServed:     v1_orderpb.OrderStatus_ORDER_STATUS_SERVED,
	model.OrderCancelled:  v1_orderpb.OrderStatus_ORDER_STATUS_CANCELLED,
}

var orderStatusFromProto = map[v1_orderpb.OrderStatus]string{
	v1_orderpb.OrderStatus_ORDER_STATUS_PLACED:      model.OrderPlaced,
	v1_orderpb.OrderStatus_ORDER_STATUS_ACCEPTED:    model.OrderAccepted,
	v1_orderpb.OrderStatus_ORDER_STATUS_IN_PROGRESS: model.OrderInProgress,
	v1_orderpb.OrderStatus_ORDER_STATUS_READY:       model.OrderReady,
	v1_orderpb.OrderStatus_ORDER_STATUS_SERVED:      model.OrderServed,
	v1_orderpb.OrderStatus_ORDER_STATUS_CANCELLED:   model.OrderCancelled,
}

var itemStatusToProto = map[string]v1_orderpb.OrderItemStatus{
	model.ItemPlaced:    v1_orderpb.OrderItemStatus_ORDER_ITEM_STATUS_PLACED,
	model.ItemCooking:   v1_orderpb.OrderItemStatus_ORDER_ITEM_STATUS_COOKING,
	model.ItemReady:     v1_orderpb.OrderItemStatus_ORDER_ITEM_STATUS_READY,
	model.ItemCancelled: v1_orderpb.OrderItemStatus_ORDER_ITEM_STATUS_CANCELLED,
}

var itemStatusFromProto = map[v1_orderpb.OrderItemStatus]string{
	v1_orderpb.OrderItemStatus_ORDER_ITEM_STATUS_PLACED:    model.ItemPlaced,
	v1_orderpb.OrderItemStatus_ORDER_ITEM_STATUS_COOKING:   model.ItemCooking,
	v1_orderpb.OrderItemStatus_ORDER_ITEM_STATUS_READY:     model.ItemReady,
	v1_orderpb.OrderItemStatus_ORDER_ITEM_STATUS_CANCELLED: model.ItemCancelled,
}

var sessionStatusToProto = map[string]v1_orderpb.TableSessionStatus{
	model.TableSessionOpen:   v1_orderpb.TableSessionStatus_TABLE_SESSION_STATUS_OPEN,
	model.TableSessionClosed: v1_orderpb.TableSessionStatus_TABLE_SESSION_STATUS_CLOSED,
}

var requestTypeToProto = map[string]v1_orderpb.ServiceRequestType{
	model.RequestCallWaiter:  v1_orderpb.ServiceRequestType_SERVICE_REQUEST_TYPE_CALL_WAITER,
	model.RequestRequestBill: v1_orderpb.ServiceRequestType_SERVICE_REQUEST_TYPE_REQUEST_BILL,
}

var requestTypeFromProto = map[v1_orderpb.ServiceRequestType]string{
	v1_orderpb.ServiceRequestType_SERVICE_REQUEST_TYPE_CALL_WAITER:  model.RequestCallWaiter,
	v1_orderpb.ServiceRequestType_SERVICE_REQUEST_TYPE_REQUEST_BILL: model.RequestRequestBill,
}

var requestStatusToProto = map[string]v1_orderpb.ServiceRequestStatus{
	model.RequestOpen:         v1_orderpb.ServiceRequestStatus_SERVICE_REQUEST_STATUS_OPEN,
	model.RequestAcknowledged: v1_orderpb.ServiceRequestStatus_SERVICE_REQUEST_STATUS_ACKNOWLEDGED,
	model.RequestResolved:     v1_orderpb.ServiceRequestStatus_SERVICE_REQUEST_STATUS_RESOLVED,
	model.RequestExpired:      v1_orderpb.ServiceRequestStatus_SERVICE_REQUEST_STATUS_EXPIRED,
}

var requestStatusFromProto = map[v1_orderpb.ServiceRequestStatus]string{
	v1_orderpb.ServiceRequestStatus_SERVICE_REQUEST_STATUS_OPEN:         model.RequestOpen,
	v1_orderpb.ServiceRequestStatus_SERVICE_REQUEST_STATUS_ACKNOWLEDGED: model.RequestAcknowledged,
	v1_orderpb.ServiceRequestStatus_SERVICE_REQUEST_STATUS_RESOLVED:     model.RequestResolved,
	v1_orderpb.ServiceRequestStatus_SERVICE_REQUEST_STATUS_EXPIRED:      model.RequestExpired,
}

var paymentMethodToProto = map[string]v1_orderpb.PaymentMethod{
	"cash":          v1_orderpb.PaymentMethod_PAYMENT_METHOD_CASH,
	"card_terminal": v1_orderpb.PaymentMethod_PAYMENT_METHOD_CARD_TERMINAL,
	"other":         v1_orderpb.PaymentMethod_PAYMENT_METHOD_OTHER,
}

var paymentMethodFromProto = map[v1_orderpb.PaymentMethod]string{
	v1_orderpb.PaymentMethod_PAYMENT_METHOD_CASH:          "cash",
	v1_orderpb.PaymentMethod_PAYMENT_METHOD_CARD_TERMINAL: "card_terminal",
	v1_orderpb.PaymentMethod_PAYMENT_METHOD_OTHER:         "other",
}

func money(amountMinor int64, currency string) *v1_orderpb.Money {
	return &v1_orderpb.Money{AmountMinor: amountMinor, Currency: currency}
}

// tsOrNil keeps an unset SQL timestamp as an absent proto field rather
// than the zero instant, which would render as 1970 in every client.
func tsOrNil(t sql.NullTime) *timestamppb.Timestamp {
	if !t.Valid {
		return nil
	}
	return timestamppb.New(t.Time)
}

func orderToProto(o *model.Order) *v1_orderpb.Order {
	out := &v1_orderpb.Order{
		Id:             o.ID,
		VenueId:        o.VenueID,
		TableId:        o.TableID,
		TableSessionId: o.TableSessionID,
		GuestSessionId: o.GuestSessionID,
		Number:         o.Number,
		Status:         orderStatusToProto[o.Status],
		TotalMinor:     o.TotalMinor,
		Currency:       o.Currency,
		MenuVersion:    o.MenuVersion,
		PlacedAt:       timestamppb.New(o.PlacedAt),
		UpdatedAt:      timestamppb.New(o.UpdatedAt),
	}
	out.Items = make([]*v1_orderpb.OrderItem, 0, len(o.Items))
	for i := range o.Items {
		out.Items = append(out.Items, orderItemToProto(&o.Items[i], o.Currency))
	}
	return out
}

func orderItemToProto(it *model.OrderItem, currency string) *v1_orderpb.OrderItem {
	out := &v1_orderpb.OrderItem{
		Id:             it.ID,
		MenuItemId:     it.MenuItemID,
		Name:           it.Name,
		UnitPrice:      money(it.UnitPriceMinor, currency),
		Qty:            it.Qty,
		Comment:        it.Comment.String,
		Status:         itemStatusToProto[it.Status],
		LineTotalMinor: it.LineTotalMinor,
	}
	out.Modifiers = make([]*v1_orderpb.OrderItemModifier, 0, len(it.Modifiers))
	for _, m := range it.Modifiers {
		out.Modifiers = append(out.Modifiers, &v1_orderpb.OrderItemModifier{
			OptionId:   m.OptionID,
			Name:       m.Name,
			PriceDelta: money(m.PriceDeltaMinor, currency),
		})
	}
	return out
}

func tableSessionToProto(s *model.TableSession) *v1_orderpb.TableSession {
	return &v1_orderpb.TableSession{
		Id:              s.ID,
		VenueId:         s.VenueID,
		TableId:         s.TableID,
		Status:          sessionStatusToProto[s.Status],
		OpenedAt:        timestamppb.New(s.OpenedAt),
		ClosedAt:        tsOrNil(s.ClosedAt),
		ClosedByStaffId: s.ClosedByStaffID.String,
		PaymentMethod:   paymentMethodToProto[s.PaymentMethod.String],
		TotalMinor:      s.TotalMinor,
		Currency:        s.Currency,
	}
}

func serviceRequestToProto(r *model.ServiceRequest) *v1_orderpb.ServiceRequest {
	return &v1_orderpb.ServiceRequest{
		Id:             r.ID,
		VenueId:        r.VenueID,
		TableId:        r.TableID,
		TableSessionId: r.TableSessionID,
		Type:           requestTypeToProto[r.Type],
		Status:         requestStatusToProto[r.Status],
		Note:           r.Note.String,
		CreatedAt:      timestamppb.New(r.CreatedAt),
		AcknowledgedAt: tsOrNil(r.AcknowledgedAt),
		ResolvedAt:     tsOrNil(r.ResolvedAt),
	}
}
