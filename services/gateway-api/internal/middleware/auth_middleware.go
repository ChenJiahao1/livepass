package middleware

import (
	"net/http"

	"livepass/pkg/xmiddleware"
)

type AuthMiddleware struct {
	accessSecret   string
	internalSecret string
	perfCfg        PerfAuthConfig
}

type PerfAuthConfig struct {
	Enabled      bool
	HeaderName   string
	HeaderSecret string
	UserIDHeader string
	AllowedPaths map[string]struct{}
}

func NewAuthMiddleware(accessSecret string, internalSecret string, perfCfg PerfAuthConfig) *AuthMiddleware {
	return &AuthMiddleware{
		accessSecret:   accessSecret,
		internalSecret: internalSecret,
		perfCfg:        perfCfg,
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

		userID, err := m.authenticate(r)
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

func (m *AuthMiddleware) authenticate(r *http.Request) (int64, error) {
	if requiresPerfAuth(r.URL.Path, m.perfCfg.AllowedPaths) {
		if userID, err := authenticatePerfRequest(r, m.perfCfg); err == nil {
			return userID, nil
		}
	}

	return xmiddleware.Authenticate(r, m.accessSecret)
}
