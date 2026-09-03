package catalogservicelogic

import (
	"context"

	"github.com/menli02/QR-menu/proto/catalog/v1"
	"github.com/menli02/QR-menu/services/catalog/internal/model"
	"github.com/menli02/QR-menu/services/catalog/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ListCategoriesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListCategoriesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListCategoriesLogic {
	return &ListCategoriesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListCategoriesLogic) ListCategories(in *v1_catalogpb.ListCategoriesRequest) (*v1_catalogpb.ListCategoriesResponse, error) {
	categories, err := model.NewCategoryModel(l.svcCtx.DB).List(l.ctx, in.GetVenueId())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list categories: %v", err)
	}
	resp := &v1_catalogpb.ListCategoriesResponse{Categories: make([]*v1_catalogpb.Category, len(categories))}
	for i := range categories {
		resp.Categories[i] = categoryToProto(&categories[i])
	}
	return resp, nil
}
