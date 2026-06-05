package controllers

import (
	"context"
	"database/sql"
	"net/http"

	"github.com/gin-gonic/gin"
	"technicien-mobile/internal/models"
	"technicien-mobile/internal/repository"
	"technicien-mobile/internal/services"
	"technicien-mobile/internal/utils"
)

// GET /api/mobile/commissioning
func HandleListCommissioning(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		tasks, err := repository.ListCommissioningTasks(c.Request.Context(), db, c.Query("status"))
		if err != nil {
			utils.RespondError(c, http.StatusInternalServerError, "Erreur base de données")
			return
		}
		if tasks == nil { tasks = []models.CommissioningTask{} }
		utils.RespondJSON(c, http.StatusOK, gin.H{"tasks": tasks, "count": len(tasks)})
	}
}

// GET /api/mobile/commissioning/:id
func HandleGetCommissioning(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := utils.ParseIDParam(c, "id")
		if err != nil {
			utils.RespondError(c, http.StatusBadRequest, err.Error())
			return
		}
		t, err := repository.GetCommissioningTask(c.Request.Context(), db, id)
		if err != nil {
			utils.RespondError(c, http.StatusInternalServerError, "Erreur base de données")
			return
		}
		if t == nil {
			utils.RespondError(c, http.StatusNotFound, "Tâche de mise en service introuvable")
			return
		}
		utils.RespondJSON(c, http.StatusOK, t)
	}
}

// POST /api/mobile/commissioning/:id/update-gps
func HandleCommissioningGPS(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := utils.ParseIDParam(c, "id")
		if err != nil {
			utils.RespondError(c, http.StatusBadRequest, err.Error())
			return
		}
		techID, _ := utils.GetTestTechnicianID(c)
		var body struct {
			Latitude  float64 `json:"latitude"`
			Longitude float64 `json:"longitude"`
			TechnicianID int  `json:"technician_id"`
		}
		if err := c.ShouldBindJSON(&body); err != nil || (body.Latitude == 0 && body.Longitude == 0) {
			utils.RespondError(c, http.StatusBadRequest, "latitude et longitude sont requis")
			return
		}
		if body.TechnicianID > 0 { techID = body.TechnicianID }
		if err := repository.CommissioningUpdateGPS(c.Request.Context(), db, id, body.Latitude, body.Longitude); err != nil {
			utils.RespondError(c, http.StatusInternalServerError, "Erreur mise à jour GPS")
			return
		}
		auditCommissioning(c, db, techID, id, "commissioning_update_gps")
		utils.RespondJSON(c, http.StatusOK, gin.H{"status": "success", "message": "GPS mis à jour", "lampadaire_id": id})
	}
}

// POST /api/mobile/commissioning/:id/test-communication
func HandleCommissioningTestComm(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, techID, ok := commParseID(c)
		if !ok { return }
		// Simulation : passe si le lampadaire est en ligne
		online := commLampOnline(c.Request.Context(), db, id) //nolint
		if err := repository.CommissioningSetTestComm(c.Request.Context(), db, id, online); err != nil {
			utils.RespondError(c, http.StatusInternalServerError, "Erreur test communication")
			return
		}
		auditCommissioning(c, db, techID, id, "commissioning_test_comm")
		result := "passed"
		if !online { result = "failed" }
		utils.RespondJSON(c, http.StatusOK, gin.H{
			"status": "success", "test": "communication", "result": result, "lampadaire_id": id,
		})
	}
}

// POST /api/mobile/commissioning/:id/test-dimming
func HandleCommissioningTestDimming(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, techID, ok := commParseID(c)
		if !ok { return }
		online := commLampOnline(c.Request.Context(), db, id)
		if err := repository.CommissioningSetTestDimming(c.Request.Context(), db, id, online); err != nil {
			utils.RespondError(c, http.StatusInternalServerError, "Erreur test dimming")
			return
		}
		auditCommissioning(c, db, techID, id, "commissioning_test_dimming")
		result := "passed"
		if !online { result = "failed" }
		utils.RespondJSON(c, http.StatusOK, gin.H{
			"status": "success", "test": "dimming", "result": result, "lampadaire_id": id,
		})
	}
}

// POST /api/mobile/commissioning/:id/validate
func HandleCommissioningValidate(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, techID, ok := commParseID(c)
		if !ok { return }
		if err := repository.CommissioningValidate(c.Request.Context(), db, id); err != nil {
			utils.RespondError(c, http.StatusInternalServerError, "Erreur validation")
			return
		}
		auditCommissioning(c, db, techID, id, "commissioning_validate")
		utils.RespondJSON(c, http.StatusOK, gin.H{
			"status": "success", "message": "Lampadaire mis en service", "lampadaire_id": id,
			"commissioning_status": "commissioned",
		})
	}
}

// POST /api/mobile/commissioning/:id/fail
func HandleCommissioningFail(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := utils.ParseIDParam(c, "id")
		if err != nil {
			utils.RespondError(c, http.StatusBadRequest, err.Error())
			return
		}
		techID, _ := utils.GetTestTechnicianID(c)
		var body struct {
			Reason       string `json:"reason"`
			TechnicianID int    `json:"technician_id"`
		}
		if err := c.ShouldBindJSON(&body); err != nil || body.Reason == "" {
			utils.RespondError(c, http.StatusBadRequest, "Le champ 'reason' est requis")
			return
		}
		if body.TechnicianID > 0 { techID = body.TechnicianID }
		if err := repository.CommissioningFail(c.Request.Context(), db, id, body.Reason); err != nil {
			utils.RespondError(c, http.StatusInternalServerError, "Erreur signalement échec")
			return
		}
		auditCommissioning(c, db, techID, id, "commissioning_fail")
		utils.RespondJSON(c, http.StatusOK, gin.H{
			"status": "success", "message": "Échec enregistré", "lampadaire_id": id,
			"commissioning_status": "failed",
		})
	}
}

// POST /api/mobile/commissioning/:id/add-note
func HandleCommissioningNote(db *sql.DB) gin.HandlerFunc {
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
		if err := repository.CommissioningAddNote(c.Request.Context(), db, id, body.Note); err != nil {
			utils.RespondError(c, http.StatusInternalServerError, "Erreur ajout note")
			return
		}
		auditCommissioning(c, db, techID, id, "commissioning_add_note")
		utils.RespondJSON(c, http.StatusOK, gin.H{"status": "success", "message": "Note ajoutée", "lampadaire_id": id})
	}
}

/* ── helpers ── */

func commParseID(c *gin.Context) (int, int, bool) {
	id, err := utils.ParseIDParam(c, "id")
	if err != nil {
		utils.RespondError(c, http.StatusBadRequest, err.Error())
		return 0, 0, false
	}
	techID, _ := utils.GetTestTechnicianID(c)
	var body struct{ TechnicianID int `json:"technician_id"` }
	_ = c.ShouldBindJSON(&body)
	if body.TechnicianID > 0 { techID = body.TechnicianID }
	return id, techID, true
}

func commLampOnline(ctx context.Context, db *sql.DB, id int) bool {
	var etat string
	_ = db.QueryRowContext(ctx, `SELECT COALESCE(etat,'offline') FROM lampadaires WHERE id=$1`, id).Scan(&etat)
	return etat == "online"
}

func auditCommissioning(c *gin.Context, db *sql.DB, techID, id int, action string) {
	services.LogAction(c.Request.Context(), db, services.AuditEvent{
		UserID: services.NullableUserID(techID), Action: action,
		TargetType: "lampadaire", TargetID: services.NullableInt(id), IPAddress: c.ClientIP(),
	})
}
