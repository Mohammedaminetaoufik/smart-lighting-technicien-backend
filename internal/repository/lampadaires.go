package repository

import (
	"context"
	"database/sql"
	"strconv"
	"strings"

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

// ListLampadaires returns lampadaires for the technician list view (with filters).
func ListLampadairesForTech(ctx context.Context, db *sql.DB, zone, etat, search string, limit int) ([]models.MapLampadaire, error) {
	if limit <= 0 {
		limit = 200
	}
	query := `
		SELECT l.id, COALESCE(l.reference,''), COALESCE(l.zone,''),
		       l.latitude, l.longitude, COALESCE(l.etat,'offline'), l.intensite, l.puissance, l.lcu_id,
		       EXISTS(SELECT 1 FROM alerts a WHERE a.lampadaire_id=l.id AND a.severity='critical' AND a.status='open')
		FROM lampadaires l
		WHERE l.archived_at IS NULL`
	var args []any
	n := 1
	if zone != "" {
		query += " AND l.zone=$" + strconv.Itoa(n); args = append(args, zone); n++
	}
	if etat != "" {
		query += " AND l.etat=$" + strconv.Itoa(n); args = append(args, etat); n++
	}
	if search != "" {
		query += " AND (LOWER(l.reference) LIKE $" + strconv.Itoa(n) + " OR LOWER(l.zone) LIKE $" + strconv.Itoa(n) + ")"
		args = append(args, "%"+strings.ToLower(search)+"%"); n++
	}
	query += " ORDER BY l.reference LIMIT $" + strconv.Itoa(n)
	args = append(args, limit)

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanMapLampadaires(rows)
}

// GetLampadaireDetail returns the full technician detail view of a lampadaire.
func GetLampadaireDetail(ctx context.Context, db *sql.DB, id, techID int) (*models.LampadaireDetail, error) {
	var d models.LampadaireDetail
	var puiss sql.NullFloat64
	var nominalW sql.NullInt64
	var lat, lng sql.NullFloat64
	var lastSeen, lastCmd sql.NullString
	var lcuID sql.NullInt64
	var lcuRef, lcuIP, lcuZone sql.NullString

	err := db.QueryRowContext(ctx, `
		SELECT l.id, COALESCE(l.reference,''), COALESCE(l.zone,''), COALESCE(l.etat,'offline'),
		       l.intensite, l.puissance, l.nominal_power_w,
		       COALESCE(l.protocole,''), COALESCE(l.type_driver,''),
		       COALESCE(l.driver_brand,''), COALESCE(l.driver_model,''),
		       COALESCE(l.commissioning_status,'discovered'),
		       l.latitude, l.longitude, COALESCE(l.location_status,'manual'),
		       COALESCE(l.address,''), COALESCE(l.quartier,''), COALESCE(l.notes,''),
		       TO_CHAR(l.last_seen_at, 'YYYY-MM-DD"T"HH24:MI:SSZ'),
		       TO_CHAR(l.last_command_at, 'YYYY-MM-DD"T"HH24:MI:SSZ'),
		       lcu.id, COALESCE(lcu.reference,''), COALESCE(lcu.ip_address,''), COALESCE(lcu.zone,'')
		FROM lampadaires l
		LEFT JOIN lcus lcu ON lcu.id = l.lcu_id
		WHERE l.id=$1`, id).
		Scan(&d.ID, &d.Reference, &d.Zone, &d.Etat,
			&d.Intensite, &puiss, &nominalW,
			&d.Protocole, &d.TypeDriver, &d.DriverBrand, &d.DriverModel,
			&d.CommissioningStatus,
			&lat, &lng, &d.LocationStatus,
			&d.Address, &d.Quartier, &d.Notes,
			&lastSeen, &lastCmd,
			&lcuID, &lcuRef, &lcuIP, &lcuZone)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if puiss.Valid    { d.Puissance = &puiss.Float64 }
	if nominalW.Valid { v := int(nominalW.Int64); d.NominalPowerW = &v }
	if lat.Valid      { d.Latitude = &lat.Float64 }
	if lng.Valid      { d.Longitude = &lng.Float64 }
	if lastSeen.Valid { d.LastSeenAt = &lastSeen.String }
	if lastCmd.Valid  { d.LastCommandAt = &lastCmd.String }
	if lcuID.Valid {
		v := int(lcuID.Int64)
		d.LCU = &models.MobileLCU{ID: &v, Reference: lcuRef.String, IPAddress: lcuIP.String, Zone: lcuZone.String}
	}

	d.Telemetry, _ = GetLatestTelemetryByLampadaireID(ctx, db, id)
	d.OpenAlerts, _ = GetLampadaireOpenAlerts(ctx, db, id)
	d.WorkOrders, _ = GetLampadaireWorkOrders(ctx, db, id)
	// assigned_to_me : un bon de travail actif assigné à ce technicien
	for _, wo := range d.WorkOrders {
		if wo.TechnicianID != nil && *wo.TechnicianID == techID &&
			wo.Status != "closed" && wo.Status != "cancelled" {
			d.AssignedToMe = true
			break
		}
	}
	return &d, nil
}

func GetLampadaireOpenAlerts(ctx context.Context, db *sql.DB, id int) ([]models.DiagnosticAlert, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT id, COALESCE(severity,'info'), COALESCE(message,''), COALESCE(status,'open'), created_at
		FROM alerts WHERE lampadaire_id=$1 AND status IN ('open','acknowledged','in_progress')
		ORDER BY created_at DESC LIMIT 20`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.DiagnosticAlert
	for rows.Next() {
		var a models.DiagnosticAlert
		if err := rows.Scan(&a.ID, &a.Severity, &a.Message, &a.Status, &a.CreatedAt); err == nil {
			out = append(out, a)
		}
	}
	return out, nil
}

func GetLampadaireWorkOrders(ctx context.Context, db *sql.DB, lampID int) ([]models.MobileWorkOrder, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT id, title, COALESCE(description,''), status, priority,
		       COALESCE(zone,''), created_at, updated_at, technician_id
		FROM work_orders WHERE lampadaire_id=$1
		ORDER BY created_at DESC LIMIT 20`, lampID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.MobileWorkOrder
	for rows.Next() {
		var wo models.MobileWorkOrder
		var techID sql.NullInt64
		if err := rows.Scan(&wo.ID, &wo.Title, &wo.Description, &wo.Status, &wo.Priority,
			&wo.Zone, &wo.CreatedAt, &wo.UpdatedAt, &techID); err != nil {
			continue
		}
		if techID.Valid { v := int(techID.Int64); wo.TechnicianID = &v }
		out = append(out, wo)
	}
	return out, nil
}

// IsLampadaireLinkedToTech returns true if the lampadaire is tied to one of the
// technician's active work orders, or is a commissioning task (location update allowed).
func IsLampadaireLinkedToTech(ctx context.Context, db *sql.DB, lampID, techID int) (bool, error) {
	var cnt int
	err := db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM work_orders
		WHERE lampadaire_id=$1 AND (technician_id=$2 OR assigned_to=$2)
		  AND status NOT IN ('closed','cancelled')`, lampID, techID).Scan(&cnt)
	if err != nil {
		return false, err
	}
	if cnt > 0 {
		return true, nil
	}
	// Autorisé aussi pendant la mise en service (statut non finalisé)
	var commStatus string
	_ = db.QueryRowContext(ctx, `SELECT COALESCE(commissioning_status,'discovered') FROM lampadaires WHERE id=$1`, lampID).Scan(&commStatus)
	if commStatus != "commissioned" {
		return true, nil
	}
	return false, nil
}

func InsertFieldNote(ctx context.Context, db *sql.DB, entityType string, entityID, techID int, note string) (int, error) {
	var id int
	err := db.QueryRowContext(ctx, `
		INSERT INTO field_notes (entity_type, entity_id, technician_id, note)
		VALUES ($1,$2,$3,$4) RETURNING id`, entityType, entityID, techID, note).Scan(&id)
	return id, err
}

func GetFieldNotes(ctx context.Context, db *sql.DB, entityType string, entityID int) ([]models.FieldNote, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT id, entity_type, entity_id, technician_id, note, created_at
		FROM field_notes WHERE entity_type=$1 AND entity_id=$2
		ORDER BY created_at DESC LIMIT 30`, entityType, entityID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.FieldNote
	for rows.Next() {
		var fn models.FieldNote
		if err := rows.Scan(&fn.ID, &fn.EntityType, &fn.EntityID, &fn.TechID, &fn.Note, &fn.CreatedAt); err == nil {
			out = append(out, fn)
		}
	}
	return out, nil
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
