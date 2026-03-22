// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package middleware

import (
	"errors"
	"net/http"

	"damai-go/pkg/xerr"
	"damai-go/pkg/xmiddleware"

	"github.com/zeromicro/go-zero/rest/httpx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type AuthMiddleware struct {
	channelHeader string
	channelMap    map[string]string
}

func NewAuthMiddleware(channelHeader string, channelMap map[string]string) *AuthMiddleware {
	return &AuthMiddleware{
		channelHeader: channelHeader,
		channelMap:    channelMap,
	}
}

func (m *AuthMiddleware) Handle(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, err := xmiddleware.Authenticate(r, m.channelHeader, m.channelMap)
		if err != nil {
			code := codes.Unauthenticated
			if errors.Is(err, xerr.ErrChannelNotFound) {
				code = codes.Unauthenticated
			}
			httpx.ErrorCtx(r.Context(), w, status.Error(code, err.Error()))
			return
		}

		next(w, r.WithContext(xmiddleware.WithUserID(r.Context(), userID)))
	}
}
