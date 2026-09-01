package catalogservicelogic

import (
	"context"

	"github.com/menli02/QR-menu/proto/catalog/v1"
	"github.com/menli02/QR-menu/services/catalog/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateModifierGroupLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateModifierGroupLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateModifierGroupLogic {
	return &CreateModifierGroupLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CreateModifierGroupLogic) CreateModifierGroup(in *v1_catalogpb.CreateModifierGroupRequest) (*v1_catalogpb.ModifierGroup, error) {
	// todo: add your logic here and delete this line

	return &v1_catalogpb.ModifierGroup{}, nil
}
