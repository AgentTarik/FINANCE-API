package auth

import (
	"errors"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// RequireAuth verifies a Bearer JWT (HS256) and injects "user_id" into the context.
// It returns 401 on missing/invalid token; 403 on claim validation failure.
func RequireAuth() gin.HandlerFunc {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		// Fail fast at startup: misconfiguration.
		panic("JWT_SECRET is required for RequireAuth middleware")
	}
	iss := os.Getenv("JWT_ISS")
	aud := os.Getenv("JWT_AUD")

	return func(c *gin.Context) {
		// 1) Extract Bearer token
		authz := c.GetHeader("Authorization")
		if !strings.HasPrefix(strings.ToLower(authz), "bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing bearer token"})
			return
		}
		raw := strings.TrimSpace(authz[len("Bearer "):])
		if raw == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "empty bearer token"})
			return
		}

		// 2) Parse + verify signature (HS256 only) and validate registered claims
		claims := &jwt.RegisteredClaims{}
		token, err := jwt.ParseWithClaims(
			raw,
			claims,
			func(t *jwt.Token) (any, error) {
				// Enforce HS256
				if t.Method != jwt.SigningMethodHS256 {
					return nil, errors.New("unexpected signing method")
				}
				return []byte(secret), nil
			},
			jwt.WithLeeway(30*time.Second),
			jwt.WithIssuer(iss),
			jwt.WithAudience(aud),
		)
		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}

		// 3) Basic subject sanity check (expect a UUID v4 user id)
		if claims.Subject == "" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "invalid subject"})
			return
		}
		if id, err := uuid.Parse(claims.Subject); err != nil || id.Version() != 4 {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "invalid subject"})
			return
		}

		// 4) Propagate identity to handlers
		c.Set("user_id", claims.Subject)

		// Continue to the handler
		c.Next()
	}
}
