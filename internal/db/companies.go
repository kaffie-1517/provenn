package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// CreateCompany inserts a new company and returns it.
func (s *Store) CreateCompany(ctx context.Context, name, plan string) (Company, error) {
	var c Company
	var pgID pgtype.UUID

	err := s.pool.QueryRow(ctx,
		`INSERT INTO companies (name, plan)
		 VALUES ($1, $2)
		 RETURNING id, name, plan, created_at`,
		name, plan,
	).Scan(&pgID, &c.Name, &c.Plan, &c.CreatedAt)
	if err != nil {
		return Company{}, fmt.Errorf("create company: %w", err)
	}

	c.ID = fromPgUUID(pgID)
	return c, nil
}

// GetCompanyByID returns a single company by primary key.
func (s *Store) GetCompanyByID(ctx context.Context, id uuid.UUID) (Company, error) {
	var c Company
	var pgID pgtype.UUID

	err := s.pool.QueryRow(ctx,
		`SELECT id, name, plan, created_at
		 FROM companies WHERE id = $1`,
		toPgUUID(id),
	).Scan(&pgID, &c.Name, &c.Plan, &c.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Company{}, ErrNotFound
		}
		return Company{}, fmt.Errorf("get company: %w", err)
	}

	c.ID = fromPgUUID(pgID)
	return c, nil
}
