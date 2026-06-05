package controllers

import (
	"database/sql"
	"net/http"

	"github.com/gin-gonic/gin"
	"technicien-mobile/internal/repository"
	"technicien-mobile/internal/utils"
)

// GET /api/mobile/me/dashboard
// TODO(auth): Quand AUTH_ENABLED=true, extraire technicianID depuis le JWT.
func HandleDashboard(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		techID, _ := utils.GetTestTechnicianID(c)
		stats, err := repository.GetDashboardStats(c.Request.Context(), db, techID)
		if err != nil {
			utils.RespondError(c, http.StatusInternalServerError, "Erreur base de données")
			return
		}
		utils.RespondJSON(c, http.StatusOK, stats)
	}
}
