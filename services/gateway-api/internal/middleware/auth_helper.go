package middleware

import (
	"net/http"
	"strings"

	"github.com/zeromicro/go-zero/rest/httpx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var publicUserPaths = map[string]struct{}{
	"/user/register": {},
	"/user/exist":    {},
	"/user/login":    {},
}

func requiresAuth(path string) bool {
	if strings.HasPrefix(path, "/user/") {
		_, ok := publicUserPaths[path]
		return !ok
	}

	return strings.HasPrefix(path, "/ticket/user/") ||
		strings.HasPrefix(path, "/order/") ||
		strings.HasPrefix(path, "/pay/") ||
		strings.HasPrefix(path, "/agent/")
}

func writeUnauthorized(w http.ResponseWriter, r *http.Request, err error) {
	httpx.ErrorCtx(r.Context(), w, status.Error(codes.Unauthenticated, err.Error()))
}
