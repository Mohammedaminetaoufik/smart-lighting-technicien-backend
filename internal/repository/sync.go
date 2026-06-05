package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"technicien-mobile/internal/models"
)

func GetSyncLogByLocalID(ctx context.Context, db *sql.DB, localID string) (*models.SyncLog, error) {
	var s models.SyncLog
	var payload []byte
	var serverID sql.NullInt64
	var entityID sql.NullInt64
	err := db.QueryRowContext(ctx, `
		SELECT id, technician_id, COALESCE(device_id,''), local_id, action_type,
		       COALESCE(entity,''), entity_id, payload, server_id,
		       status, COALESCE(error_message,''), created_at
		FROM mobile_sync_logs WHERE local_id=$1`, localID).
		Scan(&s.ID, &s.TechnicianID, &s.DeviceID, &s.LocalID, &s.ActionType,
			&s.Entity, &entityID, &payload, &serverID,
			&s.Status, &s.ErrorMessage, &s.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if entityID.Valid { v := int(entityID.Int64); s.EntityID = &v }
	if serverID.Valid { v := int(serverID.Int64); s.ServerID = &v }
	if payload != nil {
		_ = json.Unmarshal(payload, &s.Payload)
	}
	return &s, nil
}

func InsertSyncLog(ctx context.Context, db *sql.DB, log models.SyncLog) error {
	var payload []byte
	if log.Payload != nil {
		payload, _ = json.Marshal(log.Payload)
	}
	_, err := db.ExecContext(ctx, `
		INSERT INTO mobile_sync_logs
		  (technician_id, device_id, local_id, action_type, entity, entity_id, payload, server_id, status, error_message)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		ON CONFLICT (local_id) DO NOTHING`,
		log.TechnicianID, log.DeviceID, log.LocalID, log.ActionType,
		log.Entity, log.EntityID, payload, log.ServerID, log.Status, log.ErrorMessage)
	return err
}

func UpsertMobileDevice(ctx context.Context, db *sql.DB, techID int, deviceID, name, platform string) error {
	_, err := db.ExecContext(ctx, `
		INSERT INTO mobile_devices (technician_id, device_id, device_name, platform, last_sync_at)
		VALUES ($1,$2,$3,$4,$5)
		ON CONFLICT (device_id) DO UPDATE
		  SET last_sync_at=$5, device_name=EXCLUDED.device_name, platform=EXCLUDED.platform`,
		techID, deviceID, name, platform, time.Now())
	return err
}

func GetRecentSyncLogs(ctx context.Context, db *sql.DB, techID int, since *time.Time) ([]models.SyncLog, error) {
	query := `
		SELECT id, technician_id, COALESCE(device_id,''), local_id, action_type,
		       COALESCE(entity,''), entity_id, payload, server_id,
		       status, COALESCE(error_message,''), created_at
		FROM mobile_sync_logs WHERE technician_id=$1`
	args := []any{techID}
	if since != nil {
		query += " AND created_at > $2"
		args = append(args, *since)
	}
	query += " ORDER BY created_at DESC LIMIT 100"
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []models.SyncLog
	for rows.Next() {
		var s models.SyncLog
		var payload []byte
		var serverID, entityID sql.NullInt64
		if err := rows.Scan(&s.ID, &s.TechnicianID, &s.DeviceID, &s.LocalID, &s.ActionType,
			&s.Entity, &entityID, &payload, &serverID, &s.Status, &s.ErrorMessage, &s.CreatedAt); err != nil {
			continue
		}
		if entityID.Valid { v := int(entityID.Int64); s.EntityID = &v }
		if serverID.Valid { v := int(serverID.Int64); s.ServerID = &v }
		if payload != nil { _ = json.Unmarshal(payload, &s.Payload) }
		result = append(result, s)
	}
	return result, nil
}
