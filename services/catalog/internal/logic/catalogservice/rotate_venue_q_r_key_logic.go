package catalogservicelogic

import (
	"context"
	"errors"
	"time"

	"github.com/menli02/QR-menu/proto/catalog/v1"
	"github.com/menli02/QR-menu/services/catalog/internal/model"
	"github.com/menli02/QR-menu/services/catalog/internal/qrsig"
	"github.com/menli02/QR-menu/services/catalog/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/stores/sqlx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// defaultGracePeriodDays is FR-T4's default: old QR codes stay valid for
// 30 days after a rotation, giving a venue time to reprint.
const defaultGracePeriodDays = 30

type RotateVenueQRKeyLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRotateVenueQRKeyLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RotateVenueQRKeyLogic {
	return &RotateVenueQRKeyLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// RotateVenueQRKey issues a new HMAC key; every table printed under the
// previous key keeps verifying until the grace period elapses (FR-T4).
// Existing tables' key_version is never touched — their printed stickers
// are still signed under the old key and stay valid until it expires; a
// venue reprints a specific table under the new key by some future flow
// this proto doesn't define yet (see CreateTable's comment on the
// missing "retrieve the sig" gap — the same gap blocks reprinting too).
func (l *RotateVenueQRKeyLogic) RotateVenueQRKey(in *v1_catalogpb.RotateVenueQRKeyRequest) (*v1_catalogpb.RotateVenueQRKeyResponse, error) {
	graceDays := in.GetGracePeriodDays()
	if graceDays <= 0 {
		graceDays = defaultGracePeriodDays
	}

	keyModel := model.NewVenueQRKeyModel(l.svcCtx.DB)
	current, err := keyModel.FindCurrent(l.ctx, in.GetVenueId())
	if err != nil {
		if errors.Is(err, model.ErrNotFound) {
			return nil, status.Error(codes.FailedPrecondition, "venue has no QR signing key provisioned")
		}
		return nil, status.Errorf(codes.Internal, "look up current QR key: %v", err)
	}

	newSecret, err := qrsig.GenerateSecret()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "generate new key: %v", err)
	}

	newVersion := current.KeyVersion + 1
	expiresAt := time.Now().Add(time.Duration(graceDays) * 24 * time.Hour)

	err = l.svcCtx.DB.TransactCtx(l.ctx, func(ctx context.Context, session sqlx.Session) error {
		txKeyModel := model.NewVenueQRKeyModel(session)
		// Expire the current key BEFORE inserting the new one:
		// venue_qr_keys_current_idx allows only one current (expires_at
		// IS NULL) row per venue — see VenueQRKeyModel.Insert's comment.
		if err := txKeyModel.ExpireAt(ctx, in.GetVenueId(), current.KeyVersion, expiresAt); err != nil {
			return err
		}
		_, err := txKeyModel.Insert(ctx, in.GetVenueId(), newVersion, newSecret)
		return err
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "rotate QR key: %v", err)
	}

	return &v1_catalogpb.RotateVenueQRKeyResponse{
		NewKeyVersion:        newVersion,
		PreviousKeyVersion:   current.KeyVersion,
		PreviousKeyExpiresAt: timestamppb.New(expiresAt),
	}, nil
}
