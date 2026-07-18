package db

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// CreateBillingEvent idempotently inserts a billing event for an invoice.
// The idempotency_key is set to the invoice ID string; the UNIQUE constraint
// guarantees at most one event per invoice. Returns (event, created, error)
// where created is false if the event already existed.
func (s *Store) CreateBillingEvent(ctx context.Context, invoiceID uuid.UUID, amountCents int) (BillingEvent, bool, error) {
	key := invoiceID.String()

	// Try to insert; ON CONFLICT DO NOTHING if already billed.
	ct, err := s.pool.Exec(ctx,
		`INSERT INTO billing_events (invoice_id, idempotency_key, amount_cents)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (idempotency_key) DO NOTHING`,
		toPgUUID(invoiceID), key, amountCents,
	)
	if err != nil {
		return BillingEvent{}, false, fmt.Errorf("insert billing event: %w", err)
	}
	created := ct.RowsAffected() > 0

	// Always fetch the row (whether just created or pre-existing).
	be, err := s.getBillingEventByKey(ctx, key)
	if err != nil {
		return BillingEvent{}, false, err
	}
	return be, created, nil
}

func (s *Store) getBillingEventByKey(ctx context.Context, key string) (BillingEvent, error) {
	var be BillingEvent
	var pgID, pgInvoiceID pgtype.UUID

	err := s.pool.QueryRow(ctx,
		`SELECT id, invoice_id, idempotency_key, amount_cents, billed_at
		 FROM billing_events WHERE idempotency_key = $1`,
		key,
	).Scan(&pgID, &pgInvoiceID, &be.IdempotencyKey, &be.AmountCents, &be.BilledAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return BillingEvent{}, ErrNotFound
		}
		return BillingEvent{}, fmt.Errorf("get billing event: %w", err)
	}

	be.ID = fromPgUUID(pgID)
	be.InvoiceID = fromPgUUID(pgInvoiceID)
	return be, nil
}
