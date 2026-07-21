// Package invoice handles creation, versioning, PDF stamping, and hashing.
package invoice

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/kaffie-1517/provenn/internal/db"
	"github.com/kaffie-1517/provenn/internal/observability"
	"github.com/kaffie-1517/provenn/internal/storage"
	"github.com/riverqueue/river"
)

// StampAndHashArgs are the arguments for the background stamp-and-hash job.
type StampAndHashArgs struct {
	InvoiceID string `json:"invoice_id"`
}

func (StampAndHashArgs) Kind() string { return "stamp_and_hash" }

// EnqueueFunc abstracts River's Insert so the service doesn't depend on the
// generic Client[TTx] type directly.
type EnqueueFunc func(ctx context.Context, args river.JobArgs, opts *river.InsertOpts) error

// Service orchestrates invoice creation.
type Service struct {
	Store      *db.Store
	Storage    storage.Store
	EnqueueJob EnqueueFunc
}

// CreateParams holds everything needed to create an invoice.
type CreateParams struct {
	PartnerID      *uuid.UUID
	ProviderUserID *uuid.UUID
	AmountCents    int
	Currency       string
	VendorName     string
	InvoiceDate    time.Time
	PurchaseRef    *string
	PDFData        []byte // raw PDF bytes
}

// CreateResult is returned to the HTTP caller immediately.
type CreateResult struct {
	InvoiceID     uuid.UUID `json:"invoice_id"`
	ReferenceCode string    `json:"reference_code"`
	Status        string    `json:"status"`
}

// CreateInvoice implements LLD §4.1:
// 1. Generate 8-char base32 reference code (retry on collision)
// 2. Insert invoices row (status='processing')
// 3. Upload raw PDF to storage
// 4. Enqueue stamp_and_hash River job
// 5. Return immediately
func (s *Service) CreateInvoice(ctx context.Context, p CreateParams) (*CreateResult, error) {
	// 1. Generate reference code with collision retry.
	var refCode string
	var inv db.Invoice
	var err error

	for attempt := 0; attempt < 5; attempt++ {
		refCode, err = GenerateReferenceCode()
		if err != nil {
			return nil, fmt.Errorf("generate ref code: %w", err)
		}

		// 2. Insert invoice row.
		inv, err = s.Store.CreateInvoice(ctx, db.CreateInvoiceParams{
			ReferenceCode:  refCode,
			AmountCents:    p.AmountCents,
			Currency:       p.Currency,
			VendorName:     p.VendorName,
			InvoiceDate:    p.InvoiceDate,
			PartnerID:      p.PartnerID,
			ProviderUserID: p.ProviderUserID,
			PurchaseRef:    strPtrToNullable(p.PurchaseRef),
		})
		if err == nil {
			break // success — no collision
		}
		slog.Warn("ref code collision, retrying", "attempt", attempt, "code", refCode)
	}
	if err != nil {
		return nil, fmt.Errorf("create invoice: %w", err)
	}

	// 3. Upload raw PDF to storage at {invoice_id}/raw.pdf.
	key := fmt.Sprintf("%s/raw.pdf", inv.ID)
	reader := bytes.NewReader(p.PDFData)
	if err := s.Storage.Put(ctx, key, reader, int64(len(p.PDFData)), "application/pdf"); err != nil {
		return nil, fmt.Errorf("upload raw PDF: %w", err)
	}

	// 4. Enqueue stamp_and_hash job.
	if err := s.EnqueueJob(ctx, StampAndHashArgs{InvoiceID: inv.ID.String()}, nil); err != nil {
		return nil, fmt.Errorf("enqueue job: %w", err)
	}

	slog.Info("invoice created", "invoice_id", inv.ID, "ref_code", refCode)
	observability.InvoicesIssuedTotal.Inc()

	return &CreateResult{
		InvoiceID:     inv.ID,
		ReferenceCode: refCode,
		Status:        "processing",
	}, nil
}

func strPtrToNullable(s *string) *string {
	if s == nil || *s == "" {
		return nil
	}
	return s
}

// ParseAmountCents parses a string amount into cents.
func ParseAmountCents(s string) (int, error) {
	return strconv.Atoi(s)
}
