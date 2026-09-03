package catalogservicelogic

import (
	"context"
	"errors"

	"github.com/menli02/QR-menu/proto/catalog/v1"
	"github.com/menli02/QR-menu/services/catalog/internal/model"
	"github.com/menli02/QR-menu/services/catalog/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type UpdateCategoryLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateCategoryLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateCategoryLogic {
	return &UpdateCategoryLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *UpdateCategoryLogic) UpdateCategory(in *v1_catalogpb.UpdateCategoryRequest) (*v1_catalogpb.Category, error) {
	c := in.GetCategory()
	if c == nil || len(c.GetName().GetTranslations()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "category with a name is required")
	}

	var updated *model.Category
	err := l.svcCtx.DB.TransactCtx(l.ctx, func(ctx context.Context, session sqlx.Session) error {
		var err error
		updated, err = model.NewCategoryModel(session).Update(ctx, c.GetId(), c.GetVenueId(),
			c.GetName().GetTranslations(), c.GetSortOrder(), c.GetIsVisible(), c.GetImageUrl())
		if err != nil {
			return err
		}
		_, err = model.NewVenueModel(session).BumpMenuVersion(ctx, c.GetVenueId())
		return err
	})
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "category not found")
		}
		return nil, status.Errorf(codes.Internal, "update category: %v", err)
	}
	return categoryToProto(updated), nil
}
