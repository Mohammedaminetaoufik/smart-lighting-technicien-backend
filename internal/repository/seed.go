package repository

import (
	"context"
	"database/sql"
	"log"
	"time"
)

// SeedTestDataIfEmpty creates LCUs, alerts, and work orders if the tables are empty.
// Safe to call on every startup — all inserts are guarded by COUNT checks.
func SeedTestDataIfEmpty(db *sql.DB) {
	ctx := context.Background()

	if err := seedLCUs(ctx, db); err != nil {
		log.Printf("[seed] LCUs: %v", err)
	}
	if err := seedAlerts(ctx, db); err != nil {
		log.Printf("[seed] alerts: %v", err)
	}
	if err := seedWorkOrders(ctx, db); err != nil {
		log.Printf("[seed] work_orders: %v", err)
	}
}

// ── LCUs ────────────────────────────────────────────────────────────────────

func seedLCUs(ctx context.Context, db *sql.DB) error {
	var n int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM lcus").Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return nil
	}

	log.Println("[seed] Creating test LCUs...")

	lcus := []struct {
		ref, name, ip, zone, protocol string
		lat, lng                       float64
	}{
		{"LCU-NORD-01", "Contrôleur Zone Nord", "192.168.1.101", "Zone Nord", "HTTP", 33.5800, -7.5850},
		{"LCU-CENTRE-01", "Contrôleur Zone Centre", "192.168.1.102", "Zone Centre", "HTTP", 33.5731, -7.5898},
		{"LCU-SUD-01", "Contrôleur Zone Sud", "192.168.1.103", "Zone Sud", "HTTP", 33.5650, -7.5950},
	}

	var ids [3]int64
	for i, lcu := range lcus {
		err := db.QueryRowContext(ctx, `
			INSERT INTO lcus (reference, name, ip_address, port, protocol, zone, latitude, longitude, status)
			VALUES ($1, $2, $3, 8080, $4, $5, $6, $7, 'online')
			RETURNING id`,
			lcu.ref, lcu.name, lcu.ip, lcu.protocol, lcu.zone, lcu.lat, lcu.lng,
		).Scan(&ids[i])
		if err != nil {
			return err
		}
	}

	// Link the 15 lampadaires to LCUs (5 per LCU)
	rows, err := db.QueryContext(ctx, `
		SELECT id FROM lampadaires WHERE archived_at IS NULL ORDER BY id LIMIT 15`)
	if err != nil {
		return err
	}
	defer rows.Close()

	lampIDs := make([]int64, 0, 15)
	for rows.Next() {
		var id int64
		if rows.Scan(&id) == nil {
			lampIDs = append(lampIDs, id)
		}
	}

	for i, lampID := range lampIDs {
		lcuIdx := i / 5 // 0→LCU0, 1→LCU1, 2→LCU2
		if lcuIdx > 2 {
			lcuIdx = 2
		}
		_, _ = db.ExecContext(ctx, `
			UPDATE lampadaires SET lcu_id=$1 WHERE id=$2`,
			ids[lcuIdx], lampID)
	}

	log.Println("[seed] 3 LCUs created and lampadaires linked.")
	return nil
}

// ── Alerts ──────────────────────────────────────────────────────────────────

func seedAlerts(ctx context.Context, db *sql.DB) error {
	var n int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM alerts").Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return nil
	}

	log.Println("[seed] Creating test alerts...")

	// Get first 5 lampadaire IDs
	rows, err := db.QueryContext(ctx, `
		SELECT id FROM lampadaires WHERE archived_at IS NULL ORDER BY id LIMIT 5`)
	if err != nil {
		return err
	}
	defer rows.Close()

	var lampIDs []int64
	for rows.Next() {
		var id int64
		if rows.Scan(&id) == nil {
			lampIDs = append(lampIDs, id)
		}
	}

	alerts := []struct {
		typ, severity, message, status string
	}{
		{"lamp_failure", "critical", "Lampe hors service — courant 0 mA détecté", "open"},
		{"overtemperature", "major", "Température driver > 85°C — surchauffe critique", "open"},
		{"day_burn", "major", "Allumage parasite détecté en plein jour (day burner)", "open"},
		{"communication_lost", "warning", "Perte de communication depuis plus de 2 heures", "acknowledged"},
		{"power_anomaly", "warning", "Surconsommation détectée : +25% au-dessus du nominal", "open"},
	}

	var alertIDs []int64
	for i, a := range alerts {
		lampID := sql.NullInt64{}
		if i < len(lampIDs) {
			lampID = sql.NullInt64{Int64: lampIDs[i], Valid: true}
		}
		var id int64
		err := db.QueryRowContext(ctx, `
			INSERT INTO alerts (lampadaire_id, type, severity, message, status)
			VALUES ($1, $2, $3, $4, $5) RETURNING id`,
			lampID, a.typ, a.severity, a.message, a.status,
		).Scan(&id)
		if err != nil {
			log.Printf("[seed] alert insert: %v", err)
			continue
		}
		alertIDs = append(alertIDs, id)
	}

	log.Printf("[seed] %d alerts created.", len(alertIDs))
	return nil
}

// ── Work Orders ──────────────────────────────────────────────────────────────

