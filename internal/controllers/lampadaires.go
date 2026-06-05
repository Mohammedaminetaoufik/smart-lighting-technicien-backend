package controllers

import (
	"database/sql"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"technicien-mobile/internal/models"
	"technicien-mobile/internal/repository"
	"technicien-mobile/internal/services"
	"technicien-mobile/internal/utils"
)

// GET /api/mobile/lampadaires
func HandleListLampadaires(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		limit, _ := strconv.Atoi(c.DefaultQuery("limit", "200"))
		lamps, err := repository.ListLampadairesForTech(c.Request.Context(), db,
			c.Query("zone"), c.Query("etat"), c.Query("search"), limit)
		if err != nil {
			utils.RespondError(c, http.StatusInternalServerError, "Erreur base de données")
			return
		}
		if lamps == nil { lamps = []models.MapLampadaire{} }
		utils.RespondJSON(c, http.StatusOK, gin.H{"lampadaires": lamps, "count": len(lamps)})
	}
}

// GET /api/mobile/lampadaires/:id/details
func HandleLampadaireDetails(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := utils.ParseIDParam(c, "id")
		if err != nil {
			utils.RespondError(c, http.StatusBadRequest, err.Error())
			return
		}
		techID, _ := utils.GetTestTechnicianID(c)
		d, err := repository.GetLampadaireDetail(c.Request.Context(), db, id, techID)
		if err != nil {
			utils.RespondError(c, http.StatusInternalServerError, "Erreur base de données")
			return
		}
		if d == nil {
			utils.RespondError(c, http.StatusNotFound, "Lampadaire introuvable")
			return
		}
		utils.RespondJSON(c, http.StatusOK, d)
	}
}

// GET /api/mobile/lampadaires/:id/alerts
func HandleLampadaireAlerts(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := utils.ParseIDParam(c, "id")
		if err != nil {
			utils.RespondError(c, http.StatusBadRequest, err.Error())
			return
		}
		alerts, _ := repository.GetLampadaireOpenAlerts(c.Request.Context(), db, id)
		if alerts == nil { alerts = []models.DiagnosticAlert{} }
		utils.RespondJSON(c, http.StatusOK, gin.H{"lampadaire_id": id, "alerts": alerts, "count": len(alerts)})
	}
}

// GET /api/mobile/lampadaires/:id/workorders
func HandleLampadaireWorkOrders(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := utils.ParseIDParam(c, "id")
		if err != nil {
			utils.RespondError(c, http.StatusBadRequest, err.Error())
			return
		}
		wos, _ := repository.GetLampadaireWorkOrders(c.Request.Context(), db, id)
		if wos == nil { wos = []models.MobileWorkOrder{} }
		utils.RespondJSON(c, http.StatusOK, gin.H{"lampadaire_id": id, "work_orders": wos, "count": len(wos)})
	}
}

// POST /api/mobile/lampadaires/:id/field-note
func HandleLampadaireFieldNote(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := utils.ParseIDParam(c, "id")
		if err != nil {
			utils.RespondError(c, http.StatusBadRequest, err.Error())
			return
		}
		techID, _ := utils.GetTestTechnicianID(c)
		var body struct {
			Note         string `json:"note"`
			TechnicianID int    `json:"technician_id"`
		}
		if err := c.ShouldBindJSON(&body); err != nil || body.Note == "" {
			utils.RespondError(c, http.StatusBadRequest, "Le champ 'note' est requis")
			return
		}
		if body.TechnicianID > 0 { techID = body.TechnicianID }
		noteID, err := repository.InsertFieldNote(c.Request.Context(), db, "lampadaire", id, techID, body.Note)
		if err != nil {
			utils.RespondError(c, http.StatusInternalServerError, "Erreur enregistrement note")
			return
		}
		services.LogAction(c.Request.Context(), db, services.AuditEvent{
			UserID: services.NullableUserID(techID), Action: "lampadaire_field_note",
			TargetType: "lampadaire", TargetID: services.NullableInt(id), IPAddress: c.ClientIP(),
			Metadata: map[string]any{"note": body.Note},
		})
		utils.RespondJSON(c, http.StatusOK, gin.H{
			"status": "success", "message": "Note terrain ajoutée",
			"note_id": noteID, "lampadaire_id": id, "technician_id": techID,
		})
	}
}
