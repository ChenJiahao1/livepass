// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package handler

import (
	"net/http"

	"livepass/services/program-api/internal/logic"
	"livepass/services/program-api/internal/svc"
	"livepass/services/program-api/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func GetTicketCategoryDetailHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.TicketCategoryReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := logic.NewGetTicketCategoryDetailLogic(r.Context(), svcCtx)
		resp, err := l.GetTicketCategoryDetail(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
