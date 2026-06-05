package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
)

type AuditEvent struct {
	UserID     *int
	Action     string
	TargetType string
	TargetID   *int
	IPAddress  string
	UserAgent  string
	Metadata   map[string]any
}

// LogAction writes to access_logs — the same table used by the web backend.
func LogAction(ctx context.Context, db *sql.DB, evt AuditEvent) {
	var meta []byte
	if len(evt.Metadata) > 0 {
		if b, err := json.Marshal(evt.Metadata); err == nil {
			meta = b
		}
	}
	_, err := db.ExecContext(ctx, `
		INSERT INTO access_logs (user_id, action, target_type, target_id, ip_address, user_agent, metadata)
		VALUES ($1, $2, NULLIF($3,''), $4, NULLIF($5,''), NULLIF($6,''), $7)`,
		evt.UserID, evt.Action, evt.TargetType, evt.TargetID,
		evt.IPAddress, evt.UserAgent, meta)
	if err != nil {
		log.Printf("[audit] error: %v (action=%s)", err, evt.Action)
	}
}

func NullableUserID(id int) *int {
	if id <= 0 {
		return nil
	}
	return &id
}

func NullableInt(v int) *int {
	if v <= 0 {
		return nil
	}
	return &v
}
