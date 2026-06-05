package repository

import (
	"context"
	"database/sql"

	"technicien-mobile/internal/models"
)

// ListLCUsForTech returns all LCUs with their lampadaire counts for the technician list.
func ListLCUsForTech(ctx context.Context, db *sql.DB) ([]models.LCUDetail, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT lcu.id, COALESCE(lcu.reference,''), COALESCE(lcu.name,''),
		       COALESCE(lcu.ip_address,''), COALESCE(lcu.port,0), COALESCE(lcu.protocol,''),
		       COALESCE(lcu.zone,''), COALESCE(lcu.address,''), COALESCE(lcu.status,'unknown'),
		       lcu.latitude, lcu.longitude,
		       TO_CHAR(lcu.last_seen_at, 'YYYY-MM-DD"T"HH24:MI:SSZ'),
		       TO_CHAR(lcu.last_sync_at, 'YYYY-MM-DD"T"HH24:MI:SSZ'),
		       (SELECT COUNT(*) FROM lampadaires l WHERE l.lcu_id = lcu.id) AS total,
		       (SELECT COUNT(*) FROM lampadaires l WHERE l.lcu_id = lcu.id AND l.etat='online') AS online,
		       (SELECT COUNT(*) FROM lampadaires l WHERE l.lcu_id = lcu.id AND l.etat='offline') AS offline,
		       (SELECT COUNT(*) FROM lampadaires l WHERE l.lcu_id = lcu.id AND l.etat='maintenance') AS maint
		FROM lcus lcu ORDER BY lcu.reference`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.LCUDetail
	for rows.Next() {
		d, err := scanLCUDetail(rows)
		if err != nil {
			continue
		}
		out = append(out, d)
	}
	return out, nil
}

// GetLCUDetail returns one LCU with its linked lampadaires.
func GetLCUDetail(ctx context.Context, db *sql.DB, id int) (*models.LCUDetail, error) {
	row := db.QueryRowContext(ctx, `
		SELECT lcu.id, COALESCE(lcu.reference,''), COALESCE(lcu.name,''),
		       COALESCE(lcu.ip_address,''), COALESCE(lcu.port,0), COALESCE(lcu.protocol,''),
		       COALESCE(lcu.zone,''), COALESCE(lcu.address,''), COALESCE(lcu.status,'unknown'),
		       lcu.latitude, lcu.longitude,
		       TO_CHAR(lcu.last_seen_at, 'YYYY-MM-DD"T"HH24:MI:SSZ'),
		       TO_CHAR(lcu.last_sync_at, 'YYYY-MM-DD"T"HH24:MI:SSZ'),
		       (SELECT COUNT(*) FROM lampadaires l WHERE l.lcu_id = lcu.id) AS total,
		       (SELECT COUNT(*) FROM lampadaires l WHERE l.lcu_id = lcu.id AND l.etat='online') AS online,
		       (SELECT COUNT(*) FROM lampadaires l WHERE l.lcu_id = lcu.id AND l.etat='offline') AS offline,
		       (SELECT COUNT(*) FROM lampadaires l WHERE l.lcu_id = lcu.id AND l.etat='maintenance') AS maint
		FROM lcus lcu WHERE lcu.id=$1`, id)
	d, err := scanLCUDetailRow(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	d.Lampadaires, _ = GetLCULampadaires(ctx, db, id)
	return &d, nil
}

func GetLCULampadaires(ctx context.Context, db *sql.DB, lcuID int) ([]models.MapLampadaire, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT l.id, COALESCE(l.reference,''), COALESCE(l.zone,''),
		       l.latitude, l.longitude, COALESCE(l.etat,'offline'), l.intensite, l.puissance, l.lcu_id,
		       EXISTS(SELECT 1 FROM alerts a WHERE a.lampadaire_id=l.id AND a.severity='critical' AND a.status='open')
		FROM lampadaires l
		WHERE l.lcu_id=$1 AND l.archived_at IS NULL
		ORDER BY l.reference`, lcuID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanMapLampadaires(rows)
}

// UpdateLCUStatus is used by test/sync actions to refresh status + timestamps.
func UpdateLCUSeen(ctx context.Context, db *sql.DB, id int, status string, sync bool) error {
	if sync {
		_, err := db.ExecContext(ctx, `
			UPDATE lcus SET status=$1, last_seen_at=NOW(), last_sync_at=NOW(), updated_at=NOW() WHERE id=$2`,
			status, id)
		return err
	}
	_, err := db.ExecContext(ctx, `
		UPDATE lcus SET status=$1, last_seen_at=NOW(), updated_at=NOW() WHERE id=$2`,
		status, id)
	return err
}

func GetLCUStatus(ctx context.Context, db *sql.DB, id int) (string, error) {
	var status string
	err := db.QueryRowContext(ctx, `SELECT COALESCE(status,'unknown') FROM lcus WHERE id=$1`, id).Scan(&status)
	return status, err
}

func scanLCUDetail(rows *sql.Rows) (models.LCUDetail, error) {
	var d models.LCUDetail
	var lat, lng sql.NullFloat64
	var lastSeen, lastSync sql.NullString
	err := rows.Scan(&d.ID, &d.Reference, &d.Name, &d.IPAddress, &d.Port, &d.Protocol,
		&d.Zone, &d.Address, &d.Status, &lat, &lng, &lastSeen, &lastSync,
		&d.LampadairesCount, &d.OnlineCount, &d.OfflineCount, &d.MaintenanceCount)
	if err != nil {
		return d, err
	}
	if lat.Valid { d.Latitude = &lat.Float64 }
	if lng.Valid { d.Longitude = &lng.Float64 }
	if lastSeen.Valid { d.LastSeenAt = &lastSeen.String }
	if lastSync.Valid { d.LastSyncAt = &lastSync.String }
	return d, nil
}

func scanLCUDetailRow(row *sql.Row) (models.LCUDetail, error) {
	var d models.LCUDetail
	var lat, lng sql.NullFloat64
	var lastSeen, lastSync sql.NullString
	err := row.Scan(&d.ID, &d.Reference, &d.Name, &d.IPAddress, &d.Port, &d.Protocol,
		&d.Zone, &d.Address, &d.Status, &lat, &lng, &lastSeen, &lastSync,
		&d.LampadairesCount, &d.OnlineCount, &d.OfflineCount, &d.MaintenanceCount)
	if err != nil {
		return d, err
	}
	if lat.Valid { d.Latitude = &lat.Float64 }
	if lng.Valid { d.Longitude = &lng.Float64 }
	if lastSeen.Valid { d.LastSeenAt = &lastSeen.String }
	if lastSync.Valid { d.LastSyncAt = &lastSync.String }
	return d, nil
}
