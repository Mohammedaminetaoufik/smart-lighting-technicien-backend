package middleware

import (
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

type AuthClaims struct {
	Sub   string `json:"sub"`
	Name  string `json:"name"`
	Email string `json:"email"`
	Role  string `json:"role"`
	jwt.RegisteredClaims
}

func RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "token manquant"})
			c.Abort()
			return
		}
		raw := strings.TrimPrefix(header, "Bearer ")

		secret := os.Getenv("JWT_SECRET")
		if secret == "" {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "configuration serveur invalide"})
			c.Abort()
			return
		}

		token, err := jwt.ParseWithClaims(raw, &AuthClaims{}, func(t *jwt.Token) (any, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return []byte(secret), nil
		})
		if err != nil || !token.Valid {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "token invalide ou expiré"})
			c.Abort()
			return
		}

		claims, ok := token.Claims.(*AuthClaims)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "token malformé"})
			c.Abort()
			return
		}

		c.Set("user_id", claims.Sub)
		c.Set("user_name", claims.Name)
		c.Set("user_email", claims.Email)
		c.Set("user_role", claims.Role)
		c.Next()
	}
}

// OptionalAuth extracts user info from a Bearer token if present, without blocking.
func OptionalAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if !strings.HasPrefix(header, "Bearer ") {
			c.Next()
			return
		}
		raw := strings.TrimPrefix(header, "Bearer ")
		secret := os.Getenv("JWT_SECRET")
		if secret == "" {
			c.Next()
			return
		}
		token, err := jwt.ParseWithClaims(raw, &AuthClaims{}, func(t *jwt.Token) (any, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return []byte(secret), nil
		})
		if err == nil && token.Valid {
			if claims, ok := token.Claims.(*AuthClaims); ok {
				c.Set("user_id", claims.Sub)
				c.Set("user_name", claims.Name)
				c.Set("user_email", claims.Email)
				c.Set("user_role", claims.Role)
			}
		}
		c.Next()
	}
}
