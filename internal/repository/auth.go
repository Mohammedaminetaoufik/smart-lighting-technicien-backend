package repository

import (
	"context"
	"database/sql"
	"fmt"
)

type AuthUser struct {
	ID           int
	FullName     string
	Email        string
	Role         string
	Status       string
	PasswordHash string
}

func FindUserByEmail(ctx context.Context, db *sql.DB, email string) (*AuthUser, error) {
	const q = `
		SELECT id, full_name, email, role, status, COALESCE(password_hash, '')
		FROM users
		WHERE email = $1
		LIMIT 1`
	u := &AuthUser{}
	err := db.QueryRowContext(ctx, q, email).Scan(
		&u.ID, &u.FullName, &u.Email, &u.Role, &u.Status, &u.PasswordHash,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("utilisateur introuvable")
	}
	return u, err
}

func UpdateLastLogin(ctx context.Context, db *sql.DB, id int) error {
	_, err := db.ExecContext(ctx,
		`UPDATE users SET last_login_at = NOW() WHERE id = $1`, id)
	return err
}
