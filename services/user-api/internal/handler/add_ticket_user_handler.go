// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package handler

import (
	"net/http"

	"damai-go/services/user-api/internal/logic"
	"damai-go/services/user-api/internal/svc"
	"damai-go/services/user-api/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func AddTicketUserHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.AddTicketUserReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := logic.NewAddTicketUserLogic(r.Context(), svcCtx)
		resp, err := l.AddTicketUser(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