func seedWorkOrders(ctx context.Context, db *sql.DB) error {
	var n int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM work_orders").Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return nil
	}

	log.Println("[seed] Creating test work orders...")

	// Get lampadaire IDs
	rows, err := db.QueryContext(ctx, `
		SELECT id, zone FROM lampadaires WHERE archived_at IS NULL ORDER BY id LIMIT 6`)
	if err != nil {
		return err
	}
	defer rows.Close()

	type lamp struct{ id int64; zone string }
	var lamps []lamp
	for rows.Next() {
		var l lamp
		if rows.Scan(&l.id, &l.zone) == nil {
			lamps = append(lamps, l)
		}
	}

	// Get first alert ID for source_alert_id
	var firstAlertID sql.NullInt64
	_ = db.QueryRowContext(ctx, `SELECT id FROM alerts ORDER BY id LIMIT 1`).Scan(&firstAlertID)

	now := time.Now()
	due3Days := now.Add(72 * time.Hour)
	due7Days := now.Add(168 * time.Hour)

	type wo struct {
		title, description, priority, status, zone string
		lampIdx                                      int
		techID                                       *int
		assignedToName                               string
		dueDate                                      *time.Time
		acceptedAt, startedAt                        *time.Time
		useAlertID                                   bool
	}

	techID1 := 1
	acceptedAt := now.Add(-2 * time.Hour)
	startedAt := now.Add(-1 * time.Hour)

	workOrders := []wo{
		{
			title:          "Remplacement lampe hors service — LMP-001",
			description:    "Courant mesuré à 0 mA. Lampe grillée à remplacer.",
			priority:       "critical",
			status:         "accepted",
			lampIdx:        0,
			techID:         &techID1,
			assignedToName: "Technicien #1",
			dueDate:        &due3Days,
			acceptedAt:     &acceptedAt,
			useAlertID:     true,
		},
		{
			title:          "Contrôle surchauffe driver — LMP-002",
			description:    "Température driver > 85°C. Inspecter la dissipation thermique.",
			priority:       "high",
			status:         "in_progress",
			lampIdx:        1,
			techID:         &techID1,
			assignedToName: "Technicien #1",
			dueDate:        &due3Days,
			acceptedAt:     &acceptedAt,
			startedAt:      &startedAt,
		},
		{
			title:          "Vérification allumage parasite — LMP-003",
			description:    "Allumage détecté à 11h30 alors que programme prévoit extinction. Vérifier photocellule.",
			priority:       "high",
			status:         "created",
			lampIdx:        2,
			dueDate:        &due7Days,
		},
		{
			title:          "Contrôle communication LCU-NORD-01",
			description:    "LCU hors ligne depuis 2h. Vérifier alimentation et antenne.",
			priority:       "critical",
			status:         "created",
			lampIdx:        3,
			dueDate:        &due3Days,
		},
		{
			title:          "Inspection préventive Zone Centre",
			description:    "Ronde de maintenance préventive. Vérifier 5 lampadaires de la zone.",
			priority:       "medium",
			status:         "created",
			lampIdx:        4,
			dueDate:        &due7Days,
		},
		{
			title:          "Mise à jour GPS lampadaire — LMP-006",
			description:    "Coordonnées GPS manquantes ou imprécises. Mise à jour sur site.",
			priority:       "low",
			status:         "created",
			lampIdx:        5,
			dueDate:        &due7Days,
		},
	}

	inserted := 0
	for _, w := range workOrders {
		var lampID sql.NullInt64
		var zone string
		if w.lampIdx < len(lamps) {
			lampID = sql.NullInt64{Int64: lamps[w.lampIdx].id, Valid: true}
			zone = lamps[w.lampIdx].zone
		}

		var techIDVal sql.NullInt64
		var assignedToName string
		if w.techID != nil {
			techIDVal = sql.NullInt64{Int64: int64(*w.techID), Valid: true}
			assignedToName = w.assignedToName
		}

		var alertID sql.NullInt64
		if w.useAlertID && firstAlertID.Valid {
			alertID = firstAlertID
		}

		var dueDate, acceptedAt, startedAt sql.NullTime
		if w.dueDate != nil {
			dueDate = sql.NullTime{Time: *w.dueDate, Valid: true}
		}
		if w.acceptedAt != nil {
			acceptedAt = sql.NullTime{Time: *w.acceptedAt, Valid: true}
		}
		if w.startedAt != nil {
			startedAt = sql.NullTime{Time: *w.startedAt, Valid: true}
		}

		_, err := db.ExecContext(ctx, `
			INSERT INTO work_orders
				(title, description, priority, status, lampadaire_id, zone,
				 source_type, source_alert_id, technician_id, assigned_to_name,
				 due_date, accepted_at, started_at)
			VALUES ($1,$2,$3,$4,$5,$6,'manual',$7,$8,$9,$10,$11,$12)`,
			w.title, w.description, w.priority, w.status, lampID, zone,
			alertID, techIDVal, assignedToName,
			dueDate, acceptedAt, startedAt,
		)
		if err != nil {
			log.Printf("[seed] work_order insert: %v", err)
			continue
		}
		inserted++
	}

	log.Printf("[seed] %d work orders created.", inserted)
	return nil
}
