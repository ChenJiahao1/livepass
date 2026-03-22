package middleware

import (
	"net/http"
	"strconv"

	"damai-go/pkg/xmiddleware"
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
		if !requiresAuth(r.URL.Path) {
			next(w, r)
			return
		}

		userID, err := xmiddleware.Authenticate(r, m.channelHeader, m.channelMap)
		if err != nil {
			writeUnauthorized(w, r, err)
			return
		}

		r.Header.Set(userIDHeader, strconv.FormatInt(userID, 10))
		next(w, r.WithContext(xmiddleware.WithUserID(r.Context(), userID)))
	}
}
