package controllers

import (
	"database/sql"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"technicien-mobile/internal/models"
	"technicien-mobile/internal/repository"
	"technicien-mobile/internal/utils"
)

// GET /api/mobile/me/workorders
// TODO(auth): Quand AUTH_ENABLED=true, extraire technicianID depuis le JWT.
func HandleListMyWorkOrders(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		techID, _ := utils.GetTestTechnicianID(c)
		limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
		offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
		filters := repository.WorkOrderFilters{
			TechnicianID: techID,
			Status:       c.Query("status"),
			Priority:     c.Query("priority"),
			Zone:         c.Query("zone"),
			Limit:        limit,
			Offset:       offset,
		}
		list, err := repository.ListTechnicianWorkOrders(c.Request.Context(), db, filters)
		if err != nil {
			utils.RespondError(c, http.StatusInternalServerError, "Erreur base de données")
			return
		}
		if list == nil {
			list = []models.MobileWorkOrder{}
		}
		utils.RespondJSON(c, http.StatusOK, gin.H{
			"technician_id": techID,
			"count":         len(list),
			"work_orders":   list,
		})
	}
}

// GET /api/mobile/workorders/:id
// TODO(auth): Quand AUTH_ENABLED=true, vérifier que le work order appartient au technicien connecté.
func HandleGetWorkOrder(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := utils.ParseIDParam(c, "id")
		if err != nil {
			utils.RespondError(c, http.StatusBadRequest, err.Error())
			return
		}
		wo, err := repository.GetWorkOrderByID(c.Request.Context(), db, id)
		if err != nil {
			utils.RespondError(c, http.StatusInternalServerError, "Erreur base de données")
			return
		}
		if wo == nil {
			utils.RespondError(c, http.StatusNotFound, "Bon de travail introuvable")
			return
		}
		utils.RespondJSON(c, http.StatusOK, wo)
	}
}
