// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package guest

import (
	"net/http"

	"github.com/menli02/QR-menu/services/gateway/internal/logic/guest"
	"github.com/menli02/QR-menu/services/gateway/internal/svc"
	"github.com/menli02/QR-menu/services/gateway/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func ExchangeGuestSessionHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.ExchangeGuestSessionReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := guest.NewExchangeGuestSessionLogic(r.Context(), svcCtx)
		resp, err := l.ExchangeGuestSession(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
