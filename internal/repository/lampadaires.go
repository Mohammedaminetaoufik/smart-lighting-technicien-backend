package repository

import (
	"context"
	"database/sql"

	"technicien-mobile/internal/models"
)

func GetLampadaireDiagnostic(ctx context.Context, db *sql.DB, id int) (*models.DiagnosticResult, error) {
	var d models.DiagnosticResult
	var lastSeen sql.NullString
	var puiss sql.NullFloat64
	var lcuID sql.NullInt64
	var lcuRef, lcuIP sql.NullString

	err := db.QueryRowContext(ctx, `
		SELECT l.id, COALESCE(l.reference,''), COALESCE(l.zone,''), COALESCE(l.etat,'offline'),
		       l.intensite, l.puissance, COALESCE(l.commissioning_status,'discovered'),
		       TO_CHAR(l.last_seen_at, 'YYYY-MM-DD"T"HH24:MI:SSZ'),
		       lcu.id, COALESCE(lcu.reference,''), COALESCE(lcu.ip_address,'')
		FROM lampadaires l
		LEFT JOIN lcus lcu ON lcu.id = l.lcu_id
		WHERE l.id = $1`, id).
		Scan(
			&d.Lampadaire.ID, &d.Lampadaire.Reference, &d.Lampadaire.Zone,
			&d.Lampadaire.Etat, &d.Lampadaire.Intensite, &puiss,
			&d.Lampadaire.CommissioningStatus, &lastSeen,
			&lcuID, &lcuRef, &lcuIP,
		)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if puiss.Valid {
		d.Lampadaire.Puissance = &puiss.Float64
	}
	if lastSeen.Valid {
		d.Lampadaire.LastSeenAt = &lastSeen.String
	}
	if lcuID.Valid {
		lcuIDInt := int(lcuID.Int64)
		d.LCU = &models.MobileLCU{ID: &lcuIDInt, Reference: lcuRef.String, IPAddress: lcuIP.String}
	}

	// Latest telemetry
	var t models.MobileTelemetry
	var temp, lum, pui, cur, ten sql.NullFloat64
	err2 := db.QueryRowContext(ctx, `
		SELECT temperature, luminosite, puissance, courant, tension, created_at
		FROM sensor_measurements WHERE lampadaire_id=$1
		ORDER BY created_at DESC LIMIT 1`, id).
		Scan(&temp, &lum, &pui, &cur, &ten, &t.CreatedAt)
	if err2 == nil {
		if temp.Valid { t.Temperature = &temp.Float64 }
		if lum.Valid  { t.Luminosite  = &lum.Float64 }
		if pui.Valid  { t.Puissance   = &pui.Float64 }
		if cur.Valid  { t.Courant     = &cur.Float64 }
		if ten.Valid  { t.Tension     = &ten.Float64 }
		d.Telemetry = &t
	}

	// Open alerts
	rows, _ := db.QueryContext(ctx, `
		SELECT id, COALESCE(severity,'info'), COALESCE(message,''), COALESCE(status,'open'), created_at
		FROM alerts WHERE lampadaire_id=$1 AND status IN ('open','acknowledged','in_progress')
		ORDER BY created_at DESC LIMIT 10`, id)
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var a models.DiagnosticAlert
			if err := rows.Scan(&a.ID, &a.Severity, &a.Message, &a.Status, &a.CreatedAt); err == nil {
				d.OpenAlerts = append(d.OpenAlerts, a)
			}
		}
	}
	d.HasActiveAlerts = len(d.OpenAlerts) > 0

	return &d, nil
}

func GetLatestTelemetryByLampadaireID(ctx context.Context, db *sql.DB, id int) (*models.MobileTelemetry, error) {
	var t models.MobileTelemetry
	var temp, lum, pui, cur, ten sql.NullFloat64
	err := db.QueryRowContext(ctx, `
		SELECT temperature, luminosite, puissance, courant, tension, created_at
		FROM sensor_measurements WHERE lampadaire_id=$1
		ORDER BY created_at DESC LIMIT 1`, id).
		Scan(&temp, &lum, &pui, &cur, &ten, &t.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if temp.Valid { t.Temperature = &temp.Float64 }
	if lum.Valid  { t.Luminosite  = &lum.Float64 }
	if pui.Valid  { t.Puissance   = &pui.Float64 }
	if cur.Valid  { t.Courant     = &cur.Float64 }
	if ten.Valid  { t.Tension     = &ten.Float64 }
	return &t, nil
}

func UpdateLampadaireLocation(ctx context.Context, db *sql.DB, id int, lat, lng float64, source string) error {
	_, err := db.ExecContext(ctx, `
		UPDATE lampadaires
		SET latitude=$1, longitude=$2, location_status='confirmed', updated_at=NOW()
		WHERE id=$3`, lat, lng, id)
	return err
}

func GetMissingLocationLampadaires(ctx context.Context, db *sql.DB) ([]models.MapLampadaire, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT id, COALESCE(reference,''), COALESCE(zone,''), COALESCE(etat,'offline'), intensite
		FROM lampadaires
		WHERE (latitude IS NULL OR longitude IS NULL) AND archived_at IS NULL
		ORDER BY reference LIMIT 100`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []models.MapLampadaire
	for rows.Next() {
		var l models.MapLampadaire
		if err := rows.Scan(&l.ID, &l.Reference, &l.Zone, &l.Etat, &l.Intensite); err != nil {
			continue
		}
		result = append(result, l)
	}
	return result, nil
}
