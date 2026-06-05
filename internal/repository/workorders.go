package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"technicien-mobile/internal/models"
)

type WorkOrderFilters struct {
	TechnicianID int
	Status       string
	Priority     string
	Zone         string
	Limit        int
	Offset       int
}

func ListTechnicianWorkOrders(ctx context.Context, db *sql.DB, f WorkOrderFilters) ([]models.MobileWorkOrder, error) {
	if f.Limit <= 0 {
		f.Limit = 50
	}
	query := `
		SELECT wo.id, wo.title, COALESCE(wo.description,''), wo.status, wo.priority,
		       COALESCE(wo.zone,''), wo.created_at, wo.updated_at,
		       wo.accepted_at, wo.started_at, wo.resolved_at, wo.due_date,
		       wo.technician_id, COALESCE(wo.assigned_to_name,''), COALESCE(wo.resolution_note,''),
		       l.id, COALESCE(l.reference,''), COALESCE(l.zone,''), l.latitude, l.longitude,
		       COALESCE(l.etat,'offline'), l.intensite, l.puissance,
		       lcu.id, COALESCE(lcu.reference,''), COALESCE(lcu.ip_address,''), COALESCE(lcu.zone,''),
		       a.id, COALESCE(a.severity,''), COALESCE(a.message,'')
		FROM work_orders wo
		LEFT JOIN lampadaires l   ON l.id = wo.lampadaire_id
		LEFT JOIN lcus lcu        ON lcu.id = l.lcu_id
		LEFT JOIN alerts a        ON a.id = wo.source_alert_id
		WHERE (wo.technician_id = $1 OR wo.assigned_to = $1)
		  AND wo.status NOT IN ('closed','cancelled')`
	args := []any{f.TechnicianID}
	n := 2
	if f.Status != "" {
		query += fmt.Sprintf(" AND wo.status = $%d", n)
		args = append(args, f.Status)
		n++
	}
	if f.Priority != "" {
		query += fmt.Sprintf(" AND wo.priority = $%d", n)
		args = append(args, f.Priority)
		n++
	}
	if f.Zone != "" {
		query += fmt.Sprintf(" AND (wo.zone = $%d OR l.zone = $%d)", n, n)
		args = append(args, f.Zone)
		n++
	}
	query += fmt.Sprintf(`
		ORDER BY
		  CASE wo.priority WHEN 'critical' THEN 1 WHEN 'high' THEN 2 WHEN 'medium' THEN 3 ELSE 4 END,
		  wo.created_at DESC
		LIMIT $%d OFFSET $%d`, n, n+1)
	args = append(args, f.Limit, f.Offset)

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []models.MobileWorkOrder
	for rows.Next() {
		wo, err := scanWorkOrderRow(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, wo)
	}
	return result, nil
}

func GetWorkOrderByID(ctx context.Context, db *sql.DB, id int) (*models.MobileWorkOrder, error) {
	query := `
		SELECT wo.id, wo.title, COALESCE(wo.description,''), wo.status, wo.priority,
		       COALESCE(wo.zone,''), wo.created_at, wo.updated_at,
		       wo.accepted_at, wo.started_at, wo.resolved_at, wo.due_date,
		       wo.technician_id, COALESCE(wo.assigned_to_name,''), COALESCE(wo.resolution_note,''),
		       l.id, COALESCE(l.reference,''), COALESCE(l.zone,''), l.latitude, l.longitude,
		       COALESCE(l.etat,'offline'), l.intensite, l.puissance,
		       lcu.id, COALESCE(lcu.reference,''), COALESCE(lcu.ip_address,''), COALESCE(lcu.zone,''),
		       a.id, COALESCE(a.severity,''), COALESCE(a.message,'')
		FROM work_orders wo
		LEFT JOIN lampadaires l   ON l.id = wo.lampadaire_id
		LEFT JOIN lcus lcu        ON lcu.id = l.lcu_id
		LEFT JOIN alerts a        ON a.id = wo.source_alert_id
		WHERE wo.id = $1`
	rows, err := db.QueryContext(ctx, query, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, nil
	}
	wo, err := scanWorkOrderRow(rows)
	if err != nil {
		return nil, err
	}
	wo.Logs, _ = GetWorkOrderLogs(ctx, db, id)
	wo.Telemetry, _ = GetLatestTelemetryForLampadaire(ctx, db, wo.Lampadaire)
	return &wo, nil
}

func scanWorkOrderRow(rows *sql.Rows) (models.MobileWorkOrder, error) {
	var wo models.MobileWorkOrder
	var lID, lcuID, aID sql.NullInt64
	var lRef, lZone, lEtat sql.NullString
	var lLat, lLng, lPuiss sql.NullFloat64
	var lIntensity sql.NullInt64
	var lcuRef, lcuIP, lcuZone sql.NullString
	var aSev, aMsg sql.NullString
	var techID sql.NullInt64
	err := rows.Scan(
		&wo.ID, &wo.Title, &wo.Description, &wo.Status, &wo.Priority,
		&wo.Zone, &wo.CreatedAt, &wo.UpdatedAt,
		&wo.AcceptedAt, &wo.StartedAt, &wo.ResolvedAt, &wo.DueDate,
		&techID, &wo.AssignedToName, &wo.ResolutionNote,
		&lID, &lRef, &lZone, &lLat, &lLng, &lEtat, &lIntensity, &lPuiss,
		&lcuID, &lcuRef, &lcuIP, &lcuZone,
		&aID, &aSev, &aMsg,
	)
	if err != nil {
		return wo, err
	}
	if techID.Valid {
		v := int(techID.Int64)
		wo.TechnicianID = &v
	}
	if lID.Valid {
		id := int(lID.Int64)
		lamp := &models.MobileLampadaire{
			ID:        &id,
			Reference: lRef.String,
			Zone:      lZone.String,
			Etat:      lEtat.String,
		}
		if lLat.Valid {
			lamp.Latitude = &lLat.Float64
		}
		if lLng.Valid {
			lamp.Longitude = &lLng.Float64
		}
		if lIntensity.Valid {
			lamp.Intensite = int(lIntensity.Int64)
		}
		if lPuiss.Valid {
			lamp.Puissance = &lPuiss.Float64
		}
		wo.Lampadaire = lamp
		if lcuID.Valid {
			id2 := int(lcuID.Int64)
			wo.LCU = &models.MobileLCU{ID: &id2, Reference: lcuRef.String, IPAddress: lcuIP.String, Zone: lcuZone.String}
		}
	}
	if aID.Valid {
		id := int(aID.Int64)
		wo.Alert = &models.MobileAlert{ID: &id, Severity: aSev.String, Message: aMsg.String}
	}
	return wo, nil
}

