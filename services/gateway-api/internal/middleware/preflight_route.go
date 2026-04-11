package middleware

import (
	"net/http"

	"github.com/zeromicro/go-zero/gateway"
	"github.com/zeromicro/go-zero/rest"
)

func RegisterPreflightRoutes(server *gateway.Server, upstreams []gateway.Upstream) {
	registeredPaths := make(map[string]struct{})

	for _, upstream := range upstreams {
		for _, mapping := range upstream.Mappings {
			if _, ok := registeredPaths[mapping.Path]; ok {
				continue
			}

			registeredPaths[mapping.Path] = struct{}{}
			server.AddRoute(rest.Route{
				Method: http.MethodOptions,
				Path:   mapping.Path,
				Handler: func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusNoContent)
				},
			})
		}
	}
}
