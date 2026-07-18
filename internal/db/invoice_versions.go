package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// CreateInvoiceVersion inserts a new version row for an invoice (called by the
// stamp-and-hash worker after processing the PDF).
func (s *Store) CreateInvoiceVersion(ctx context.Context, p CreateInvoiceVersionParams) (InvoiceVersion, error) {
	var v InvoiceVersion
	var pgID, pgInvoiceID, pgUploadedBy pgtype.UUID

	err := s.pool.QueryRow(ctx,
		`INSERT INTO invoice_versions (invoice_id, version_number, sha256_hash, storage_key, uploaded_by)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id, invoice_id, version_number, sha256_hash, storage_key, uploaded_at, uploaded_by`,
		toPgUUID(p.InvoiceID), p.VersionNumber, p.SHA256Hash, p.StorageKey, toPgNullUUID(p.UploadedBy),
	).Scan(&pgID, &pgInvoiceID, &v.VersionNumber, &v.SHA256Hash, &v.StorageKey, &v.UploadedAt, &pgUploadedBy)
	if err != nil {
		return InvoiceVersion{}, fmt.Errorf("create invoice version: %w", err)
	}

	v.ID = fromPgUUID(pgID)
	v.InvoiceID = fromPgUUID(pgInvoiceID)
	v.UploadedBy = fromPgNullUUID(pgUploadedBy)
	return v, nil
}

// GetLatestInvoiceVersion returns the highest-numbered version for an invoice.
func (s *Store) GetLatestInvoiceVersion(ctx context.Context, invoiceID uuid.UUID) (InvoiceVersion, error) {
	var v InvoiceVersion
	var pgID, pgInvoiceID, pgUploadedBy pgtype.UUID

	err := s.pool.QueryRow(ctx,
		`SELECT id, invoice_id, version_number, sha256_hash, storage_key, uploaded_at, uploaded_by
		 FROM invoice_versions
		 WHERE invoice_id = $1
		 ORDER BY version_number DESC
		 LIMIT 1`,
		toPgUUID(invoiceID),
	).Scan(&pgID, &pgInvoiceID, &v.VersionNumber, &v.SHA256Hash, &v.StorageKey, &v.UploadedAt, &pgUploadedBy)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return InvoiceVersion{}, ErrNotFound
		}
		return InvoiceVersion{}, fmt.Errorf("get latest invoice version: %w", err)
	}

	v.ID = fromPgUUID(pgID)
	v.InvoiceID = fromPgUUID(pgInvoiceID)
	v.UploadedBy = fromPgNullUUID(pgUploadedBy)
	return v, nil
}
