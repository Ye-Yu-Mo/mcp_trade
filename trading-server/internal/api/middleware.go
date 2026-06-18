package api

import (
	"net/http"
	"strings"
)

// AuthMiddleware returns a chi-compatible middleware that validates
// the Authorization header against the configured API token.
// Requests without a valid Bearer token receive 401.
func AuthMiddleware(token string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			auth := r.Header.Get("Authorization")
			if auth == "" {
				Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "missing Authorization header")
				return
			}

			parts := strings.SplitN(auth, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "invalid Authorization format, expected: Bearer <token>")
				return
			}

			if parts[1] != token {
				Error(w, http.StatusUnauthorized, "UNAUTHORIZED", "invalid token")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
