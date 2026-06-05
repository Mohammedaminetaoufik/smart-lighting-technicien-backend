package controllers

import (
	"database/sql"
	"net/http"

	"github.com/gin-gonic/gin"
	"technicien-mobile/internal/repository"
	"technicien-mobile/internal/services"
	"technicien-mobile/internal/utils"
)

func requireStatus(allowed ...string) func(string) bool {
	return func(s string) bool {
		for _, a := range allowed {
			if s == a {
				return true
			}
		}
		return false
	}
}

// POST /api/mobile/workorders/:id/accept
func HandleAcceptWorkOrder(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := utils.ParseIDParam(c, "id")
		if err != nil {
			utils.RespondError(c, http.StatusBadRequest, err.Error())
			return
		}
		techID, _ := utils.GetTestTechnicianID(c)
		// Override from body if provided
		var body struct {
			TechnicianID int `json:"technician_id"`
		}
		_ = c.ShouldBindJSON(&body)
		if body.TechnicianID > 0 {
			techID = body.TechnicianID
		}

		curStatus, err := repository.GetWorkOrderCurrentStatus(c.Request.Context(), db, id)
		if err != nil {
			utils.RespondError(c, http.StatusNotFound, "Bon de travail introuvable")
			return
		}
		if !requireStatus("created", "open")(curStatus) {
			utils.RespondError(c, http.StatusConflict, "Le bon de travail ne peut pas être accepté depuis l'état: "+curStatus)
			return
		}
		if err := repository.UpdateWorkOrderStatus(c.Request.Context(), db, id, techID, "accepted", "", ""); err != nil {
			utils.RespondError(c, http.StatusInternalServerError, "Erreur mise à jour")
			return
		}
		_ = repository.InsertWorkOrderLog(c.Request.Context(), db, id, techID, "accepted", "", curStatus, "accepted")
		services.LogAction(c.Request.Context(), db, services.AuditEvent{
			UserID: services.NullableUserID(techID), Action: "workorder_accepted",
			TargetType: "work_order", TargetID: services.NullableInt(id),
			IPAddress: c.ClientIP(), UserAgent: c.Request.UserAgent(),
			Metadata: map[string]any{"technician_id": techID},
		})
		utils.RespondJSON(c, http.StatusOK, gin.H{
			"status": "success", "message": "Bon de travail accepté",
			"work_order_id": id, "technician_id": techID, "new_status": "accepted",
		})
	}
}

// POST /api/mobile/workorders/:id/start
func HandleStartWorkOrder(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := utils.ParseIDParam(c, "id")
		if err != nil {
			utils.RespondError(c, http.StatusBadRequest, err.Error())
			return
		}
		techID, _ := utils.GetTestTechnicianID(c)
		var body struct{ TechnicianID int `json:"technician_id"` }
		_ = c.ShouldBindJSON(&body)
		if body.TechnicianID > 0 { techID = body.TechnicianID }

		curStatus, err := repository.GetWorkOrderCurrentStatus(c.Request.Context(), db, id)
		if err != nil {
			utils.RespondError(c, http.StatusNotFound, "Bon de travail introuvable")
			return
		}
		if !requireStatus("accepted")(curStatus) {
			utils.RespondError(c, http.StatusConflict, "Le bon de travail doit être accepté avant de démarrer, état actuel: "+curStatus)
			return
		}
		if err := repository.UpdateWorkOrderStatus(c.Request.Context(), db, id, techID, "in_progress", "", ""); err != nil {
			utils.RespondError(c, http.StatusInternalServerError, "Erreur mise à jour")
			return
		}
		_ = repository.InsertWorkOrderLog(c.Request.Context(), db, id, techID, "started", "", curStatus, "in_progress")
		services.LogAction(c.Request.Context(), db, services.AuditEvent{
			UserID: services.NullableUserID(techID), Action: "workorder_started",
			TargetType: "work_order", TargetID: services.NullableInt(id),
			IPAddress: c.ClientIP(),
		})
		utils.RespondJSON(c, http.StatusOK, gin.H{
			"status": "success", "message": "Intervention démarrée",
			"work_order_id": id, "technician_id": techID, "new_status": "in_progress",
		})
	}
}

// POST /api/mobile/workorders/:id/add-note
func HandleAddNote(db *sql.DB) gin.HandlerFunc {
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
		curStatus, _ := repository.GetWorkOrderCurrentStatus(c.Request.Context(), db, id)
		if err := repository.InsertWorkOrderLog(c.Request.Context(), db, id, techID, "note_added", body.Note, curStatus, curStatus); err != nil {
			utils.RespondError(c, http.StatusInternalServerError, "Erreur ajout de note")
			return
		}
		utils.RespondJSON(c, http.StatusOK, gin.H{
			"status": "success", "message": "Note ajoutée",
			"work_order_id": id, "technician_id": techID,
		})
	}
}

// POST /api/mobile/workorders/:id/resolve
func HandleResolveWorkOrder(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := utils.ParseIDParam(c, "id")
		if err != nil {
			utils.RespondError(c, http.StatusBadRequest, err.Error())
			return
		}
		techID, _ := utils.GetTestTechnicianID(c)
		var body struct {
			ResolutionNote string `json:"resolution_note"`
			Note           string `json:"note"`
			TechnicianID   int    `json:"technician_id"`
		}
		_ = c.ShouldBindJSON(&body)
		if body.TechnicianID > 0 { techID = body.TechnicianID }
		note := body.ResolutionNote
		if note == "" { note = body.Note }

		curStatus, err := repository.GetWorkOrderCurrentStatus(c.Request.Context(), db, id)
		if err != nil {
			utils.RespondError(c, http.StatusNotFound, "Bon de travail introuvable")
			return
		}
		if !requireStatus("in_progress", "accepted")(curStatus) {
			utils.RespondError(c, http.StatusConflict, "Le bon de travail ne peut pas être résolu depuis l'état: "+curStatus)
			return
		}
		if err := repository.UpdateWorkOrderStatus(c.Request.Context(), db, id, techID, "resolved", "", note); err != nil {
			utils.RespondError(c, http.StatusInternalServerError, "Erreur mise à jour")
			return
		}
		_ = repository.InsertWorkOrderLog(c.Request.Context(), db, id, techID, "resolved", note, curStatus, "resolved")
		services.LogAction(c.Request.Context(), db, services.AuditEvent{
			UserID: services.NullableUserID(techID), Action: "workorder_resolved",
			TargetType: "work_order", TargetID: services.NullableInt(id),
			IPAddress: c.ClientIP(),
		})
		utils.RespondJSON(c, http.StatusOK, gin.H{
			"status": "success", "message": "Intervention résolue",
			"work_order_id": id, "technician_id": techID, "new_status": "resolved",
		})
	}
}

// POST /api/mobile/workorders/:id/block
func HandleBlockWorkOrder(db *sql.DB) gin.HandlerFunc {
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
		curStatus, _ := repository.GetWorkOrderCurrentStatus(c.Request.Context(), db, id)
		_ = repository.InsertWorkOrderLog(c.Request.Context(), db, id, techID, "blocked", body.Note, curStatus, curStatus)
		utils.RespondJSON(c, http.StatusOK, gin.H{
			"status": "success", "message": "Blocage enregistré",
			"work_order_id": id, "technician_id": techID,
		})
	}
}
