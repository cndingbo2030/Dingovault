package server

import (
	"net/http"
	"strings"
)

// CORSMiddleware restricts browser cross-origin access to the API.
// allowedOriginsCSV is a comma-separated list of exact origins (e.g. https://app.example.com,http://localhost:5173).
// If empty, the handler is returned unchanged (no CORS headers — same-origin / non-browser clients only).
func CORSMiddleware(allowedOriginsCSV string, next http.Handler) http.Handler {
	allowed := parseOriginList(allowedOriginsCSV)
	if len(allowed) == 0 {
		return next
	}
	allowedSet := make(map[string]struct{}, len(allowed))
	for _, o := range allowed {
		if o != "" {
			allowedSet[o] = struct{}{}
		}
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := strings.TrimSpace(r.Header.Get("Origin"))
		if r.Method == http.MethodOptions {
			if origin != "" {
				if _, ok := allowedSet[origin]; ok {
					w.Header().Set("Access-Control-Allow-Origin", origin)
					w.Header().Set("Vary", "Origin")
					w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
					w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
					w.Header().Set("Access-Control-Max-Age", "86400")
					w.WriteHeader(http.StatusNoContent)
					return
				}
				w.WriteHeader(http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
			return
		}
		if origin != "" {
			if _, ok := allowedSet[origin]; ok {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Vary", "Origin")
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}
		}
		next.ServeHTTP(w, r)
	})
}

func parseOriginList(csv string) []string {
	if strings.TrimSpace(csv) == "" {
		return nil
	}
	parts := strings.Split(csv, ",")
	var out []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
