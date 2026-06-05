package controllers

import (
	"database/sql"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"technicien-mobile/internal/models"
	"technicien-mobile/internal/repository"
	"technicien-mobile/internal/services"
	"technicien-mobile/internal/utils"
)

// GET /api/mobile/lcus
func HandleListLCUsMobile(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		lcus, err := repository.ListLCUsForTech(c.Request.Context(), db)
		if err != nil {
			utils.RespondError(c, http.StatusInternalServerError, "Erreur base de données")
			return
		}
		if lcus == nil { lcus = []models.LCUDetail{} }
		utils.RespondJSON(c, http.StatusOK, gin.H{"lcus": lcus, "count": len(lcus)})
	}
}

// GET /api/mobile/lcus/:id/details
func HandleLCUDetails(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := utils.ParseIDParam(c, "id")
		if err != nil {
			utils.RespondError(c, http.StatusBadRequest, err.Error())
			return
		}
		d, err := repository.GetLCUDetail(c.Request.Context(), db, id)
		if err != nil {
			utils.RespondError(c, http.StatusInternalServerError, "Erreur base de données")
			return
		}
		if d == nil {
			utils.RespondError(c, http.StatusNotFound, "LCU introuvable")
			return
		}
		utils.RespondJSON(c, http.StatusOK, d)
	}
}

// GET /api/mobile/lcus/:id/lampadaires
func HandleLCULampadairesMobile(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := utils.ParseIDParam(c, "id")
		if err != nil {
			utils.RespondError(c, http.StatusBadRequest, err.Error())
			return
		}
		lamps, _ := repository.GetLCULampadaires(c.Request.Context(), db, id)
		if lamps == nil { lamps = []models.MapLampadaire{} }
		utils.RespondJSON(c, http.StatusOK, gin.H{"lcu_id": id, "lampadaires": lamps, "count": len(lamps)})
	}
}

// GET /api/mobile/lcus/:id/diagnostic
func HandleLCUDiagnostic(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := utils.ParseIDParam(c, "id")
		if err != nil {
			utils.RespondError(c, http.StatusBadRequest, err.Error())
			return
		}
		d, err := repository.GetLCUDetail(c.Request.Context(), db, id)
		if err != nil || d == nil {
			utils.RespondError(c, http.StatusNotFound, "LCU introuvable")
			return
		}
		health := "ok"
		if d.OfflineCount > 0 && d.OnlineCount == 0 {
			health = "critical"
		} else if d.OfflineCount > 0 {
			health = "degraded"
		}
		utils.RespondJSON(c, http.StatusOK, gin.H{
			"lcu_id":            id,
			"reference":         d.Reference,
			"status":            d.Status,
			"health":            health,
			"lampadaires_count": d.LampadairesCount,
			"online_count":      d.OnlineCount,
			"offline_count":     d.OfflineCount,
			"maintenance_count": d.MaintenanceCount,
			"last_seen_at":      d.LastSeenAt,
			"last_sync_at":      d.LastSyncAt,
		})
	}
}

// POST /api/mobile/lcus/:id/test — teste la connectivité (simulation)
func HandleLCUTest(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := utils.ParseIDParam(c, "id")
		if err != nil {
			utils.RespondError(c, http.StatusBadRequest, err.Error())
			return
		}
		techID, _ := utils.GetTestTechnicianID(c)
		cur, e := repository.GetLCUStatus(c.Request.Context(), db, id)
		if e != nil {
			utils.RespondError(c, http.StatusNotFound, "LCU introuvable")
			return
		}
		// Simulation : si la LCU était online elle répond, sinon timeout
		online := cur == "online"
		status := "online"
		latency := 42
		message := "Connexion établie"
		if !online {
			status = "offline"
			latency = 0
			message = "Pas de réponse (timeout)"
		}
		_ = repository.UpdateLCUSeen(c.Request.Context(), db, id, status, false)
		services.LogAction(c.Request.Context(), db, services.AuditEvent{
			UserID: services.NullableUserID(techID), Action: "lcu_test",
			TargetType: "lcu", TargetID: services.NullableInt(id), IPAddress: c.ClientIP(),
			Metadata: map[string]any{"status": status, "latency_ms": latency},
		})
		utils.RespondJSON(c, http.StatusOK, gin.H{
			"status": status, "latency_ms": latency, "message": message,
			"tested_at": time.Now().UTC().Format(time.RFC3339), "lcu_id": id,
		})
	}
}

// POST /api/mobile/lcus/:id/sync — demande une synchronisation (simulation)
func HandleLCUSync(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := utils.ParseIDParam(c, "id")
		if err != nil {
			utils.RespondError(c, http.StatusBadRequest, err.Error())
			return
		}
		techID, _ := utils.GetTestTechnicianID(c)
		cur, e := repository.GetLCUStatus(c.Request.Context(), db, id)
		if e != nil {
			utils.RespondError(c, http.StatusNotFound, "LCU introuvable")
			return
		}
		if cur != "online" {
			utils.RespondJSON(c, http.StatusConflict, gin.H{
				"status": "failed", "message": "LCU hors ligne, synchronisation impossible", "lcu_id": id,
			})
			return
		}
		_ = repository.UpdateLCUSeen(c.Request.Context(), db, id, "online", true)
		services.LogAction(c.Request.Context(), db, services.AuditEvent{
			UserID: services.NullableUserID(techID), Action: "lcu_sync",
			TargetType: "lcu", TargetID: services.NullableInt(id), IPAddress: c.ClientIP(),
		})
		utils.RespondJSON(c, http.StatusOK, gin.H{
			"status": "success", "message": "Synchronisation effectuée",
			"synced_at": time.Now().UTC().Format(time.RFC3339), "lcu_id": id,
		})
	}
}

// POST /api/mobile/lcus/:id/field-note
func HandleLCUFieldNote(db *sql.DB) gin.HandlerFunc {
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
		noteID, err := repository.InsertFieldNote(c.Request.Context(), db, "lcu", id, techID, body.Note)
		if err != nil {
			utils.RespondError(c, http.StatusInternalServerError, "Erreur enregistrement note")
			return
		}
		services.LogAction(c.Request.Context(), db, services.AuditEvent{
			UserID: services.NullableUserID(techID), Action: "lcu_field_note",
			TargetType: "lcu", TargetID: services.NullableInt(id), IPAddress: c.ClientIP(),
			Metadata: map[string]any{"note": body.Note},
		})
		utils.RespondJSON(c, http.StatusOK, gin.H{
			"status": "success", "message": "Note terrain LCU ajoutée",
			"note_id": noteID, "lcu_id": id, "technician_id": techID,
		})
	}
}
