package services

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"

	"technicien-mobile/internal/models"
	"technicien-mobile/internal/repository"
)

// ApplySyncAction applies one SyncAction and returns a SyncActionResult.
// It is idempotent: if the local_id was already processed, the cached result is returned.
func ApplySyncAction(ctx context.Context, db *sql.DB, action models.SyncAction, techID int, deviceID string) models.SyncActionResult {
	// Idempotence check
	existing, _ := repository.GetSyncLogByLocalID(ctx, db, action.LocalID)
	if existing != nil {
		result := models.SyncActionResult{
			LocalID:  action.LocalID,
			Status:   existing.Status,
			ServerID: existing.ServerID,
			Message:  "Déjà traité",
		}
		return result
	}

	var serverID *int
	var errMsg string
	status := "success"
	message := "OK"

	switch action.Type {
	case "ACCEPT_WORK_ORDER":
		// Garde-fou anti double-prise : si le bon n'est plus libre (déjà pris
		// par un autre technicien pendant que celui-ci était hors-ligne), on
		// renvoie un conflit au lieu d'écraser silencieusement son affectation.
		cur, errStat := repository.GetWorkOrderCurrentStatus(ctx, db, action.EntityID)
		if errStat != nil {
			status, errMsg, message = "error", errStat.Error(), "Bon de travail introuvable"
			break
		}
		if cur != "created" && cur != "open" {
			status, message = "conflict", fmt.Sprintf("Bon de travail #%d déjà pris par un autre technicien", action.EntityID)
			break
		}
		err := applyWorkOrderTransition(ctx, db, action.EntityID, techID, "accepted", "", "")
		if err != nil {
			status, errMsg, message = "error", err.Error(), err.Error()
		} else {
			message = "Bon de travail accepté"
		}

	case "START_WORK_ORDER":
		err := applyWorkOrderTransition(ctx, db, action.EntityID, techID, "in_progress", "", "")
		if err != nil {
			status, errMsg, message = "error", err.Error(), err.Error()
		} else {
			message = "Intervention démarrée"
		}

	case "ADD_NOTE":
		note, _ := action.Payload["note"].(string)
		if note == "" {
			status, errMsg, message = "error", "note vide", "note manquante"
			break
		}
		curStatus, _ := repository.GetWorkOrderCurrentStatus(ctx, db, action.EntityID)
		err := repository.InsertWorkOrderLog(ctx, db, action.EntityID, techID, "note_added", note, curStatus, curStatus)
		if err != nil {
			status, errMsg, message = "error", err.Error(), err.Error()
		} else {
			message = "Note ajoutée"
		}

	case "RESOLVE_WORK_ORDER":
		note, _ := action.Payload["resolution_note"].(string)
		if note == "" {
			note, _ = action.Payload["note"].(string)
		}
		err := applyWorkOrderTransition(ctx, db, action.EntityID, techID, "resolved", "", note)
		if err != nil {
			status, errMsg, message = "error", err.Error(), err.Error()
		} else {
			message = "Intervention résolue"
		}

	case "BLOCK_WORK_ORDER":
		note, _ := action.Payload["note"].(string)
		curStatus, _ := repository.GetWorkOrderCurrentStatus(ctx, db, action.EntityID)
		err := repository.InsertWorkOrderLog(ctx, db, action.EntityID, techID, "blocked", note, curStatus, curStatus)
		if err != nil {
			status, errMsg, message = "error", err.Error(), err.Error()
		} else {
			message = "Blocage enregistré"
		}

	case "UPDATE_LOCATION":
		lat, _ := toFloat(action.Payload["latitude"])
		lng, _ := toFloat(action.Payload["longitude"])
		src, _ := action.Payload["source"].(string)
		if src == "" { src = "technician_mobile" }
		if lat == 0 && lng == 0 {
			status, errMsg, message = "error", "coordonnées manquantes", "lat/lng requis"
			break
		}
		err := repository.UpdateLampadaireLocation(ctx, db, action.EntityID, lat, lng, src)
		if err != nil {
			status, errMsg, message = "error", err.Error(), err.Error()
		} else {
			message = "Localisation mise à jour"
		}

	case "ADD_LAMPADAIRE_FIELD_NOTE":
		note, _ := action.Payload["note"].(string)
		if note == "" {
			status, errMsg, message = "error", "note vide", "note manquante"
			break
		}
		if _, err := repository.InsertFieldNote(ctx, db, "lampadaire", action.EntityID, techID, note); err != nil {
			status, errMsg, message = "error", err.Error(), err.Error()
		} else {
			message = "Note terrain lampadaire ajoutée"
		}

	case "ADD_LCU_FIELD_NOTE":
		note, _ := action.Payload["note"].(string)
		if note == "" {
			status, errMsg, message = "error", "note vide", "note manquante"
			break
		}
		if _, err := repository.InsertFieldNote(ctx, db, "lcu", action.EntityID, techID, note); err != nil {
			status, errMsg, message = "error", err.Error(), err.Error()
		} else {
			message = "Note terrain LCU ajoutée"
		}

	case "TEST_LCU_CONNECTIVITY":
		cur, err := repository.GetLCUStatus(ctx, db, action.EntityID)
		if err != nil {
			status, errMsg, message = "error", err.Error(), "LCU introuvable"
			break
		}
		newStatus := "offline"
		if cur == "online" { newStatus = "online" }
		_ = repository.UpdateLCUSeen(ctx, db, action.EntityID, newStatus, false)
		message = "Test de connectivité LCU effectué"

	case "SYNC_LCU":
		_ = repository.UpdateLCUSeen(ctx, db, action.EntityID, "online", true)
		message = "Synchronisation LCU effectuée"

	case "COMMISSIONING_UPDATE_GPS":
		lat, _ := toFloat(action.Payload["latitude"])
		lng, _ := toFloat(action.Payload["longitude"])
		if err := repository.CommissioningUpdateGPS(ctx, db, action.EntityID, lat, lng); err != nil {
			status, errMsg, message = "error", err.Error(), err.Error()
		} else {
			message = "GPS de mise en service mis à jour"
		}

	case "COMMISSIONING_TEST_COMMUNICATION":
		if err := repository.CommissioningSetTestComm(ctx, db, action.EntityID, true); err != nil {
			status, errMsg, message = "error", err.Error(), err.Error()
		} else {
			message = "Test communication enregistré"
		}

	case "COMMISSIONING_TEST_DIMMING":
		if err := repository.CommissioningSetTestDimming(ctx, db, action.EntityID, true); err != nil {
			status, errMsg, message = "error", err.Error(), err.Error()
		} else {
			message = "Test dimming enregistré"
		}

	case "COMMISSIONING_VALIDATE":
		if err := repository.CommissioningValidate(ctx, db, action.EntityID); err != nil {
			status, errMsg, message = "error", err.Error(), err.Error()
		} else {
			message = "Mise en service validée"
		}

	case "COMMISSIONING_FAIL":
		reason, _ := action.Payload["reason"].(string)
		if err := repository.CommissioningFail(ctx, db, action.EntityID, reason); err != nil {
			status, errMsg, message = "error", err.Error(), err.Error()
		} else {
			message = "Échec de mise en service enregistré"
		}

	case "COMMISSIONING_ADD_NOTE":
		note, _ := action.Payload["note"].(string)
		if err := repository.CommissioningAddNote(ctx, db, action.EntityID, note); err != nil {
			status, errMsg, message = "error", err.Error(), err.Error()
		} else {
			message = "Note de mise en service ajoutée"
		}

	default:
		message = fmt.Sprintf("Type d'action non supporté: %s", action.Type)
	}

	// Persist sync log
	_ = repository.InsertSyncLog(ctx, db, models.SyncLog{
		TechnicianID: techID,
		DeviceID:     deviceID,
		LocalID:      action.LocalID,
		ActionType:   action.Type,
		Entity:       action.Entity,
		EntityID:     NullableInt(action.EntityID),
		Payload:      action.Payload,
		ServerID:     serverID,
		Status:       status,
		ErrorMessage: errMsg,
	})

	return models.SyncActionResult{
		LocalID:  action.LocalID,
		Status:   status,
		ServerID: serverID,
		Message:  message,
	}
}

func applyWorkOrderTransition(ctx context.Context, db *sql.DB, id, techID int, newStatus, note, resolutionNote string) error {
	curStatus, err := repository.GetWorkOrderCurrentStatus(ctx, db, id)
	if err != nil {
		return fmt.Errorf("bon de travail #%d introuvable", id)
	}
	if err := repository.UpdateWorkOrderStatus(ctx, db, id, techID, newStatus, note, resolutionNote); err != nil {
		return err
	}
	return repository.InsertWorkOrderLog(ctx, db, id, techID, newStatus, note, curStatus, newStatus)
}

func toFloat(v any) (float64, bool) {
	switch val := v.(type) {
	case float64:
		return val, true
	case string:
		f, err := strconv.ParseFloat(val, 64)
		return f, err == nil
	}
	return 0, false
}
