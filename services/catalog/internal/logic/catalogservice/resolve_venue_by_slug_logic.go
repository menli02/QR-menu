package catalogservicelogic

import (
	"context"
	"errors"

	"github.com/menli02/QR-menu/proto/catalog/v1"
	"github.com/menli02/QR-menu/services/catalog/internal/model"
	"github.com/menli02/QR-menu/services/catalog/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ResolveVenueBySlugLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewResolveVenueBySlugLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ResolveVenueBySlugLogic {
	return &ResolveVenueBySlugLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ResolveVenueBySlug maps a public venue slug to its id, for the gateway's
// staff login (docs/TZ.md §8.1 takes a venue_slug; identity.Login takes a
// venue_id, and catalog owns the venues table).
//
// The response is deliberately four fields. This is reachable before any
// authentication — it backs the login screen — so it returns only what a
// login page needs to render, never the venue's settings. GetVenueSettings
// remains the authenticated read for those.
//
// It also does not distinguish "no such venue" from any other failure in
// its message, for the same reason a login form shouldn't: the slug is
// guessable, and a distinct response would turn this into a venue
// enumeration oracle. The gRPC code is NotFound because the gateway needs
// to map it, but the message says nothing an attacker can use.
func (l *ResolveVenueBySlugLogic) ResolveVenueBySlug(in *v1_catalogpb.ResolveVenueBySlugRequest) (*v1_catalogpb.ResolveVenueBySlugResponse, error) {
	if in.GetSlug() == "" {
		return nil, status.Error(codes.InvalidArgument, "slug is required")
	}

	venue, err := model.NewVenueModel(l.svcCtx.DB).FindBySlug(l.ctx, in.GetSlug())
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "venue not found")
		}
		l.Errorf("resolve venue by slug %q: %v", in.GetSlug(), err)
		return nil, status.Error(codes.Internal, "look up venue")
	}

	return &v1_catalogpb.ResolveVenueBySlugResponse{
		VenueId:       venue.ID,
		Name:          venue.Name,
		DefaultLocale: venue.DefaultLocale,
		Currency:      venue.Currency,
	}, nil
}
