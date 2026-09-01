// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package floor

import (
	"net/http"

	"github.com/menli02/QR-menu/services/gateway/internal/logic/floor"
	"github.com/menli02/QR-menu/services/gateway/internal/svc"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func ListFloorTablesHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		l := floor.NewListFloorTablesLogic(r.Context(), svcCtx)
		resp, err := l.ListFloorTables()
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
