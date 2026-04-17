package middleware

import (
	"net/http"

	"livepass/pkg/xmiddleware"
)

type AuthMiddleware struct {
	accessSecret   string
	internalSecret string
}

func NewAuthMiddleware(accessSecret string, internalSecret string) *AuthMiddleware {
	return &AuthMiddleware{
		accessSecret:   accessSecret,
		internalSecret: internalSecret,
	}
}

func (m *AuthMiddleware) Handle(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		xmiddleware.ClearGatewayIdentityHeaders(r.Header)

		if r.Method == http.MethodOptions {
			next(w, r)
			return
		}

		if !requiresAuth(r.URL.Path) {
			xmiddleware.ClearExternalAuthHeaders(r.Header)
			next(w, r)
			return
		}

		userID, err := xmiddleware.Authenticate(r, m.accessSecret)
		if err != nil {
			writeUnauthorized(w, r, err)
			return
		}

		xmiddleware.ClearExternalAuthHeaders(r.Header)
		if err := xmiddleware.AttachGatewayIdentityHeaders(r.Header, userID, m.internalSecret); err != nil {
			writeUnauthorized(w, r, err)
			return
		}

		next(w, r.WithContext(xmiddleware.WithUserID(r.Context(), userID)))
	}
}
