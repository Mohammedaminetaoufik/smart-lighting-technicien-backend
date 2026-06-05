package middleware

import (
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

// RequireAuth is a placeholder middleware.
// TODO(auth): Quand AUTH_ENABLED=true, implémenter la vérification JWT :
//  1. Extraire le token depuis le header "Authorization: Bearer <token>"
//  2. Valider la signature avec JWT_SECRET
//  3. Extraire technician_id, user_name, user_role depuis les claims
//  4. Injecter dans le contexte Gin : c.Set("technician_id", claims.Sub)
//  5. Retourner 401 si token absent ou invalide
//  6. Retourner 403 si le rôle n'est pas technicien
//
// Pour l'instant (AUTH_ENABLED=false) : route complètement ouverte.
func RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		if os.Getenv("AUTH_ENABLED") == "true" {
			// TODO(auth): Insérer ici la logique JWT
			c.JSON(http.StatusNotImplemented, gin.H{
				"error":   "Auth not implemented",
				"message": "Set AUTH_ENABLED=false pour le mode test",
			})
			c.Abort()
			return
		}
		// Mode test : pas d'authentification
		c.Next()
	}
}

// OptionalAuth extrait les informations du token si présent, sans bloquer.
// TODO(auth): Quand AUTH_ENABLED=true, parser le JWT et injecter les claims.
func OptionalAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
	}
}
