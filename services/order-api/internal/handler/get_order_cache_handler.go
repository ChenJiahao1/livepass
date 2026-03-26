// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package handler

import (
	"net/http"

	"damai-go/services/order-api/internal/logic"
	"damai-go/services/order-api/internal/svc"
	"damai-go/services/order-api/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func GetOrderCacheHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.OrderCacheReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := logic.NewGetOrderCacheLogic(r.Context(), svcCtx)
		resp, err := l.GetOrderCache(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
