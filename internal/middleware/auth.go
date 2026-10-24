// Package middleware holds HTTP middleware shared by all routes.
package middleware

import (
	"context"
	"net/http"
	"strings"
)

type userIDKey struct{}

// RequireAuth rejects requests without a valid "Authorization: Bearer <jwt>"
// and stores the authenticated user ID in the request context.
func RequireAuth(parse func(token string) (int64, error)) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
			id, err := parse(token)
			if !ok || err != nil {
				writeError(w, http.StatusUnauthorized, "unauthorized")
				return
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userIDKey{}, id)))
		})
	}
}

// UserID returns the authenticated user ID; only valid behind RequireAuth.
func UserID(ctx context.Context) int64 {
	id, _ := ctx.Value(userIDKey{}).(int64)
	return id
}
