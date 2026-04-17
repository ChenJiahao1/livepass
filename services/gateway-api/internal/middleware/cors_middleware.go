package middleware

import (
	"net/http"
	"strconv"
	"strings"

	"livepass/services/gateway-api/internal/config"
)

var (
	defaultCorsAllowHeaders = []string{
		"Content-Type",
		"Origin",
		"X-CSRF-Token",
		"Authorization",
		"AccessToken",
		"Token",
		"Range",
	}
	defaultCorsExposeHeaders = []string{
		"Content-Length",
		"Access-Control-Allow-Origin",
		"Access-Control-Allow-Headers",
	}
	defaultCorsAllowMethods = []string{
		http.MethodGet,
		http.MethodHead,
		http.MethodPost,
		http.MethodPatch,
		http.MethodPut,
		http.MethodDelete,
		http.MethodOptions,
	}
)

type CorsMiddleware struct {
	allowOrigins     []string
	allowHeaders     string
	exposeHeaders    string
	allowMethods     string
	allowCredentials bool
	maxAge           string
}

func NewCorsMiddleware(cfg config.CorsConfig) *CorsMiddleware {
	allowHeaders := cfg.AllowHeaders
	if len(allowHeaders) == 0 {
		allowHeaders = defaultCorsAllowHeaders
	}

	exposeHeaders := cfg.ExposeHeaders
	if len(exposeHeaders) == 0 {
		exposeHeaders = defaultCorsExposeHeaders
	}

	allowMethods := cfg.AllowMethods
	if len(allowMethods) == 0 {
		allowMethods = defaultCorsAllowMethods
	}

	maxAge := cfg.MaxAge
	if maxAge <= 0 {
		maxAge = 86400
	}

	return &CorsMiddleware{
		allowOrigins:     cfg.AllowOrigins,
		allowHeaders:     strings.Join(allowHeaders, ", "),
		exposeHeaders:    strings.Join(exposeHeaders, ", "),
		allowMethods:     strings.Join(allowMethods, ", "),
		allowCredentials: cfg.AllowCredentials,
		maxAge:           strconv.Itoa(maxAge),
	}
}

func (m *CorsMiddleware) Handle(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		origin, ok := m.allowedOrigin(r.Header.Get("Origin"))
		if ok {
			m.writeHeaders(w.Header(), r, origin)
		}

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next(w, r)
	}
}

func (m *CorsMiddleware) allowedOrigin(origin string) (string, bool) {
	if origin == "" {
		return "", false
	}

	if len(m.allowOrigins) == 0 {
		return origin, true
	}

	for _, allowOrigin := range m.allowOrigins {
		if strings.EqualFold(allowOrigin, origin) {
			return origin, true
		}
	}

	return "", false
}

func (m *CorsMiddleware) writeHeaders(header http.Header, r *http.Request, origin string) {
	addVaryHeader(header, "Origin")
	if r.Method == http.MethodOptions {
		addVaryHeader(header, "Access-Control-Request-Method")
		addVaryHeader(header, "Access-Control-Request-Headers")
	}

	header.Set("Access-Control-Allow-Origin", origin)
	header.Set("Access-Control-Allow-Methods", m.allowMethods)
	header.Set("Access-Control-Allow-Headers", m.allowHeaders)
	header.Set("Access-Control-Expose-Headers", m.exposeHeaders)
	header.Set("Access-Control-Max-Age", m.maxAge)
	if m.allowCredentials {
		header.Set("Access-Control-Allow-Credentials", "true")
	}
}

func addVaryHeader(header http.Header, value string) {
	for _, current := range header.Values("Vary") {
		for _, item := range strings.Split(current, ",") {
			if strings.EqualFold(strings.TrimSpace(item), value) {
				return
			}
		}
	}

	header.Add("Vary", value)
}
