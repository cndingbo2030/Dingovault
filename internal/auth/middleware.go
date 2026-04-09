package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/dingbo/dingovault/internal/tenant"
)

type ctxKey int

const claimsKey ctxKey = 1

// ClaimsFromContext returns JWT claims set by BearerAuthMiddleware, if any.
func ClaimsFromContext(ctx context.Context) (*Claims, bool) {
	c, ok := ctx.Value(claimsKey).(*Claims)
	return c, ok
}

// BearerAuthMiddleware validates Authorization: Bearer <jwt>, sets tenant user id and claims on the request context.
func BearerAuthMiddleware(j *JWT, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw := r.Header.Get("Authorization")
		if raw == "" {
			http.Error(w, `{"error":"missing Authorization header"}`, http.StatusUnauthorized)
			return
		}
		const p = "Bearer "
		if !strings.HasPrefix(raw, p) {
			http.Error(w, `{"error":"invalid Authorization scheme"}`, http.StatusUnauthorized)
			return
		}
		tok := strings.TrimSpace(strings.TrimPrefix(raw, p))
		claims, err := j.ParseAccessToken(tok)
		if err != nil {
			http.Error(w, `{"error":"invalid or expired token"}`, http.StatusUnauthorized)
			return
		}
		ctx := context.WithValue(r.Context(), claimsKey, claims)
		ctx = tenant.WithUserID(ctx, strings.TrimSpace(claims.Subject))
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
