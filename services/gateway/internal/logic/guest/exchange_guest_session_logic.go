// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package guest

import (
	"context"
	"strconv"
	"strings"

	"github.com/google/uuid"
	v1_catalogpb "github.com/menli02/QR-menu/proto/catalog/v1"
	v1_identitypb "github.com/menli02/QR-menu/proto/identity/v1"
	"github.com/menli02/QR-menu/services/gateway/internal/convert"
	"github.com/menli02/QR-menu/services/gateway/internal/errs"
	"github.com/menli02/QR-menu/services/gateway/internal/reqctx"
	"github.com/menli02/QR-menu/services/gateway/internal/rpcerr"
	"github.com/menli02/QR-menu/services/gateway/internal/svc"
	"github.com/menli02/QR-menu/services/gateway/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

// guestTokenTTLSeconds is FR-O1's 4h sliding session. It must match the
// order service's GuestSessionTTLSeconds, which writes the bookkeeping
// copy of this expiry onto the guest_sessions row.
const guestTokenTTLSeconds = 14400

type ExchangeGuestSessionLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewExchangeGuestSessionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ExchangeGuestSessionLogic {
	return &ExchangeGuestSessionLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// ExchangeGuestSession turns a scanned QR link into a guest JWT (FR-O1,
// FR-T2). It is the only unauthenticated guest route, so it is also the
// one that has to be most careful.
//
// The gateway mints the guest_session_id here rather than letting identity
// or order generate one. That is deliberate and the schema depends on it:
// order_db.guest_sessions.id has no DEFAULT precisely because the id must
// equal the one already inside the token, and only the component that
// sequences "make an id → put it in a token → hand it to the guest" can
// guarantee that.
//
// The table_code is signed (HMAC, per-venue key, versioned) and catalog
// verifies it. A guest can therefore not fabricate a session for a table
// they are not sitting at, which is what stops them ordering onto — or
// reading the bill of — someone else's table.
func (l *ExchangeGuestSessionLogic) ExchangeGuestSession(req *types.ExchangeGuestSessionReq) (resp *types.ExchangeGuestSessionResp, err error) {
	slug := strings.TrimSpace(req.VenueSlug)
	code := strings.TrimSpace(req.TableCode)
	if slug == "" || code == "" || req.Sig == "" {
		return nil, errs.New(errs.CodeValidationFailed, "venueSlug, tableCode and sig are required")
	}

	sig, keyVersion := splitSignature(req.Sig)

	table, err := l.svcCtx.CatalogRpc.ResolveTable(l.ctx, &v1_catalogpb.ResolveTableRequest{
		VenueSlug:  slug,
		TableCode:  code,
		Sig:        sig,
		KeyVersion: keyVersion,
	})
	if err != nil {
		converted := rpcerr.FromCatalog(err)
		if converted.Code == errs.CodeNotFound {
			// Same response as a bad signature: a valid-looking code for a
			// venue that doesn't exist must not be distinguishable from a
			// forged one, or the endpoint becomes a way to enumerate
			// venues and tables.
			return nil, invalidQRCode()
		}
		return nil, converted
	}

	switch {
	case !table.GetSigValid():
		l.Infof("rejected guest session: bad signature for venue %q table_code %q from %s",
			slug, code, reqctx.ClientIP(l.ctx))
		return nil, invalidQRCode()
	case table.GetKeyVersionExpired():
		// A real, correctly signed code whose key rotated out of its grace
		// period (FR-T4). Distinct from a forgery, and worth telling the
		// guest apart, because the fix is "ask for the new menu card"
		// rather than "you did something wrong".
		return nil, errs.New(errs.CodeValidationFailed, "this QR code has expired; please ask staff for the current one")
	case !table.GetTableIsActive():
		return nil, errs.New(errs.CodeTableInactive, "this table is not currently in service")
	}

	guestSessionID := uuid.NewString()

	pair, err := l.svcCtx.IdentityRpc.IssueGuestToken(l.ctx, &v1_identitypb.IssueGuestTokenRequest{
		VenueId:        table.GetVenueId(),
		TableId:        table.GetTableId(),
		GuestSessionId: guestSessionID,
		TtlSeconds:     guestTokenTTLSeconds,
	})
	if err != nil {
		return nil, rpcerr.From(err)
	}

	return &types.ExchangeGuestSessionResp{
		AccessToken:    pair.GetAccessToken(),
		TokenType:      "Bearer",
		ExpiresAt:      convert.Time(pair.GetAccessTokenExpiresAt()),
		VenueId:        table.GetVenueId(),
		TableId:        table.GetTableId(),
		GuestSessionId: guestSessionID,
	}, nil
}

func invalidQRCode() *errs.Error {
	return errs.New(errs.CodeValidationFailed, "this QR code is not valid")
}

// splitSignature accepts the signature either bare or as "<version>.<sig>".
//
// FR-T4 rotates the signing key and keeps the previous version valid for a
// grace period, so verification needs to know which key signed a given
// code. ResolveTableRequest carries key_version as its own field, but the
// public request body (§8.1) has only `sig` — so the version travels
// inside it. A bare signature means version 1, which is what every code
// printed before the first rotation carries.
func splitSignature(raw string) (sig string, keyVersion int32) {
	version, rest, found := strings.Cut(raw, ".")
	if !found {
		return raw, 1
	}
	n, err := strconv.Atoi(version)
	if err != nil || n <= 0 {
		// Not a version prefix after all — a signature that happens to
		// contain a dot. Treat the whole thing as the signature; catalog
		// will reject it if it really is malformed.
		return raw, 1
	}
	return rest, int32(n)
}
