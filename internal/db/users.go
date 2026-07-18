package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// CreateUser inserts a new user. companyID is nil for platform_admin.
func (s *Store) CreateUser(ctx context.Context, email, passwordHash, role string, companyID *uuid.UUID) (User, error) {
	var u User
	var pgID, pgCompanyID pgtype.UUID

	err := s.pool.QueryRow(ctx,
		`INSERT INTO users (email, password_hash, role, company_id)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, email, password_hash, role, company_id, created_at`,
		email, passwordHash, role, toPgNullUUID(companyID),
	).Scan(&pgID, &u.Email, &u.PasswordHash, &u.Role, &pgCompanyID, &u.CreatedAt)
	if err != nil {
		return User{}, fmt.Errorf("create user: %w", err)
	}

	u.ID = fromPgUUID(pgID)
	u.CompanyID = fromPgNullUUID(pgCompanyID)
	return u, nil
}

// GetUserByEmail looks up a user by their unique email (used by login).
func (s *Store) GetUserByEmail(ctx context.Context, email string) (User, error) {
	var u User
	var pgID, pgCompanyID pgtype.UUID

	err := s.pool.QueryRow(ctx,
		`SELECT id, email, password_hash, role, company_id, created_at
		 FROM users WHERE email = $1`,
		email,
	).Scan(&pgID, &u.Email, &u.PasswordHash, &u.Role, &pgCompanyID, &u.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return User{}, ErrNotFound
		}
		return User{}, fmt.Errorf("get user by email: %w", err)
	}

	u.ID = fromPgUUID(pgID)
	u.CompanyID = fromPgNullUUID(pgCompanyID)
	return u, nil
}

// GetUserByID looks up a user by primary key (used by JWT middleware).
func (s *Store) GetUserByID(ctx context.Context, id uuid.UUID) (User, error) {
	var u User
	var pgID, pgCompanyID pgtype.UUID

	err := s.pool.QueryRow(ctx,
		`SELECT id, email, password_hash, role, company_id, created_at
		 FROM users WHERE id = $1`,
		toPgUUID(id),
	).Scan(&pgID, &u.Email, &u.PasswordHash, &u.Role, &pgCompanyID, &u.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return User{}, ErrNotFound
		}
		return User{}, fmt.Errorf("get user by id: %w", err)
	}

	u.ID = fromPgUUID(pgID)
	u.CompanyID = fromPgNullUUID(pgCompanyID)
	return u, nil
}
