package tenant

import "context"

// LocalUserID is the default tenant for single-user desktop / local SQLite.
const LocalUserID = "local"

type ctxKey struct{}

var userIDKey ctxKey

// WithUserID attaches a tenant user id to ctx (used by HTTP middleware and tests).
func WithUserID(ctx context.Context, userID string) context.Context {
	if userID == "" {
		userID = LocalUserID
	}
	return context.WithValue(ctx, userIDKey, userID)
}

// UserID returns the tenant id, or LocalUserID if unset.
func UserID(ctx context.Context) string {
	if ctx == nil {
		return LocalUserID
	}
	v, _ := ctx.Value(userIDKey).(string)
	if v == "" {
		return LocalUserID
	}
	return v
}
