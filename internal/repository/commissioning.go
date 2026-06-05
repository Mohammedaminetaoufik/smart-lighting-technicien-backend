package repository

import (
	"context"
	"database/sql"

	"technicien-mobile/internal/models"
)

// ListCommissioningTasks returns lampadaires not yet fully commissioned.
func ListCommissioningTasks(ctx context.Context, db *sql.DB, status string) ([]models.CommissioningTask, error) {
	query := `
		SELECT l.id, COALESCE(l.reference,''), COALESCE(l.zone,''), COALESCE(l.etat,'offline'),
		       COALESCE(l.commissioning_status,'discovered'), COALESCE(l.commissioning_step,0),
		       COALESCE(l.test_comm_status,'pending'), COALESCE(l.test_dimming_status,'pending'),
		       COALESCE(l.commissioning_notes,''),
		       l.latitude, l.longitude, COALESCE(l.location_status,'manual'),
		       l.lcu_id, COALESCE(lcu.reference,'')
		FROM lampadaires l
		LEFT JOIN lcus lcu ON lcu.id = l.lcu_id
		WHERE l.archived_at IS NULL`
	var args []any
	if status != "" {
		query += " AND l.commissioning_status = $1"
		args = append(args, status)
	} else {
		query += " AND l.commissioning_status <> 'commissioned'"
	}
	query += " ORDER BY l.commissioning_step DESC, l.reference LIMIT 300"

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.CommissioningTask
	for rows.Next() {
		t, err := scanCommissioning(rows)
		if err != nil {
			continue
		}
		out = append(out, t)
	}
	return out, nil
}

func GetCommissioningTask(ctx context.Context, db *sql.DB, id int) (*models.CommissioningTask, error) {
	row := db.QueryRowContext(ctx, `
		SELECT l.id, COALESCE(l.reference,''), COALESCE(l.zone,''), COALESCE(l.etat,'offline'),
		       COALESCE(l.commissioning_status,'discovered'), COALESCE(l.commissioning_step,0),
		       COALESCE(l.test_comm_status,'pending'), COALESCE(l.test_dimming_status,'pending'),
		       COALESCE(l.commissioning_notes,''),
		       l.latitude, l.longitude, COALESCE(l.location_status,'manual'),
		       l.lcu_id, COALESCE(lcu.reference,'')
		FROM lampadaires l
		LEFT JOIN lcus lcu ON lcu.id = l.lcu_id
		WHERE l.id=$1`, id)
	t, err := scanCommissioningRow(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// CommissioningUpdateGPS sets coordinates and advances to 'located' if still discovered.
func CommissioningUpdateGPS(ctx context.Context, db *sql.DB, id int, lat, lng float64) error {
	_, err := db.ExecContext(ctx, `
		UPDATE lampadaires
		SET latitude=$1, longitude=$2, location_status='confirmed',
		    commissioning_status = CASE WHEN commissioning_status='discovered' THEN 'located' ELSE commissioning_status END,
		    commissioning_step = GREATEST(commissioning_step, 1),
		    updated_at=NOW()
		WHERE id=$3`, lat, lng, id)
	return err
}

func CommissioningSetTestComm(ctx context.Context, db *sql.DB, id int, ok bool) error {
	status := "passed"
	if !ok { status = "failed" }
	_, err := db.ExecContext(ctx, `
		UPDATE lampadaires
		SET test_comm_status=$1,
		    commissioning_step = GREATEST(commissioning_step, 2),
		    updated_at=NOW()
		WHERE id=$2`, status, id)
	return err
}

func CommissioningSetTestDimming(ctx context.Context, db *sql.DB, id int, ok bool) error {
	status := "passed"
	if !ok { status = "failed" }
	_, err := db.ExecContext(ctx, `
		UPDATE lampadaires
		SET test_dimming_status=$1,
		    commissioning_status = CASE WHEN commissioning_status IN ('located','configured') THEN 'tested' ELSE commissioning_status END,
		    commissioning_step = GREATEST(commissioning_step, 3),
		    updated_at=NOW()
		WHERE id=$2`, status, id)
	return err
}

func CommissioningValidate(ctx context.Context, db *sql.DB, id int) error {
	_, err := db.ExecContext(ctx, `
		UPDATE lampadaires
		SET commissioning_status='commissioned',
		    commissioning_step = 5,
		    commissioned_at=NOW(),
		    updated_at=NOW()
		WHERE id=$1`, id)
	return err
}

func CommissioningFail(ctx context.Context, db *sql.DB, id int, reason string) error {
	_, err := db.ExecContext(ctx, `
		UPDATE lampadaires
		SET commissioning_status='failed',
		    commissioning_notes = COALESCE(commissioning_notes,'') || E'\n[ÉCHEC] ' || $1,
		    updated_at=NOW()
		WHERE id=$2`, reason, id)
	return err
}

func CommissioningAddNote(ctx context.Context, db *sql.DB, id int, note string) error {
	_, err := db.ExecContext(ctx, `
		UPDATE lampadaires
		SET commissioning_notes = COALESCE(commissioning_notes,'') || E'\n' || $1,
		    updated_at=NOW()
		WHERE id=$2`, note, id)
	return err
}

func scanCommissioning(rows *sql.Rows) (models.CommissioningTask, error) {
	var t models.CommissioningTask
	var lat, lng sql.NullFloat64
	var lcuID sql.NullInt64
	err := rows.Scan(&t.ID, &t.Reference, &t.Zone, &t.Etat,
		&t.CommissioningStatus, &t.CommissioningStep,
		&t.TestCommStatus, &t.TestDimmingStatus, &t.CommissioningNotes,
		&lat, &lng, &t.LocationStatus, &lcuID, &t.LCUReference)
	if err != nil {
		return t, err
	}
	if lat.Valid { t.Latitude = &lat.Float64 }
	if lng.Valid { t.Longitude = &lng.Float64 }
	if lcuID.Valid { v := int(lcuID.Int64); t.LCUID = &v }
	return t, nil
}

func scanCommissioningRow(row *sql.Row) (models.CommissioningTask, error) {
	var t models.CommissioningTask
	var lat, lng sql.NullFloat64
	var lcuID sql.NullInt64
	err := row.Scan(&t.ID, &t.Reference, &t.Zone, &t.Etat,
		&t.CommissioningStatus, &t.CommissioningStep,
		&t.TestCommStatus, &t.TestDimmingStatus, &t.CommissioningNotes,
		&lat, &lng, &t.LocationStatus, &lcuID, &t.LCUReference)
	if err != nil {
		return t, err
	}
	if lat.Valid { t.Latitude = &lat.Float64 }
	if lng.Valid { t.Longitude = &lng.Float64 }
	if lcuID.Valid { v := int(lcuID.Int64); t.LCUID = &v }
	return t, nil
}
