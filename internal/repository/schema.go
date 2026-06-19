package repository

import (
	"database/sql"
	"log"
)

func EnsureMobileSchema(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS mobile_devices (
			id            SERIAL PRIMARY KEY,
			technician_id INT  NOT NULL,
			device_id     TEXT NOT NULL UNIQUE,
			device_name   TEXT,
			platform      TEXT,
			last_sync_at  TIMESTAMPTZ,
			created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS mobile_sync_logs (
			id            SERIAL PRIMARY KEY,
			technician_id INT  NOT NULL,
			device_id     TEXT,
			local_id      TEXT NOT NULL UNIQUE,
			action_type   TEXT NOT NULL,
			entity        TEXT,
			entity_id     INT,
			payload       JSONB,
			server_id     INT,
			status        TEXT NOT NULL,
			error_message TEXT,
			created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_mobile_sync_local_id ON mobile_sync_logs(local_id)`,
		`CREATE INDEX IF NOT EXISTS idx_mobile_sync_technician ON mobile_sync_logs(technician_id)`,
		// Notes terrain saisies par les techniciens (lampadaire / lcu)
		`CREATE TABLE IF NOT EXISTS field_notes (
			id            SERIAL PRIMARY KEY,
			entity_type   TEXT NOT NULL,
			entity_id     INT  NOT NULL,
			technician_id INT  NOT NULL,
			note          TEXT NOT NULL,
			created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_field_notes_entity ON field_notes(entity_type, entity_id)`,
		`CREATE TABLE IF NOT EXISTS work_order_photos (
			id            SERIAL PRIMARY KEY,
			work_order_id INT  NOT NULL,
			technician_id INT  NOT NULL,
			file_path     TEXT NOT NULL,
			created_at    TIMESTAMPTZ DEFAULT NOW()
		)`,
		`CREATE INDEX IF NOT EXISTS idx_wo_photos_wo ON work_order_photos(work_order_id)`,
		`CREATE INDEX IF NOT EXISTS idx_wo_photos_tech ON work_order_photos(technician_id)`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			log.Printf("[schema] warning: %v", err)
		}
	}
	return nil
}
