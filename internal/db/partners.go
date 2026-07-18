package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// CreatePartner inserts a new partner with its bcrypt-hashed API key.
func (s *Store) CreatePartner(ctx context.Context, name, apiKeyHash string) (Partner, error) {
	var p Partner
	var pgID pgtype.UUID

	err := s.pool.QueryRow(ctx,
		`INSERT INTO partners (name, api_key_hash)
		 VALUES ($1, $2)
		 RETURNING id, name, api_key_hash, created_at`,
		name, apiKeyHash,
	).Scan(&pgID, &p.Name, &p.APIKeyHash, &p.CreatedAt)
	if err != nil {
		return Partner{}, fmt.Errorf("create partner: %w", err)
	}

	p.ID = fromPgUUID(pgID)
	return p, nil
}

// GetPartnerByID returns a single partner by primary key.
func (s *Store) GetPartnerByID(ctx context.Context, id uuid.UUID) (Partner, error) {
	var p Partner
	var pgID pgtype.UUID

	err := s.pool.QueryRow(ctx,
		`SELECT id, name, api_key_hash, created_at
		 FROM partners WHERE id = $1`,
		toPgUUID(id),
	).Scan(&pgID, &p.Name, &p.APIKeyHash, &p.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Partner{}, ErrNotFound
		}
		return Partner{}, fmt.Errorf("get partner: %w", err)
	}

	p.ID = fromPgUUID(pgID)
	return p, nil
}

// ListPartners returns all partners. Used by API-key auth middleware to
// iterate and bcrypt.Compare against the presented key.
func (s *Store) ListPartners(ctx context.Context) ([]Partner, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, name, api_key_hash, created_at FROM partners ORDER BY created_at`)
	if err != nil {
		return nil, fmt.Errorf("list partners: %w", err)
	}
	defer rows.Close()

	var partners []Partner
	for rows.Next() {
		var p Partner
		var pgID pgtype.UUID
		if err := rows.Scan(&pgID, &p.Name, &p.APIKeyHash, &p.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan partner: %w", err)
		}
		p.ID = fromPgUUID(pgID)
		partners = append(partners, p)
	}
	return partners, rows.Err()
}
