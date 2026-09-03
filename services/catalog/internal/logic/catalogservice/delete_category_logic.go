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

type DeleteCategoryLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewDeleteCategoryLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteCategoryLogic {
	return &DeleteCategoryLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// DeleteCategory is a genuine hard DELETE (see CategoryModel.Delete's
// comment) — the foreign key from menu_items blocks deleting a category
// that still has items, surfaced here as FAILED_PRECONDITION rather than
// a bare internal error.
func (l *DeleteCategoryLogic) DeleteCategory(in *v1_catalogpb.DeleteCategoryRequest) (*v1_catalogpb.DeleteCategoryResponse, error) {
	var deleted bool
	err := l.svcCtx.DB.TransactCtx(l.ctx, func(ctx context.Context, session sqlx.Session) error {
		var err error
		deleted, err = model.NewCategoryModel(session).Delete(ctx, in.GetCategoryId(), in.GetVenueId())
		if err != nil {
			return err
		}
		_, err = model.NewVenueModel(session).BumpMenuVersion(ctx, in.GetVenueId())
		return err
	})
	if err != nil {
		if pgErrorCode(err) == pgForeignKeyViolation {
			return nil, status.Error(codes.FailedPrecondition, "category still has menu items; delete or move them first")
		}
		return nil, status.Errorf(codes.Internal, "delete category: %v", err)
	}
	return &v1_catalogpb.DeleteCategoryResponse{Deleted: deleted}, nil
}
