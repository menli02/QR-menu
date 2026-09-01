package catalogservicelogic

import (
	"context"

	"github.com/menli02/QR-menu/proto/catalog/v1"
	"github.com/menli02/QR-menu/services/catalog/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ResolveTableLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewResolveTableLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ResolveTableLogic {
	return &ResolveTableLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// ResolveTable validates a QR table_code + HMAC signature and returns the
func (l *ResolveTableLogic) ResolveTable(in *v1_catalogpb.ResolveTableRequest) (*v1_catalogpb.ResolveTableResponse, error) {
	// todo: add your logic here and delete this line

	return &v1_catalogpb.ResolveTableResponse{}, nil
}
