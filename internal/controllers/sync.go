package controllers

import (
	"database/sql"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"technicien-mobile/internal/models"
	"technicien-mobile/internal/repository"
	"technicien-mobile/internal/services"
	"technicien-mobile/internal/utils"
)

// GET /api/mobile/sync/bootstrap
// Returns all data needed for the technician to work offline.
func HandleSyncBootstrap(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		techID, _ := utils.GetTestTechnicianID(c)
		ctx := c.Request.Context()

		dashboard, _ := repository.GetTechnicianDashboard(ctx, db, techID)
		workOrders, _ := repository.ListTechnicianWorkOrders(ctx, db, repository.WorkOrderFilters{
			TechnicianID: techID, Limit: 100,
		})
		lamps, _ := repository.GetTechnicianAssignedLampadaires(ctx, db, techID)
		lcus, _ := repository.GetMapLCUs(ctx, db)
		conns, _ := repository.GetMapConnections(ctx, db, 0)
		commissioning, _ := repository.ListCommissioningTasks(ctx, db, "")
		missing, _ := repository.GetMissingLocationLampadaires(ctx, db)

		if workOrders == nil { workOrders = []models.MobileWorkOrder{} }
		if lamps == nil { lamps = []models.MapLampadaire{} }
		if lcus == nil { lcus = []models.MapLCU{} }
		if conns == nil { conns = []models.MapConnection{} }
		if commissioning == nil { commissioning = []models.CommissioningTask{} }
		if missing == nil { missing = []models.MapLampadaire{} }

		utils.RespondJSON(c, http.StatusOK, gin.H{
			"technician_id":    techID,
			"server_time":      time.Now().UTC().Format(time.RFC3339),
			"dashboard":        dashboard,
			"work_orders":      workOrders,
			"lampadaires":      lamps,
			"lcus":             lcus,
			"connections":      conns,
			"commissioning":    commissioning,
			"missing_location": missing,
			"last_sync_at":     dashboard.Sync.LastSyncAt,
		})
	}
}

// GET /api/mobile/sync/pull
// Returns data changed since a given timestamp.
func HandleSyncPull(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		techID, _ := utils.GetTestTechnicianID(c)
		ctx := c.Request.Context()

		var since *time.Time
		if sinceStr := c.Query("since"); sinceStr != "" {
			if t, err := time.Parse(time.RFC3339, sinceStr); err == nil {
				since = &t
			}
		}

		logs, _ := repository.GetRecentSyncLogs(ctx, db, techID, since)
		workOrders, _ := repository.ListTechnicianWorkOrders(ctx, db, repository.WorkOrderFilters{
			TechnicianID: techID, Limit: 100,
		})
		if workOrders == nil { workOrders = []models.MobileWorkOrder{} }
		if logs == nil { logs = []models.SyncLog{} }

		utils.RespondJSON(c, http.StatusOK, gin.H{
			"technician_id": techID,
			"server_time":   time.Now().UTC().Format(time.RFC3339),
			"work_orders":   workOrders,
			"sync_logs":     logs,
		})
	}
}

// POST /api/mobile/sync/push
// Applies a batch of SyncActions from the mobile device.
func HandleSyncPush(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req models.SyncPushRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			utils.RespondError(c, http.StatusBadRequest, "Corps JSON invalide: "+err.Error())
			return
		}
		// Allow technician_id override from query param
		techID, _ := utils.GetTestTechnicianID(c)
		techName := c.GetString("user_name")
		if req.TechnicianID > 0 {
			techID = req.TechnicianID
		}
		deviceID := req.DeviceID
		if deviceID == "" { deviceID = "unknown" }

		// Register device
		_ = repository.UpsertMobileDevice(c.Request.Context(), db, techID, deviceID, req.DeviceName, req.Platform)

		var results []models.SyncActionResult
		for _, action := range req.Actions {
			if action.LocalID == "" {
				results = append(results, models.SyncActionResult{
					LocalID: action.LocalID, Status: "error", Message: "local_id est requis",
				})
				continue
			}
			result := services.ApplySyncAction(c.Request.Context(), db, action, techID, techName, deviceID)
			results = append(results, result)
		}
		if results == nil { results = []models.SyncActionResult{} }

		syncID := fmt.Sprintf("sync_%s_%03d", time.Now().UTC().Format("20060102_150405"), len(results))
		utils.RespondJSON(c, http.StatusOK, models.SyncPushResponse{
			SyncID:     syncID,
			Status:     "completed",
			ServerTime: time.Now().UTC(),
			Results:    results,
		})
	}
}

// POST /api/mobile/sync/full
// Full sync: bootstrap + push in one request.
func HandleSyncFull(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req models.SyncPushRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			utils.RespondError(c, http.StatusBadRequest, "Corps JSON invalide")
			return
		}
		techID, _ := utils.GetTestTechnicianID(c)
		techName := c.GetString("user_name")
		if req.TechnicianID > 0 { techID = req.TechnicianID }
		deviceID := req.DeviceID
		if deviceID == "" { deviceID = "unknown" }
		_ = repository.UpsertMobileDevice(c.Request.Context(), db, techID, deviceID, req.DeviceName, req.Platform)

		var results []models.SyncActionResult
		for _, action := range req.Actions {
			if action.LocalID == "" { continue }
			results = append(results, services.ApplySyncAction(c.Request.Context(), db, action, techID, techName, deviceID))
		}

		// Bootstrap data
		workOrders, _ := repository.ListTechnicianWorkOrders(c.Request.Context(), db, repository.WorkOrderFilters{
			TechnicianID: techID, Limit: 100,
		})
		lamps, _ := repository.GetTechnicianAssignedLampadaires(c.Request.Context(), db, techID)
		lcus, _ := repository.GetMapLCUs(c.Request.Context(), db)

		if workOrders == nil { workOrders = []models.MobileWorkOrder{} }
		if lamps == nil { lamps = []models.MapLampadaire{} }
		if lcus == nil { lcus = []models.MapLCU{} }
		if results == nil { results = []models.SyncActionResult{} }

		utils.RespondJSON(c, http.StatusOK, gin.H{
			"sync_id":       fmt.Sprintf("sync_%s", time.Now().UTC().Format("20060102_150405")),
			"status":        "completed",
			"server_time":   time.Now().UTC().Format(time.RFC3339),
			"push_results":  results,
			"work_orders":   workOrders,
			"lampadaires":   lamps,
			"lcus":          lcus,
		})
	}
}
