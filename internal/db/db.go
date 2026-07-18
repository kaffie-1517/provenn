// Package db handles migrations and the repository layer.
package db

import (
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Sentinel errors returned by repository methods.
var (
	ErrNotFound = errors.New("not found")
	ErrConflict = errors.New("conflict: duplicate key")
)

// Store wraps a pgxpool.Pool and exposes typed repository methods for every
// table. Methods are organised by table in separate files.
type Store struct {
	pool *pgxpool.Pool
}

// NewStore returns a Store backed by pool.
func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// Pool returns the underlying pool (useful in tests or for running raw queries).
func (s *Store) Pool() *pgxpool.Pool {
	return s.pool
}

// ── pgtype ↔ google/uuid conversion helpers ────────────────────────────────

func toPgUUID(id uuid.UUID) pgtype.UUID {
	return pgtype.UUID{Bytes: id, Valid: true}
}

func toPgNullUUID(id *uuid.UUID) pgtype.UUID {
	if id == nil {
		return pgtype.UUID{}
	}
	return pgtype.UUID{Bytes: *id, Valid: true}
}

func fromPgUUID(u pgtype.UUID) uuid.UUID {
	return uuid.UUID(u.Bytes)
}

func fromPgNullUUID(u pgtype.UUID) *uuid.UUID {
	if !u.Valid {
		return nil
	}
	id := uuid.UUID(u.Bytes)
	return &id
}
