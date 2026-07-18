package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

const verifCols = `id, invoice_id, company_id, submitted_by, submitted_at,
	submitted_hash, matched_version_id, result, approval_status, approved_by, approved_at`

func scanVerification(row interface{ Scan(dest ...any) error }) (Verification, error) {
	var v Verification
	var pgID, pgInvoiceID, pgCompanyID, pgSubmittedBy, pgMatchedVer, pgApprovedBy pgtype.UUID

	err := row.Scan(
		&pgID, &pgInvoiceID, &pgCompanyID, &pgSubmittedBy, &v.SubmittedAt,
		&v.SubmittedHash, &pgMatchedVer, &v.Result, &v.ApprovalStatus,
		&pgApprovedBy, &v.ApprovedAt,
	)
	if err != nil {
		return Verification{}, err
	}

	v.ID = fromPgUUID(pgID)
	v.InvoiceID = fromPgNullUUID(pgInvoiceID)
	v.CompanyID = fromPgUUID(pgCompanyID)
	v.SubmittedBy = fromPgUUID(pgSubmittedBy)
	v.MatchedVersionID = fromPgNullUUID(pgMatchedVer)
	v.ApprovedBy = fromPgNullUUID(pgApprovedBy)
	return v, nil
}

// CreateVerification inserts a new verification row (approval_status defaults to 'pending').
func (s *Store) CreateVerification(ctx context.Context, p CreateVerificationParams) (Verification, error) {
	row := s.pool.QueryRow(ctx,
		`INSERT INTO verifications (invoice_id, company_id, submitted_by,
		                            submitted_hash, matched_version_id, result)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING `+verifCols,
		toPgNullUUID(p.InvoiceID), toPgUUID(p.CompanyID), toPgUUID(p.SubmittedBy),
		p.SubmittedHash, toPgNullUUID(p.MatchedVersionID), p.Result,
	)

	v, err := scanVerification(row)
	if err != nil {
		return Verification{}, fmt.Errorf("create verification: %w", err)
	}
	return v, nil
}

// GetVerificationByID returns a single verification by primary key.
func (s *Store) GetVerificationByID(ctx context.Context, id uuid.UUID) (Verification, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT `+verifCols+` FROM verifications WHERE id = $1`,
		toPgUUID(id),
	)
	v, err := scanVerification(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Verification{}, ErrNotFound
		}
		return Verification{}, fmt.Errorf("get verification: %w", err)
	}
	return v, nil
}

// ListVerificationsByCompany returns verifications scoped to a company,
// with optional result/approval_status filters and pagination.
func (s *Store) ListVerificationsByCompany(ctx context.Context, companyID uuid.UUID, f VerificationFilter) ([]Verification, error) {
	query := `SELECT ` + verifCols + ` FROM verifications WHERE company_id = $1`
	args := []any{toPgUUID(companyID)}
	idx := 2

	if f.Result != nil {
		query += fmt.Sprintf(` AND result = $%d`, idx)
		args = append(args, *f.Result)
		idx++
	}
	if f.ApprovalStatus != nil {
		query += fmt.Sprintf(` AND approval_status = $%d`, idx)
		args = append(args, *f.ApprovalStatus)
		idx++
	}

	query += ` ORDER BY submitted_at DESC`

	if f.Limit > 0 {
		query += fmt.Sprintf(` LIMIT $%d`, idx)
		args = append(args, f.Limit)
		idx++
	}
	if f.Offset > 0 {
		query += fmt.Sprintf(` OFFSET $%d`, idx)
		args = append(args, f.Offset)
		idx++ //nolint:ineffassign // keep the pattern consistent
	}

	return s.queryVerifications(ctx, query, args)
}

// ListVerificationsByUser returns only the calling employee's own submissions.
func (s *Store) ListVerificationsByUser(ctx context.Context, userID uuid.UUID, f VerificationFilter) ([]Verification, error) {
	query := `SELECT ` + verifCols + ` FROM verifications WHERE submitted_by = $1`
	args := []any{toPgUUID(userID)}
	idx := 2

	if f.Result != nil {
		query += fmt.Sprintf(` AND result = $%d`, idx)
		args = append(args, *f.Result)
		idx++
	}
	if f.ApprovalStatus != nil {
		query += fmt.Sprintf(` AND approval_status = $%d`, idx)
		args = append(args, *f.ApprovalStatus)
		idx++
	}

	query += ` ORDER BY submitted_at DESC`

	if f.Limit > 0 {
		query += fmt.Sprintf(` LIMIT $%d`, idx)
		args = append(args, f.Limit)
		idx++
	}
	if f.Offset > 0 {
		query += fmt.Sprintf(` OFFSET $%d`, idx)
		args = append(args, f.Offset)
		idx++ //nolint:ineffassign
	}

	return s.queryVerifications(ctx, query, args)
}

// UpdateVerificationApproval sets the approval decision. This never touches
// the result field — the automated check and the human decision are independent.
func (s *Store) UpdateVerificationApproval(ctx context.Context, id uuid.UUID, decision string, approvedBy uuid.UUID) (Verification, error) {
	row := s.pool.QueryRow(ctx,
		`UPDATE verifications
		 SET approval_status = $1, approved_by = $2, approved_at = now()
		 WHERE id = $3
		 RETURNING `+verifCols,
		decision, toPgUUID(approvedBy), toPgUUID(id),
	)

	v, err := scanVerification(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Verification{}, ErrNotFound
		}
		return Verification{}, fmt.Errorf("update verification approval: %w", err)
	}
	return v, nil
}

// ListApprovedForExport returns the joined rows needed for the Excel export:
// only approval_status='approved' verifications for the given company.
func (s *Store) ListApprovedForExport(ctx context.Context, companyID uuid.UUID) ([]VerificationExportRow, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT
			u.email                AS employee_email,
			COALESCE(i.vendor_name, '')  AS vendor_name,
			COALESCE(i.amount_cents, 0)  AS amount_cents,
			COALESCE(i.currency, '')     AS currency,
			COALESCE(i.invoice_date, '1970-01-01') AS invoice_date,
			v.result,
			COALESCE(approver.email, '') AS approved_by_email,
			v.approved_at
		 FROM verifications v
		 JOIN users u ON v.submitted_by = u.id
		 LEFT JOIN invoices i ON v.invoice_id = i.id
		 LEFT JOIN users approver ON v.approved_by = approver.id
		 WHERE v.company_id = $1 AND v.approval_status = 'approved'
		 ORDER BY v.approved_at DESC`,
		toPgUUID(companyID),
	)
	if err != nil {
		return nil, fmt.Errorf("list approved for export: %w", err)
	}
	defer rows.Close()

	var out []VerificationExportRow
	for rows.Next() {
		var r VerificationExportRow
		if err := rows.Scan(
			&r.EmployeeEmail, &r.VendorName, &r.AmountCents, &r.Currency,
			&r.InvoiceDate, &r.Result, &r.ApprovedByEmail, &r.ApprovedAt,
		); err != nil {
			return nil, fmt.Errorf("scan export row: %w", err)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ── helpers ─────────────────────────────────────────────────────────────────

func (s *Store) queryVerifications(ctx context.Context, query string, args []any) ([]Verification, error) {
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query verifications: %w", err)
	}
	defer rows.Close()

	var out []Verification
	for rows.Next() {
		v, err := scanVerification(rows)
		if err != nil {
			return nil, fmt.Errorf("scan verification: %w", err)
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
