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

// GET /api/map/overview
func HandleMapOverview(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		lamps, _ := repository.GetMapLampadaires(c.Request.Context(), db, "", "", 0)
		lcus, _ := repository.GetMapLCUs(c.Request.Context(), db)
		missing, _ := repository.GetMissingLocationLampadaires(c.Request.Context(), db)
		online, offline, maintenance := 0, 0, 0
		for _, l := range lamps {
			switch l.Etat {
			case "online": online++
			case "offline": offline++
			case "maintenance": maintenance++
			}
		}
		utils.RespondJSON(c, http.StatusOK, gin.H{
			"lampadaires_total":       len(lamps),
			"lampadaires_online":      online,
			"lampadaires_offline":     offline,
			"lampadaires_maintenance": maintenance,
			"lcus_total":              len(lcus),
			"missing_location":        len(missing),
		})
	}
}

// GET /api/map/lampadaires
func HandleMapLampadaires(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		lcuID, _ := strconv.Atoi(c.Query("lcu_id"))
		lamps, err := repository.GetMapLampadaires(c.Request.Context(), db, c.Query("zone"), c.Query("etat"), lcuID)
		if err != nil {
			utils.RespondError(c, http.StatusInternalServerError, "Erreur base de données")
			return
		}
		if lamps == nil { lamps = []models.MapLampadaire{} }
		utils.RespondJSON(c, http.StatusOK, gin.H{"lampadaires": lamps, "count": len(lamps)})
	}
}

// GET /api/map/lcus
func HandleMapLCUs(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		lcus, err := repository.GetMapLCUs(c.Request.Context(), db)
		if err != nil {
			utils.RespondError(c, http.StatusInternalServerError, "Erreur base de données")
			return
		}
		if lcus == nil { lcus = []models.MapLCU{} }
		utils.RespondJSON(c, http.StatusOK, gin.H{"lcus": lcus, "count": len(lcus)})
	}
}

// GET /api/map/connections
func HandleMapConnections(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		lcuID, _ := strconv.Atoi(c.Query("lcu_id"))
		conns, err := repository.GetMapConnections(c.Request.Context(), db, lcuID)
		if err != nil {
			utils.RespondError(c, http.StatusInternalServerError, "Erreur base de données")
			return
		}
		if conns == nil { conns = []models.MapConnection{} }
		utils.RespondJSON(c, http.StatusOK, gin.H{"connections": conns, "count": len(conns)})
	}
}

// GET /api/map/lampadaires/missing-location
func HandleMissingLocation(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		missing, err := repository.GetMissingLocationLampadaires(c.Request.Context(), db)
		if err != nil {
			utils.RespondError(c, http.StatusInternalServerError, "Erreur base de données")
			return
		}
		if missing == nil { missing = []models.MapLampadaire{} }
		utils.RespondJSON(c, http.StatusOK, gin.H{"lampadaires": missing, "count": len(missing)})
	}
}

// GET /api/map/technician-context
// TODO(auth): Quand AUTH_ENABLED=true, extraire technicianID depuis le JWT.
func HandleTechnicianContext(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		techID, _ := utils.GetTestTechnicianID(c)
		ctx := c.Request.Context()

		result := models.TechnicianContext{TechnicianID: techID}

		// Parse optional position
		if latStr := c.Query("latitude"); latStr != "" {
			if lat, err := strconv.ParseFloat(latStr, 64); err == nil {
				if lngStr := c.Query("longitude"); lngStr != "" {
					if lng, err2 := strconv.ParseFloat(lngStr, 64); err2 == nil {
						result.Position = &models.GeoPosition{Latitude: lat, Longitude: lng}
					}
				}
			}
		}

		// Assigned lampadaires (via active work orders)
		assigned, _ := repository.GetTechnicianAssignedLampadaires(ctx, db, techID)
		if assigned == nil { assigned = []models.MapLampadaire{} }
		result.AssignedLampadaires = assigned

		// Nearby (if position provided)
		if result.Position != nil {
			radius, _ := strconv.ParseFloat(c.DefaultQuery("radius", "2000"), 64)
			nearby, _ := repository.GetNearbyLampadaires(ctx, db, result.Position.Latitude, result.Position.Longitude, radius)
			if nearby == nil { nearby = []models.MapLampadaire{} }
			result.NearbyLampadaires = nearby
		} else {
			result.NearbyLampadaires = []models.MapLampadaire{}
		}

		// LCUs
		if c.DefaultQuery("include_lcus", "true") == "true" {
			lcus, _ := repository.GetMapLCUs(ctx, db)
			if lcus == nil { lcus = []models.MapLCU{} }
			result.LCUs = lcus
		}

		// Connections
		if c.DefaultQuery("include_connections", "true") == "true" {
			conns, _ := repository.GetMapConnections(ctx, db, 0)
			if conns == nil { conns = []models.MapConnection{} }
			result.Connections = conns
		}

		// Missing location
		missing, _ := repository.GetMissingLocationLampadaires(ctx, db)
		if missing == nil { missing = []models.MapLampadaire{} }
		result.MissingLocation = missing

		utils.RespondJSON(c, http.StatusOK, result)
	}
}

