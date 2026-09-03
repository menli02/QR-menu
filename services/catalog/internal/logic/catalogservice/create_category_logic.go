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

type CreateCategoryLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateCategoryLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateCategoryLogic {
	return &CreateCategoryLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// CreateCategory bumps venues.menu_version in the same transaction as the
// insert (FR-C6): a category is guest-visible content, so adding one
// changes what GetMenu/ResolveOrderItems callers should treat as stale.
func (l *CreateCategoryLogic) CreateCategory(in *v1_catalogpb.CreateCategoryRequest) (*v1_catalogpb.Category, error) {
	name := in.GetName().GetTranslations()
	if len(name) == 0 {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}

	var created *model.Category
	err := l.svcCtx.DB.TransactCtx(l.ctx, func(ctx context.Context, session sqlx.Session) error {
		var err error
		created, err = model.NewCategoryModel(session).Insert(ctx, in.GetVenueId(), name, in.GetSortOrder(), in.GetIsVisible(), in.GetImageUrl())
		if err != nil {
			return err
		}
		_, err = model.NewVenueModel(session).BumpMenuVersion(ctx, in.GetVenueId())
		return err
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "create category: %v", err)
	}
	return categoryToProto(created), nil
}
