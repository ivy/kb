package server

import (
	"net/http"
)

// authMiddleware applies authentication based on Tailscale configuration.
//
// Three modes:
//   - tsnet disabled (cfg.Enabled=false): no auth, plain TCP
//   - tsnet enabled, authz disabled (cfg.AuthZ=false): tsnet listener, no capability checks
//   - tsnet enabled, authz enabled (cfg.AuthZ=true): WhoIs lookup + capability check
func (s *Server) authMiddleware(next http.Handler) http.Handler {
	if !s.cfg.Enabled {
		return next
	}

	if !s.cfg.AuthZ {
		return next
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// TODO: WhoIs lookup + capability check
		// For now, pass through
		next.ServeHTTP(w, r)
	})
}
