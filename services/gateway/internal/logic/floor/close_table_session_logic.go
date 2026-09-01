// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package floor

import (
	"context"

	"github.com/menli02/QR-menu/services/gateway/internal/svc"
	"github.com/menli02/QR-menu/services/gateway/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type CloseTableSessionLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCloseTableSessionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CloseTableSessionLogic {
	return &CloseTableSessionLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CloseTableSessionLogic) CloseTableSession(req *types.CloseTableSessionReq) (resp *types.TableSession, err error) {
	// todo: add your logic here and delete this line

	return
}
