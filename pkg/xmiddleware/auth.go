package xmiddleware

import (
	"context"
	"net/http"
	"strings"

	"livepass/pkg/xerr"
	"livepass/pkg/xjwt"
)

type contextKey string

const (
	userIDContextKey contextKey = "userId"
)

func Authenticate(r *http.Request, secret string) (int64, error) {
	if r == nil {
		return 0, xerr.ErrUnauthorized
	}

	authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return 0, xerr.ErrUnauthorized
	}

	token := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
	if token == "" {
		return 0, xerr.ErrUnauthorized
	}

	secret = strings.TrimSpace(secret)
	if secret == "" {
		return 0, xerr.ErrUnauthorized
	}

	claims, err := xjwt.ParseToken(token, secret)
	if err != nil || claims == nil || claims.UserID <= 0 {
		return 0, xerr.ErrUnauthorized
	}

	return claims.UserID, nil
}

func WithUserID(ctx context.Context, userID int64) context.Context {
	return context.WithValue(ctx, userIDContextKey, userID)
}

func UserIDFromContext(ctx context.Context) (int64, bool) {
	if ctx == nil {
		return 0, false
	}

	value, ok := ctx.Value(userIDContextKey).(int64)
	if !ok || value <= 0 {
		return 0, false
	}

	return value, true
}
