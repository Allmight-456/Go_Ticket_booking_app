package middleware

import (
	"context"
	"net/http"
	"strings"

	"github.com/Allmight-456/ticketflow/internal/domain"
	"github.com/Allmight-456/ticketflow/internal/service"
)

type contextKey string

const claimsKey contextKey = "userClaims"

// Authenticate parses the Bearer token from Authorization header, validates it,
// and injects the UserClaims into the request context.
func Authenticate(authSvc *service.AuthService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if !strings.HasPrefix(header, "Bearer ") {
				http.Error(w, `{"error":"missing or invalid authorization header"}`, http.StatusUnauthorized)
				return
			}

			token := strings.TrimPrefix(header, "Bearer ")
			claims, err := authSvc.ValidateToken(token)
			if err != nil {
				http.Error(w, `{"error":"invalid or expired token"}`, http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), claimsKey, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetClaims retrieves the UserClaims injected by the Authenticate middleware.
func GetClaims(ctx context.Context) (*domain.UserClaims, bool) {
	claims, ok := ctx.Value(claimsKey).(*domain.UserClaims)
	return claims, ok
}
