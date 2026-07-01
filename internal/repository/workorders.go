package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"technicien-mobile/internal/models"
)

type WorkOrderFilters struct {
	TechnicianID int
	// Scope controls which work orders are returned:
	//   "mine"      → assigned to this technician (technician_id or assigned_to)
	//   "available" → unassigned and still open/created (pool to pick up)
	//   "all" or "" → every active work order (default; lets technicians take any)
	Scope    string
	Status   string
	Priority string
	Zone     string
	Limit    int
	Offset   int
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
		       a.id, COALESCE(a.severity,''), COALESCE(a.message,''),
		       wo.source_alert_id, wo.maintenance_window_id
		FROM work_orders wo
		LEFT JOIN lampadaires l   ON l.id = wo.lampadaire_id
		LEFT JOIN lcus lcu        ON lcu.id = l.lcu_id
		LEFT JOIN alerts a        ON a.id = wo.source_alert_id`

	conds := []string{"wo.status NOT IN ('closed','cancelled')"}
	args := []any{}
	n := 1

	switch f.Scope {
	case "mine":
		conds = append(conds, fmt.Sprintf("(wo.technician_id = $%d OR wo.assigned_to = $%d)", n, n))
		args = append(args, f.TechnicianID)
		n++
	case "available":
		conds = append(conds, "wo.technician_id IS NULL AND wo.assigned_to IS NULL AND wo.status IN ('created','open')")
	default:
		// "all" (or empty): no assignment restriction — every active work order is visible.
	}

	if f.Status != "" {
		conds = append(conds, fmt.Sprintf("wo.status = $%d", n))
		args = append(args, f.Status)
		n++
	}
	if f.Priority != "" {
		conds = append(conds, fmt.Sprintf("wo.priority = $%d", n))
		args = append(args, f.Priority)
		n++
	}
	if f.Zone != "" {
		conds = append(conds, fmt.Sprintf("(wo.zone = $%d OR l.zone = $%d)", n, n))
		args = append(args, f.Zone)
		n++
	}

	query += "\n\t\tWHERE " + strings.Join(conds, "\n\t\t  AND ")
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
		       a.id, COALESCE(a.severity,''), COALESCE(a.message,''),
		       wo.source_alert_id, wo.maintenance_window_id
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
	wo.MaintenanceWindow, _ = getMaintenanceWindowForWO(ctx, db, wo.MaintenanceWindowID)
	return &wo, nil
}

// getMaintenanceWindowForWO fetches the linked maintenance window (detail view only).
func getMaintenanceWindowForWO(ctx context.Context, db *sql.DB, mwID *int) (*models.MobileMaintenanceWindow, error) {
	if mwID == nil {
		return nil, nil
	}
	var mw models.MobileMaintenanceWindow
	var reason sql.NullString
	err := db.QueryRowContext(ctx, `
		SELECT id, COALESCE(title,''), COALESCE(status,'planned'),
		       COALESCE(impact_level,'low'), start_at, end_at, COALESCE(reason,'')
		FROM maintenance_windows WHERE id=$1`, *mwID).
		Scan(&mw.ID, &mw.Title, &mw.Status, &mw.ImpactLevel, &mw.StartAt, &mw.EndAt, &reason)
	if err != nil {
		return nil, err
	}
	mw.Reason = reason.String
	return &mw, nil
}

func scanWorkOrderRow(rows *sql.Rows) (models.MobileWorkOrder, error) {
	var wo models.MobileWorkOrder
	var lID, lcuID, aID sql.NullInt64
	var lRef, lZone, lEtat sql.NullString
	var lLat, lLng, lPuiss sql.NullFloat64
	var lIntensity sql.NullInt64
	var lcuRef, lcuIP, lcuZone sql.NullString
	var aSev, aMsg sql.NullString
	var techID, sourceAlertID, mwID sql.NullInt64
	err := rows.Scan(
		&wo.ID, &wo.Title, &wo.Description, &wo.Status, &wo.Priority,
		&wo.Zone, &wo.CreatedAt, &wo.UpdatedAt,
		&wo.AcceptedAt, &wo.StartedAt, &wo.ResolvedAt, &wo.DueDate,
		&techID, &wo.AssignedToName, &wo.ResolutionNote,
		&lID, &lRef, &lZone, &lLat, &lLng, &lEtat, &lIntensity, &lPuiss,
		&lcuID, &lcuRef, &lcuIP, &lcuZone,
		&aID, &aSev, &aMsg,
		&sourceAlertID, &mwID,
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
	if sourceAlertID.Valid {
		v := int(sourceAlertID.Int64)
		wo.SourceAlertID = &v
	}
	if mwID.Valid {
		v := int(mwID.Int64)
		wo.MaintenanceWindowID = &v
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

// UpdateWorkOrderStatus transitions a work order to a new status and syncs the linked maintenance window.
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
	if err != nil {
		return err
	}
	syncMobileMaintenanceStatus(ctx, db, id, newStatus)
	return nil
}

// syncMobileMaintenanceStatus propagates WO status changes to the linked maintenance window.
func syncMobileMaintenanceStatus(ctx context.Context, db *sql.DB, workOrderID int, newStatus string) {
	var mwID sql.NullInt64
	_ = db.QueryRowContext(ctx, `SELECT maintenance_window_id FROM work_orders WHERE id=$1`, workOrderID).Scan(&mwID)
	if !mwID.Valid {
		return
	}
	id := mwID.Int64
	switch newStatus {
	case "in_progress":
		_, _ = db.ExecContext(ctx, `UPDATE maintenance_windows SET status='in_progress', updated_at=NOW()
			WHERE id=$1 AND status='planned'`, id)
	case "resolved", "closed":
		var pending int
		_ = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM work_orders
			WHERE maintenance_window_id=$1 AND status NOT IN ('resolved','closed','cancelled')`, id).Scan(&pending)
		if pending == 0 {
			_, _ = db.ExecContext(ctx, `UPDATE maintenance_windows SET status='completed', completed_at=NOW(), updated_at=NOW()
				WHERE id=$1 AND status NOT IN ('completed','cancelled')`, id)
		}
	}
}

