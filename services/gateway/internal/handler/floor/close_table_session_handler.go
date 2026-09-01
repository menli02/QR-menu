// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package floor

import (
	"net/http"

	"github.com/menli02/QR-menu/services/gateway/internal/logic/floor"
	"github.com/menli02/QR-menu/services/gateway/internal/svc"
	"github.com/menli02/QR-menu/services/gateway/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func CloseTableSessionHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.CloseTableSessionReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := floor.NewCloseTableSessionLogic(r.Context(), svcCtx)
		resp, err := l.CloseTableSession(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
