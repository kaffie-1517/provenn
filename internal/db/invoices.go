package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

const invoiceCols = `id, partner_id, provider_user_id, reference_code, purchase_ref,
	amount_cents, currency, vendor_name, invoice_date, status, created_at`

func scanInvoice(row interface{ Scan(dest ...any) error }) (Invoice, error) {
	var inv Invoice
	var pgID, pgPartnerID, pgProviderUserID pgtype.UUID

	err := row.Scan(
		&pgID, &pgPartnerID, &pgProviderUserID,
		&inv.ReferenceCode, &inv.PurchaseRef,
		&inv.AmountCents, &inv.Currency, &inv.VendorName,
		&inv.InvoiceDate, &inv.Status, &inv.CreatedAt,
	)
	if err != nil {
		return Invoice{}, err
	}

	inv.ID = fromPgUUID(pgID)
	inv.PartnerID = fromPgNullUUID(pgPartnerID)
	inv.ProviderUserID = fromPgNullUUID(pgProviderUserID)
	return inv, nil
}

// CreateInvoice inserts a new invoice row (status defaults to 'processing').
func (s *Store) CreateInvoice(ctx context.Context, p CreateInvoiceParams) (Invoice, error) {
	row := s.pool.QueryRow(ctx,
		`INSERT INTO invoices (partner_id, provider_user_id, reference_code, purchase_ref,
		                       amount_cents, currency, vendor_name, invoice_date)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		 RETURNING `+invoiceCols,
		toPgNullUUID(p.PartnerID), toPgNullUUID(p.ProviderUserID),
		p.ReferenceCode, p.PurchaseRef,
		p.AmountCents, p.Currency, p.VendorName, p.InvoiceDate,
	)

	inv, err := scanInvoice(row)
	if err != nil {
		return Invoice{}, fmt.Errorf("create invoice: %w", err)
	}
	return inv, nil
}

// GetInvoiceByID returns a single invoice by primary key.
func (s *Store) GetInvoiceByID(ctx context.Context, id uuid.UUID) (Invoice, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT `+invoiceCols+` FROM invoices WHERE id = $1`,
		toPgUUID(id),
	)
	inv, err := scanInvoice(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Invoice{}, ErrNotFound
		}
		return Invoice{}, fmt.Errorf("get invoice by id: %w", err)
	}
	return inv, nil
}

// GetInvoiceByReferenceCode looks up an invoice by its unique reference code.
func (s *Store) GetInvoiceByReferenceCode(ctx context.Context, code string) (Invoice, error) {
	row := s.pool.QueryRow(ctx,
		`SELECT `+invoiceCols+` FROM invoices WHERE reference_code = $1`,
		code,
	)
	inv, err := scanInvoice(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Invoice{}, ErrNotFound
		}
		return Invoice{}, fmt.Errorf("get invoice by reference code: %w", err)
	}
	return inv, nil
}

// UpdateInvoiceStatus sets the status column (e.g. 'processing' → 'ready').
func (s *Store) UpdateInvoiceStatus(ctx context.Context, id uuid.UUID, status string) error {
	ct, err := s.pool.Exec(ctx,
		`UPDATE invoices SET status = $1 WHERE id = $2`,
		status, toPgUUID(id),
	)
	if err != nil {
		return fmt.Errorf("update invoice status: %w", err)
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
