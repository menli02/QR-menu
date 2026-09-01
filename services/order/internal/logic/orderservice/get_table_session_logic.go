package orderservicelogic

import (
	"context"

	"github.com/menli02/QR-menu/proto/order/v1"
	"github.com/menli02/QR-menu/services/order/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetTableSessionLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetTableSessionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetTableSessionLogic {
	return &GetTableSessionLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetTableSessionLogic) GetTableSession(in *v1_orderpb.GetTableSessionRequest) (*v1_orderpb.TableSession, error) {
	// todo: add your logic here and delete this line

	return &v1_orderpb.TableSession{}, nil
}