// POST /api/map/lampadaires/:id/location
// TODO(auth): Quand AUTH_ENABLED=true, enregistrer l'id du technicien connecté dans access_logs.
func HandleUpdateLocation(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := utils.ParseIDParam(c, "id")
		if err != nil {
			utils.RespondError(c, http.StatusBadRequest, err.Error())
			return
		}
		techID, _ := utils.GetTestTechnicianID(c)
		var body struct {
			Latitude  float64 `json:"latitude"  binding:"required"`
			Longitude float64 `json:"longitude" binding:"required"`
			Accuracy  float64 `json:"accuracy"`
			Source    string  `json:"source"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			utils.RespondError(c, http.StatusBadRequest, "latitude et longitude sont requis")
			return
		}
		src := body.Source
		if src == "" { src = "technician_mobile" }
		if err := repository.UpdateLampadaireLocation(c.Request.Context(), db, id, body.Latitude, body.Longitude, src); err != nil {
			utils.RespondError(c, http.StatusInternalServerError, "Erreur mise à jour localisation")
			return
		}
		services.LogAction(c.Request.Context(), db, services.AuditEvent{
			UserID: services.NullableUserID(techID), Action: "location_updated",
			TargetType: "lampadaire", TargetID: services.NullableInt(id),
			IPAddress: c.ClientIP(),
			Metadata: map[string]any{"latitude": body.Latitude, "longitude": body.Longitude, "source": src},
		})
		utils.RespondJSON(c, http.StatusOK, gin.H{
			"status": "success", "message": "Localisation mise à jour",
			"lampadaire_id": id, "latitude": body.Latitude, "longitude": body.Longitude,
		})
	}
}

// POST /api/map/lampadaires/:id/dimming
func HandleUpdateLampadaireDimming(db *sql.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id, err := utils.ParseIDParam(c, "id")
		if err != nil {
			utils.RespondError(c, http.StatusBadRequest, err.Error())
			return
		}
		techID, _ := utils.GetTestTechnicianID(c)
		var body struct {
			Intensity int    `json:"intensity"`
			Reason    string `json:"reason"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			utils.RespondError(c, http.StatusBadRequest, "intensité requise")
			return
		}

		// Simple update in mobile backend (parity with web's dimming_commands but simplified)
		_, err = db.ExecContext(c.Request.Context(), `
			UPDATE lampadaires SET intensite = $1, updated_at = NOW() WHERE id = $2
		`, body.Intensity, id)

		if err != nil {
			utils.RespondError(c, http.StatusInternalServerError, "Erreur mise à jour intensité")
			return
		}

		services.LogAction(c.Request.Context(), db, services.AuditEvent{
			UserID: services.NullableUserID(techID), Action: "lampadaire_dimmed",
			TargetType: "lampadaire", TargetID: services.NullableInt(id),
			IPAddress: c.ClientIP(),
			Metadata: map[string]any{"intensity": body.Intensity, "reason": body.Reason},
		})

		utils.RespondJSON(c, http.StatusOK, gin.H{
			"status": "success", "message": "Intensité mise à jour",
			"lampadaire_id": id, "intensity": body.Intensity,
		})
	}
}
