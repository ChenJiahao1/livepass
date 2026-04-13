// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package middleware

import (
	"net/http"
	"time"

	"damai-go/pkg/xmiddleware"

	"github.com/zeromicro/go-zero/rest/httpx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type AuthMiddleware struct {
	secret       string
	maxClockSkew time.Duration
}

func NewAuthMiddleware(secret string, maxClockSkew time.Duration) *AuthMiddleware {
	return &AuthMiddleware{
		secret:       secret,
		maxClockSkew: maxClockSkew,
	}
}

func (m *AuthMiddleware) Handle(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, err := xmiddleware.AuthenticateGatewayIdentity(r, m.secret, m.maxClockSkew)
		if err != nil {
			httpx.ErrorCtx(r.Context(), w, status.Error(codes.Unauthenticated, err.Error()))
			return
		}

		next(w, r.WithContext(xmiddleware.WithUserID(r.Context(), userID)))
	}
}
