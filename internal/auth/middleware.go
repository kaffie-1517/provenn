package auth

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/kaffie-1517/provenn/internal/db"
	"golang.org/x/crypto/bcrypt"
)

// JWTMiddleware validates the Authorization: Bearer <token> header, parses the
// JWT, and injects the claims into the request context via ContextWithClaims.
func JWTMiddleware(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if header == "" || !strings.HasPrefix(header, "Bearer ") {
				http.Error(w, `{"error":"missing or invalid Authorization header"}`, http.StatusUnauthorized)
				return
			}
			tokenStr := strings.TrimPrefix(header, "Bearer ")

			claims, err := ParseJWT(secret, tokenStr)
			if err != nil {
				http.Error(w, `{"error":"invalid or expired token"}`, http.StatusUnauthorized)
				return
			}

			ctx := ContextWithClaims(r.Context(), claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// APIKeyMiddleware validates the X-Partner-Key header by iterating stored
// partner bcrypt hashes and injects the matched partner_id into context.
func APIKeyMiddleware(store *db.Store) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := r.Header.Get("X-Partner-Key")
			if key == "" {
				http.Error(w, `{"error":"missing X-Partner-Key header"}`, http.StatusUnauthorized)
				return
			}

			partners, err := store.ListPartners(r.Context())
			if err != nil {
				slog.Error("list partners for API key auth", "error", err)
				http.Error(w, `{"error":"internal server error"}`, http.StatusInternalServerError)
				return
			}

			for _, p := range partners {
				if bcrypt.CompareHashAndPassword([]byte(p.APIKeyHash), []byte(key)) == nil {
					ctx := ContextWithPartnerID(r.Context(), p.ID)
					next.ServeHTTP(w, r.WithContext(ctx))
					return
				}
			}

			http.Error(w, `{"error":"invalid API key"}`, http.StatusUnauthorized)
		})
	}
}

// RequireRole returns a middleware that rejects requests unless the JWT
// claim's role is in the allowed set. This is the reusable role-check
// mechanism — never use inline role checks in handlers.
//
// Usage: r.With(auth.RequireRole("employee", "company_admin")).Get("/...", handler)
func RequireRole(roles ...string) func(http.Handler) http.Handler {
	allowed := make(map[string]bool, len(roles))
	for _, role := range roles {
		allowed[role] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, err := ClaimsFromContext(r.Context())
			if err != nil {
				http.Error(w, `{"error":"authentication required"}`, http.StatusUnauthorized)
				return
			}
			if !allowed[claims.Role] {
				http.Error(w, `{"error":"forbidden: insufficient role"}`, http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