// InsertWorkOrderLog inserts a log entry for a work order action.
func InsertWorkOrderLog(ctx context.Context, db *sql.DB, workOrderID, techID int, techName, action, note, oldStatus, newStatus string) error {
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

// GetTechnicianDashboard builds the enriched Phase-2 dashboard payload.
func GetTechnicianDashboard(ctx context.Context, db *sql.DB, techID int) (models.TechnicianDashboard, error) {
	d := models.TechnicianDashboard{
		TechnicianID: techID,
		ServerTime:   time.Now().UTC().Format(time.RFC3339),
	}

	// Stats block
	_ = db.QueryRowContext(ctx, `
		SELECT
		  COUNT(*) FILTER (WHERE status IN ('created','open','accepted'))    AS assigned,
		  COUNT(*) FILTER (WHERE priority IN ('critical','high') AND status NOT IN ('closed','cancelled','resolved')) AS urgent,
		  COUNT(*) FILTER (WHERE status = 'in_progress')                     AS in_progress,
		  COUNT(*) FILTER (WHERE status = 'resolved' AND DATE(resolved_at) = CURRENT_DATE) AS completed_today
		FROM work_orders
		WHERE technician_id = $1 OR assigned_to = $1`, techID).
		Scan(&d.Stats.Assigned, &d.Stats.Urgent, &d.Stats.InProgress, &d.Stats.CompletedToday)

	// Next work order (prochaine intervention prioritaire)
	next, _ := ListTechnicianWorkOrders(ctx, db, WorkOrderFilters{TechnicianID: techID, Scope: "mine", Limit: 1})
	if len(next) > 0 {
		d.NextWorkOrder = &next[0]
	}

	// Map summary
	_ = db.QueryRowContext(ctx, `
		SELECT
		  (SELECT COUNT(DISTINCT l.id) FROM lampadaires l
		     JOIN work_orders wo ON wo.lampadaire_id = l.id
		     WHERE (wo.technician_id=$1 OR wo.assigned_to=$1) AND wo.status NOT IN ('closed','cancelled','resolved')),
		  (SELECT COUNT(*) FROM lcus),
		  (SELECT COUNT(*) FROM alerts WHERE severity='critical' AND status='open'),
		  (SELECT COUNT(*) FROM lampadaires WHERE (latitude IS NULL OR longitude IS NULL) AND archived_at IS NULL)
		`, techID).
		Scan(&d.MapSummary.AssignedLampadaires, &d.MapSummary.NearbyLCUs,
			&d.MapSummary.CriticalAlerts, &d.MapSummary.MissingLocation)

	// Important alerts (critiques liées aux lampadaires du technicien sinon globales)
	rows, _ := db.QueryContext(ctx, `
		SELECT a.id, COALESCE(a.severity,'info'), COALESCE(a.message,''),
		       COALESCE(l.reference,''), COALESCE(l.zone,'')
		FROM alerts a
		LEFT JOIN lampadaires l ON l.id = a.lampadaire_id
		WHERE a.status='open' AND a.severity IN ('critical','major')
		ORDER BY a.created_at DESC LIMIT 5`)
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var al models.DashboardAlert
			if err := rows.Scan(&al.ID, &al.Severity, &al.Message, &al.LampadaireReference, &al.Zone); err == nil {
				d.ImportantAlerts = append(d.ImportantAlerts, al)
			}
		}
	}
	if d.ImportantAlerts == nil {
		d.ImportantAlerts = []models.DashboardAlert{}
	}

	// Sync info
	var lastSync sql.NullString
	_ = db.QueryRowContext(ctx, `
		SELECT TO_CHAR(MAX(last_sync_at), 'YYYY-MM-DD"T"HH24:MI:SSZ')
		FROM mobile_devices WHERE technician_id=$1`, techID).Scan(&lastSync)
	if lastSync.Valid {
		d.Sync.LastSyncAt = &lastSync.String
	}
	d.Sync.OfflineAvailable = true
	d.Sync.PendingActions = 0 // côté serveur on ne connaît pas la file locale

	return d, nil
}
