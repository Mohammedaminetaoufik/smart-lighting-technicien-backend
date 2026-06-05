package controllers

import (
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"technicien-mobile/internal/utils"
)

// GET /api/mobile/health
func HandleHealth() gin.HandlerFunc {
	return func(c *gin.Context) {
		utils.RespondJSON(c, http.StatusOK, gin.H{
			"status":       "ok",
			"service":      "smart-lighting-technicien-backend",
			"auth_enabled": utils.AuthEnabled(),
			"server_time":  time.Now().UTC().Format(time.RFC3339),
			"port":         os.Getenv("PORT"),
		})
	}
}

// GET /api/mobile/test-context
// TODO(auth): Quand AUTH_ENABLED=true, supprimer cet endpoint ou le protéger.
func HandleTestContext() gin.HandlerFunc {
	return func(c *gin.Context) {
		techID, source := utils.GetTestTechnicianID(c)
		utils.RespondJSON(c, http.StatusOK, gin.H{
			"auth_enabled":   utils.AuthEnabled(),
			"technician_id":  techID,
			"source":         source,
			"server_time":    time.Now().UTC().Format(time.RFC3339),
			"note":           "En mode test — AUTH_ENABLED=false. Définir technician_id via ?technician_id=X, header X-Test-Technician-Id, ou DEFAULT_TECHNICIAN_ID dans .env",
		})
	}
}
