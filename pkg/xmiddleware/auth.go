package xmiddleware

import (
	"context"
	"net/http"
	"strings"

	"damai-go/pkg/xerr"
	"damai-go/pkg/xjwt"
)

type contextKey string

const (
	defaultChannelCodeHeader            = "X-Channel-Code"
	userIDContextKey         contextKey = "userId"
)

func Authenticate(r *http.Request, channelHeader string, channelMap map[string]string) (int64, error) {
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

	headerName := strings.TrimSpace(channelHeader)
	if headerName == "" {
		headerName = defaultChannelCodeHeader
	}

	channelCode := strings.TrimSpace(r.Header.Get(headerName))
	if channelCode == "" {
		return 0, xerr.ErrChannelNotFound
	}

	secret := channelMap[channelCode]
	if secret == "" {
		return 0, xerr.ErrChannelNotFound
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
