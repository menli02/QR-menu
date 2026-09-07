package svc

import (
	"context"
	"errors"

	v1_orderpb "github.com/menli02/QR-menu/proto/order/v1"
	"github.com/menli02/QR-menu/services/gateway/internal/middleware"
	"github.com/menli02/QR-menu/services/gateway/internal/ws"
	"github.com/menli02/QR-menu/services/order/client/orderservice"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// This file adapts the auth middlewares and the order client to the narrow
// interfaces the ws package declares.
//
// The indirection buys a one-way dependency: ws knows nothing about
// middleware, grpc or the order service, so it can be tested with plain
// fakes. These three types are the only place the two vocabularies meet.

type staffVerifier struct {
	m *middleware.StaffAuthMiddleware
}

func (v staffVerifier) Verify(ctx context.Context, token string) (ws.StaffIdentity, error) {
	claims, err := v.m.Verify(ctx, token)
	if err != nil {
		return ws.StaffIdentity{}, err
	}
	return ws.StaffIdentity{StaffID: claims.StaffID, VenueID: claims.VenueID, Role: claims.Role}, nil
}

type guestVerifier struct {
	m *middleware.GuestAuthMiddleware
}

func (v guestVerifier) Verify(ctx context.Context, token string) (ws.GuestIdentity, error) {
	claims, err := v.m.Verify(ctx, token)
	if err != nil {
		return ws.GuestIdentity{}, err
	}
	return ws.GuestIdentity{
		VenueID:        claims.VenueID,
		TableID:        claims.TableID,
		GuestSessionID: claims.GuestSessionID,
	}, nil
}

type sessionResolver struct{ order orderservice.OrderService }

// CurrentSessionID returns the open session at a table, or "" if there is
// none.
//
// "No session" is a normal state, not an error: a guest who has scanned
// the QR code but not yet ordered has no session to subscribe to. They get
// the menu channel and pick up their session channel on the reconnect
// after their first order.
func (r sessionResolver) CurrentSessionID(ctx context.Context, venueID, tableID string) (string, error) {
	session, err := r.order.GetTableSession(ctx, &v1_orderpb.GetTableSessionRequest{
		VenueId: venueID,
		TableId: tableID,
	})
	if err != nil {
		if st, ok := status.FromError(err); ok && st.Code() == codes.NotFound {
			return "", nil
		}
		return "", err
	}
	if session.GetId() == "" {
		return "", errors.New("order service returned a session with no id")
	}
	return session.GetId(), nil
}
