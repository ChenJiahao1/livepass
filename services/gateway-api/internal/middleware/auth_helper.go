package middleware

import (
	"net/http"
	"strconv"
	"strings"

	"livepass/pkg/xerr"
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

func requiresPerfAuth(path string, allowedPaths map[string]struct{}) bool {
	if len(allowedPaths) == 0 {
		return false
	}

	_, ok := allowedPaths[path]
	return ok
}

func authenticatePerfRequest(r *http.Request, perfCfg PerfAuthConfig) (int64, error) {
	if r == nil || !perfCfg.Enabled {
		return 0, xerr.ErrUnauthorized
	}

	headerName := strings.TrimSpace(perfCfg.HeaderName)
	headerSecret := strings.TrimSpace(perfCfg.HeaderSecret)
	userIDHeader := strings.TrimSpace(perfCfg.UserIDHeader)
	if headerName == "" || headerSecret == "" || userIDHeader == "" {
		return 0, xerr.ErrUnauthorized
	}

	if strings.TrimSpace(r.Header.Get(headerName)) != headerSecret {
		return 0, xerr.ErrUnauthorized
	}

	userID, err := strconv.ParseInt(strings.TrimSpace(r.Header.Get(userIDHeader)), 10, 64)
	if err != nil || userID <= 0 {
		return 0, xerr.ErrUnauthorized
	}

	return userID, nil
}
