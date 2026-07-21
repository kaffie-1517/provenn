package db

import (
	"time"

	"github.com/google/uuid"
)

// ── Table models ────────────────────────────────────────────────────────────

type Company struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Plan      string    `json:"plan"`
	CreatedAt time.Time `json:"created_at"`
}

type Partner struct {
	ID         uuid.UUID `json:"id"`
	Name       string    `json:"name"`
	APIKeyHash string    `json:"-"`
	CreatedAt  time.Time `json:"created_at"`
}

type User struct {
	ID           uuid.UUID  `json:"id"`
	Email        string     `json:"email"`
	PasswordHash string     `json:"-"`
	Role         string     `json:"role"`
	CompanyID    *uuid.UUID `json:"company_id"`
	CreatedAt    time.Time  `json:"created_at"`
}

type Invoice struct {
	ID             uuid.UUID  `json:"id"`
	PartnerID      *uuid.UUID `json:"partner_id"`
	ProviderUserID *uuid.UUID `json:"provider_user_id"`
	ReferenceCode  string     `json:"reference_code"`
	PurchaseRef    *string    `json:"purchase_ref"`
	AmountCents    int        `json:"amount_cents"`
	Currency       string     `json:"currency"`
	VendorName     string     `json:"vendor_name"`
	InvoiceDate    time.Time  `json:"invoice_date"`
	Status         string     `json:"status"`
	CreatedAt      time.Time  `json:"created_at"`
}

type InvoiceVersion struct {
	ID            uuid.UUID  `json:"id"`
	InvoiceID     uuid.UUID  `json:"invoice_id"`
	VersionNumber int        `json:"version_number"`
	SHA256Hash    string     `json:"sha256_hash"`
	StorageKey    string     `json:"storage_key"`
	UploadedAt    time.Time  `json:"uploaded_at"`
	UploadedBy    *uuid.UUID `json:"uploaded_by"`
}

type BillingEvent struct {
	ID             uuid.UUID `json:"id"`
	InvoiceID      uuid.UUID `json:"invoice_id"`
	IdempotencyKey string    `json:"idempotency_key"`
	AmountCents    int       `json:"amount_cents"`
	BilledAt       time.Time `json:"billed_at"`
}

type Verification struct {
	ID               uuid.UUID  `json:"id"`
	InvoiceID        *uuid.UUID `json:"invoice_id"`
	CompanyID        uuid.UUID  `json:"company_id"`
	SubmittedBy      uuid.UUID  `json:"submitted_by"`
	SubmittedAt      time.Time  `json:"submitted_at"`
	SubmittedHash    string     `json:"submitted_hash"`
	MatchedVersionID *uuid.UUID `json:"matched_version_id"`
	Result           string     `json:"result"`
	ApprovalStatus   string     `json:"approval_status"`
	ApprovedBy       *uuid.UUID `json:"approved_by"`
	ApprovedAt       *time.Time `json:"approved_at"`

	// Joined fields for display
	EmployeeEmail string `json:"employee_email,omitempty"`
	VendorName    string `json:"vendor_name,omitempty"`
	AmountCents   int    `json:"amount_cents,omitempty"`
	Currency      string `json:"currency,omitempty"`
}

// ── Parameter structs ───────────────────────────────────────────────────────

type CreateInvoiceParams struct {
	PartnerID      *uuid.UUID
	ProviderUserID *uuid.UUID
	ReferenceCode  string
	PurchaseRef    *string
	AmountCents    int
	Currency       string
	VendorName     string
	InvoiceDate    time.Time
}

type CreateInvoiceVersionParams struct {
	InvoiceID     uuid.UUID
	VersionNumber int
	SHA256Hash    string
	StorageKey    string
	UploadedBy    *uuid.UUID
}

type CreateVerificationParams struct {
	InvoiceID        *uuid.UUID
	CompanyID        uuid.UUID
	SubmittedBy      uuid.UUID
	SubmittedHash    string
	MatchedVersionID *uuid.UUID
	Result           string
}

type VerificationFilter struct {
	Result         *string
	ApprovalStatus *string
	Limit          int
	Offset         int
}

// VerificationExportRow is the joined row shape for the export endpoint.
type VerificationExportRow struct {
	EmployeeEmail   string    `json:"employee_email"`
	VendorName      string    `json:"vendor_name"`
	AmountCents     int       `json:"amount_cents"`
	Currency        string    `json:"currency"`
	InvoiceDate     time.Time `json:"invoice_date"`
	Result          string    `json:"result"`
	ApprovedByEmail string    `json:"approved_by_email"`
	ApprovedAt      time.Time `json:"approved_at"`
}
