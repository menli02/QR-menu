package catalogservicelogic

import (
	"context"

	"github.com/menli02/QR-menu/proto/catalog/v1"
	"github.com/menli02/QR-menu/services/catalog/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
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

func (l *DeleteModifierGroupLogic) DeleteModifierGroup(in *v1_catalogpb.DeleteModifierGroupRequest) (*v1_catalogpb.DeleteModifierGroupResponse, error) {
	// todo: add your logic here and delete this line

	return &v1_catalogpb.DeleteModifierGroupResponse{}, nil
}
