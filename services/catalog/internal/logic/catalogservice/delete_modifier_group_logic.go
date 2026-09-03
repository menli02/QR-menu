package catalogservicelogic

import (
	"context"

	"github.com/menli02/QR-menu/proto/catalog/v1"
	"github.com/menli02/QR-menu/services/catalog/internal/model"
	"github.com/menli02/QR-menu/services/catalog/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type DeleteModifierGroupLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewDeleteModifierGroupLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteModifierGroupLogic {
	return &DeleteModifierGroupLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// DeleteModifierGroup is a genuine hard DELETE — see ModifierGroupModel.Delete's
// comment: options cascade, and orders snapshot names/prices with no FK
// back to catalog either way.
func (l *DeleteModifierGroupLogic) DeleteModifierGroup(in *v1_catalogpb.DeleteModifierGroupRequest) (*v1_catalogpb.DeleteModifierGroupResponse, error) {
	var deleted bool
	err := l.svcCtx.DB.TransactCtx(l.ctx, func(ctx context.Context, session sqlx.Session) error {
		var err error
		deleted, err = model.NewModifierGroupModel(session).Delete(ctx, in.GetGroupId(), in.GetItemId())
		if err != nil {
			return err
		}
		_, err = model.NewVenueModel(session).BumpMenuVersion(ctx, in.GetVenueId())
		return err
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "delete modifier group: %v", err)
	}
	return &v1_catalogpb.DeleteModifierGroupResponse{Deleted: deleted}, nil
}