func GetLatestTelemetryForLampadaire(ctx context.Context, db *sql.DB, lamp *models.MobileLampadaire) (*models.MobileTelemetry, error) {
	if lamp == nil || lamp.ID == nil {
		return nil, nil
	}
	var t models.MobileTelemetry
	var temp, lum, pui, cur, ten sql.NullFloat64
	err := db.QueryRowContext(ctx, `
		SELECT temperature, luminosite, puissance, courant, tension, created_at
		FROM sensor_measurements WHERE lampadaire_id=$1
		ORDER BY created_at DESC LIMIT 1`, *lamp.ID).
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

func GetWorkOrderLogs(ctx context.Context, db *sql.DB, workOrderID int) ([]models.WorkOrderLog, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT id, COALESCE(user_name,''), COALESCE(action,''), COALESCE(note,''),
		       COALESCE(old_status,''), COALESCE(new_status,''), created_at
		FROM work_order_logs WHERE work_order_id=$1
		ORDER BY created_at DESC LIMIT 20`, workOrderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var logs []models.WorkOrderLog
	for rows.Next() {
		var l models.WorkOrderLog
		if err := rows.Scan(&l.ID, &l.UserName, &l.Action, &l.Note, &l.OldStatus, &l.NewStatus, &l.CreatedAt); err != nil {
			continue
		}
		logs = append(logs, l)
	}
	return logs, nil
}

func GetDashboardStats(ctx context.Context, db *sql.DB, techID int) (models.DashboardStats, error) {
	stats := models.DashboardStats{TechnicianID: techID, ServerTime: time.Now()}
	row := db.QueryRowContext(ctx, `
		SELECT
		  COUNT(*) FILTER (WHERE status IN ('created','open','accepted'))    AS assigned_count,
		  COUNT(*) FILTER (WHERE status = 'accepted')                        AS accepted_count,
		  COUNT(*) FILTER (WHERE status = 'in_progress')                     AS in_progress_count,
		  COUNT(*) FILTER (WHERE priority IN ('critical','high') AND status NOT IN ('closed','cancelled','resolved')) AS urgent_count,
		  COUNT(*) FILTER (WHERE status = 'resolved' AND DATE(resolved_at) = CURRENT_DATE) AS resolved_today
		FROM work_orders
		WHERE technician_id = $1 OR assigned_to = $1`, techID)
	_ = row.Scan(&stats.AssignedCount, &stats.AcceptedCount, &stats.InProgressCount, &stats.UrgentCount, &stats.ResolvedToday)
	return stats, nil
}

// UpdateWorkOrderStatus transitions a work order to a new status.
func UpdateWorkOrderStatus(ctx context.Context, db *sql.DB, id, techID int, newStatus, note, resolutionNote string) error {
	now := time.Now()
	var err error
	switch newStatus {
	case "accepted":
		_, err = db.ExecContext(ctx,
			`UPDATE work_orders SET status='accepted', technician_id=$1, accepted_at=$2, updated_at=$2 WHERE id=$3`,
			techID, now, id)
	case "in_progress":
		_, err = db.ExecContext(ctx,
			`UPDATE work_orders SET status='in_progress', started_at=$1, updated_at=$1 WHERE id=$2`,
			now, id)
	case "resolved":
		_, err = db.ExecContext(ctx,
			`UPDATE work_orders SET status='resolved', resolved_at=$1, updated_at=$1, resolution_note=$2 WHERE id=$3`,
			now, resolutionNote, id)
	default:
		_, err = db.ExecContext(ctx,
			`UPDATE work_orders SET status=$1, updated_at=$2 WHERE id=$3`,
			newStatus, now, id)
	}
	return err
}

// InsertWorkOrderLog inserts a log entry for a work order action.
func InsertWorkOrderLog(ctx context.Context, db *sql.DB, workOrderID, techID int, action, note, oldStatus, newStatus string) error {
	techName := fmt.Sprintf("Technicien #%d", techID)
	_, err := db.ExecContext(ctx, `
		INSERT INTO work_order_logs (work_order_id, user_id, user_name, role, action, note, old_status, new_status)
		VALUES ($1, $2, $3, 'technician', $4, $5, $6, $7)`,
		workOrderID, techID, techName, action, note, oldStatus, newStatus)
	return err
}

// GetWorkOrderCurrentStatus returns the current status of a work order.
func GetWorkOrderCurrentStatus(ctx context.Context, db *sql.DB, id int) (string, error) {
	var status string
	err := db.QueryRowContext(ctx, `SELECT status FROM work_orders WHERE id=$1`, id).Scan(&status)
	return status, err
}
