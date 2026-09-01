// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package admin

import (
	"net/http"

	"github.com/menli02/QR-menu/services/gateway/internal/logic/admin"
	"github.com/menli02/QR-menu/services/gateway/internal/svc"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func RotateVenueQrKeyHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l := admin.NewRotateVenueQrKeyLogic(r.Context(), svcCtx)
		resp, err := l.RotateVenueQrKey()
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
