// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package handler

import (
	"net/http"

	"damai-go/services/program-api/internal/logic"
	"damai-go/services/program-api/internal/svc"
	"damai-go/services/program-api/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func GetProgramDetailHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.GetProgramDetailReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := logic.NewGetProgramDetailLogic(r.Context(), svcCtx)
		resp, err := l.GetProgramDetail(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
