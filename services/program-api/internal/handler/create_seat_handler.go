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

func CreateSeatHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.SeatAddReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := logic.NewCreateSeatLogic(r.Context(), svcCtx)
		resp, err := l.CreateSeat(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
