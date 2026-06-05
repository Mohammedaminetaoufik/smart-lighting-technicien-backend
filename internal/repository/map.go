package repository

import (
	"context"
	"database/sql"
	"fmt"
	"math"

	"technicien-mobile/internal/models"
)

func GetMapLampadaires(ctx context.Context, db *sql.DB, zone, etat string, lcuID int) ([]models.MapLampadaire, error) {
	query := `
		SELECT l.id, COALESCE(l.reference,''), COALESCE(l.zone,''),
		       l.latitude, l.longitude, COALESCE(l.etat,'offline'), l.intensite, l.puissance, l.lcu_id,
		       EXISTS(SELECT 1 FROM alerts a WHERE a.lampadaire_id=l.id AND a.severity='critical' AND a.status='open') AS has_critical
		FROM lampadaires l
		WHERE l.archived_at IS NULL AND l.latitude IS NOT NULL AND l.longitude IS NOT NULL`
	var args []any
	n := 1
	if zone != "" {
		query += fmt.Sprintf(" AND l.zone=$%d", n)
		args = append(args, zone)
		n++
	}
	if etat != "" {
		query += fmt.Sprintf(" AND l.etat=$%d", n)
		args = append(args, etat)
		n++
	}
	if lcuID > 0 {
		query += fmt.Sprintf(" AND l.lcu_id=$%d", n)
		args = append(args, lcuID)
	}
	query += " ORDER BY l.reference LIMIT 1000"
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanMapLampadaires(rows)
}

func GetMapLCUs(ctx context.Context, db *sql.DB) ([]models.MapLCU, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT id, COALESCE(reference,''), COALESCE(name,''), COALESCE(ip_address,''),
		       COALESCE(zone,''), latitude, longitude, COALESCE(status,'unknown')
		FROM lcus ORDER BY reference`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []models.MapLCU
	for rows.Next() {
		var l models.MapLCU
		var lat, lng sql.NullFloat64
		if err := rows.Scan(&l.ID, &l.Reference, &l.Name, &l.IPAddress, &l.Zone, &lat, &lng, &l.Status); err != nil {
			continue
		}
		if lat.Valid { l.Latitude = &lat.Float64 }
		if lng.Valid { l.Longitude = &lng.Float64 }
		result = append(result, l)
	}
	return result, nil
}

func GetMapConnections(ctx context.Context, db *sql.DB, lcuID int) ([]models.MapConnection, error) {
	query := `SELECT lcu_id, id FROM lampadaires WHERE lcu_id IS NOT NULL AND latitude IS NOT NULL`
	var args []any
	if lcuID > 0 {
		query += " AND lcu_id=$1"
		args = append(args, lcuID)
	}
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []models.MapConnection
	for rows.Next() {
		var c models.MapConnection
		if err := rows.Scan(&c.LCUID, &c.LampadaireID); err == nil {
			result = append(result, c)
		}
	}
	return result, nil
}

func GetTechnicianAssignedLampadaires(ctx context.Context, db *sql.DB, techID int) ([]models.MapLampadaire, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT DISTINCT l.id, COALESCE(l.reference,''), COALESCE(l.zone,''),
		       l.latitude, l.longitude, COALESCE(l.etat,'offline'), l.intensite, l.puissance, l.lcu_id,
		       EXISTS(SELECT 1 FROM alerts a WHERE a.lampadaire_id=l.id AND a.severity='critical' AND a.status='open')
		FROM lampadaires l
		JOIN work_orders wo ON wo.lampadaire_id = l.id
		WHERE (wo.technician_id=$1 OR wo.assigned_to=$1)
		  AND wo.status NOT IN ('closed','cancelled','resolved')
		  AND l.latitude IS NOT NULL`, techID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanMapLampadaires(rows)
}

// GetNearbyLampadaires uses Haversine formula in Go (no PostGIS required).
func GetNearbyLampadaires(ctx context.Context, db *sql.DB, lat, lng, radiusM float64) ([]models.MapLampadaire, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT l.id, COALESCE(l.reference,''), COALESCE(l.zone,''),
		       l.latitude, l.longitude, COALESCE(l.etat,'offline'), l.intensite, l.puissance, l.lcu_id,
		       EXISTS(SELECT 1 FROM alerts a WHERE a.lampadaire_id=l.id AND a.severity='critical' AND a.status='open')
		FROM lampadaires l
		WHERE l.archived_at IS NULL AND l.latitude IS NOT NULL AND l.longitude IS NOT NULL
		LIMIT 500`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	all, err := scanMapLampadaires(rows)
	if err != nil {
		return nil, err
	}
	var nearby []models.MapLampadaire
	for _, l := range all {
		if l.Latitude == nil || l.Longitude == nil {
			continue
		}
		if haversineM(lat, lng, *l.Latitude, *l.Longitude) <= radiusM {
			nearby = append(nearby, l)
		}
	}
	return nearby, nil
}

func haversineM(lat1, lng1, lat2, lng2 float64) float64 {
	const R = 6371000.0
	dLat := (lat2 - lat1) * math.Pi / 180
	dLng := (lng2 - lng1) * math.Pi / 180
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1*math.Pi/180)*math.Cos(lat2*math.Pi/180)*
			math.Sin(dLng/2)*math.Sin(dLng/2)
	return R * 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
}

func scanMapLampadaires(rows *sql.Rows) ([]models.MapLampadaire, error) {
	var result []models.MapLampadaire
	for rows.Next() {
		var l models.MapLampadaire
		var lat, lng, pui sql.NullFloat64
		var lcuID sql.NullInt64
		if err := rows.Scan(&l.ID, &l.Reference, &l.Zone, &lat, &lng, &l.Etat, &l.Intensite, &pui, &lcuID, &l.HasCriticalAlert); err != nil {
			continue
		}
		if lat.Valid { l.Latitude = &lat.Float64 }
		if lng.Valid { l.Longitude = &lng.Float64 }
		if pui.Valid { l.Puissance = &pui.Float64 }
		if lcuID.Valid { v := int(lcuID.Int64); l.LCUID = &v }
		result = append(result, l)
	}
	return result, nil
}
