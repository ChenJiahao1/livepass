package middleware

import (
	"net/http"
	"strings"

	"github.com/zeromicro/go-zero/rest/httpx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const userIDHeader = "X-User-Id"

func requiresAuth(path string) bool {
	return strings.HasPrefix(path, "/order/") ||
		strings.HasPrefix(path, "/pay/") ||
		strings.HasPrefix(path, "/agent/")
}

func writeUnauthorized(w http.ResponseWriter, r *http.Request, err error) {
	httpx.ErrorCtx(r.Context(), w, status.Error(codes.Unauthenticated, err.Error()))
}
