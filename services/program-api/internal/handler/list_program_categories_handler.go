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

func ListProgramCategoriesHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.EmptyReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := logic.NewListProgramCategoriesLogic(r.Context(), svcCtx)
		resp, err := l.ListProgramCategories(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
