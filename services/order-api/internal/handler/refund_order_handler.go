// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package handler

import (
	"net/http"

	"livepass/services/order-api/internal/logic"
	"livepass/services/order-api/internal/svc"
	"livepass/services/order-api/internal/types"
	"github.com/zeromicro/go-zero/rest/httpx"
)

func RefundOrderHandler(svcCtx *svc.ServiceContext) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req types.RefundOrderReq
		if err := httpx.Parse(r, &req); err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
			return
		}

		l := logic.NewRefundOrderLogic(r.Context(), svcCtx)
		resp, err := l.RefundOrder(&req)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, err)
		} else {
			httpx.OkJsonCtx(r.Context(), w, resp)
		}
	}
}
