package middleware

import (
	"net/http"
	"strings"

	"github.com/zeromicro/go-zero/rest/httpx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func requiresAuth(path string) bool {
	return strings.HasPrefix(path, "/order/")
}

func writeUnauthorized(w http.ResponseWriter, r *http.Request, err error) {
	httpx.ErrorCtx(r.Context(), w, status.Error(codes.Unauthenticated, err.Error()))
}
