// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package admin

import (
	"context"

	"github.com/menli02/QR-menu/services/gateway/internal/svc"
	"github.com/menli02/QR-menu/services/gateway/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListTablesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListTablesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListTablesLogic {
	return &ListTablesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListTablesLogic) ListTables(req *types.ListTablesReq) (resp *types.ListTablesResp, err error) {
	// todo: add your logic here and delete this line

	return
}
