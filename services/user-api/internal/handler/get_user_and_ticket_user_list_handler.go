// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package handler

import (
	"net/http"

	"livepass/services/user-api/internal/logic"
	"livepass/services/user-api/internal/svc"
	"livepass/services/user-api/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func GetUserAndTicketUserListHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.GetUserAndTicketUserListReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := logic.NewGetUserAndTicketUserListLogic(r.Context(), svcCtx)
		resp, err := l.GetUserAndTicketUserList(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
