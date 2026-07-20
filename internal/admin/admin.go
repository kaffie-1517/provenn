// Package admin handles platform_admin-only cross-tenant queries (Portal 2).
// LLD §6: this package intentionally queries across ALL companies and partners.
// These methods must NEVER be added to the tenant-scoped db.Store — they live
// here precisely to prevent accidental cross-tenant leaks.
package admin

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Store provides platform_admin-only queries that span all tenants.
type Store struct {
	pool *pgxpool.Pool
}

// NewStore creates an admin store from a shared pgxpool.
func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// PartnerRow is the response shape for GET /api/v1/admin/partners.
type PartnerRow struct {
	ID               uuid.UUID `json:"id"`
	Name             string    `json:"name"`
	CreatedAt        time.Time `json:"created_at"`
	InvoiceCount30d  int       `json:"invoice_count_30d"`
}

// CompanyRow is the response shape for GET /api/v1/admin/companies.
type CompanyRow struct {
	ID                    uuid.UUID `json:"id"`
	Name                  string    `json:"name"`
	Plan                  string    `json:"plan"`
	CreatedAt             time.Time `json:"created_at"`
	VerificationCount30d  int       `json:"verification_count_30d"`
}

// ListPartners returns all partners with their invoice issuance counts
// for the last 30 days. Deliberately cross-tenant.
func (s *Store) ListPartners(ctx context.Context) ([]PartnerRow, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT
			p.id,
			p.name,
			p.created_at,
			COUNT(i.id) AS invoice_count_30d
		FROM partners p
		LEFT JOIN invoices i
			ON i.partner_id = p.id
			AND i.created_at >= NOW() - INTERVAL '30 days'
		GROUP BY p.id, p.name, p.created_at
		ORDER BY invoice_count_30d DESC, p.name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []PartnerRow
	for rows.Next() {
		var r PartnerRow
		if err := rows.Scan(&r.ID, &r.Name, &r.CreatedAt, &r.InvoiceCount30d); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ListCompanies returns all companies with their verification counts
// for the last 30 days. Deliberately cross-tenant.
func (s *Store) ListCompanies(ctx context.Context) ([]CompanyRow, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT
			c.id,
			c.name,
			c.plan,
			c.created_at,
			COUNT(v.id) AS verification_count_30d
		FROM companies c
		LEFT JOIN verifications v
			ON v.company_id = c.id
			AND v.submitted_at >= NOW() - INTERVAL '30 days'
		GROUP BY c.id, c.name, c.plan, c.created_at
		ORDER BY verification_count_30d DESC, c.name
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []CompanyRow
	for rows.Next() {
		var r CompanyRow
		if err := rows.Scan(&r.ID, &r.Name, &r.Plan, &r.CreatedAt, &r.VerificationCount30d); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
