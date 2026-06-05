package controllers

import (
	"database/sql"
	"net/http"

	"github.com/gin-gonic/gin"
	"technicien-mobile/internal/repository"
	"technicien-mobile/internal/utils"
)

// GET /api/mobile/lampadaires/:id/diagnostic
// TODO(auth): Quand AUTH_ENABLED=true, limiter aux lampadaires liés aux interventions du technicien.
func HandleDiagnostic(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := utils.ParseIDParam(c, "id")
		if err != nil {
			utils.RespondError(c, http.StatusBadRequest, err.Error())
			return
		}
		result, err := repository.GetLampadaireDiagnostic(c.Request.Context(), db, id)
		if err != nil {
			utils.RespondError(c, http.StatusInternalServerError, "Erreur base de données")
			return
		}
		if result == nil {
			utils.RespondError(c, http.StatusNotFound, "Lampadaire introuvable")
			return
		}
		utils.RespondJSON(c, http.StatusOK, result)
	}
}

// GET /api/mobile/lampadaires/:id/telemetry/latest
// TODO(auth): Quand AUTH_ENABLED=true, limiter aux lampadaires liés aux interventions du technicien.
func HandleLatestTelemetry(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := utils.ParseIDParam(c, "id")
		if err != nil {
			utils.RespondError(c, http.StatusBadRequest, err.Error())
			return
		}
		t, err := repository.GetLatestTelemetryByLampadaireID(c.Request.Context(), db, id)
		if err != nil {
			utils.RespondError(c, http.StatusInternalServerError, "Erreur base de données")
			return
		}
		if t == nil {
			utils.RespondJSON(c, http.StatusOK, gin.H{"message": "Aucune télémétrie disponible", "lampadaire_id": id})
			return
		}
		utils.RespondJSON(c, http.StatusOK, gin.H{"lampadaire_id": id, "telemetry": t})
	}
}
