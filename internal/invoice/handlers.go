package invoice

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/kaffie-1517/provenn/internal/auth"
)

// Handlers holds HTTP handler dependencies for invoice endpoints.
type Handlers struct {
	Service *Service
}

// CreateFromPartner handles POST /api/v1/partner/invoices (API-key auth).
// partner_id comes from context (set by APIKeyMiddleware) — never from the body.
func (h *Handlers) CreateFromPartner(w http.ResponseWriter, r *http.Request) {
	partnerID, err := auth.PartnerIDFromContext(r.Context())
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	h.createInvoice(w, r, &CreateParams{PartnerID: &partnerID})
}

// CreateFromProvider handles POST /api/v1/invoices (JWT auth, role=provider).
// provider_user_id comes from context (set by JWTMiddleware) — never from the body.
func (h *Handlers) CreateFromProvider(w http.ResponseWriter, r *http.Request) {
	userID, err := auth.UserIDFromContext(r.Context())
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	h.createInvoice(w, r, &CreateParams{ProviderUserID: &userID})
}

// createInvoice is the shared creation logic for both partner and provider paths.
// Accepts multipart/form-data with a "pdf" file and form fields.
func (h *Handlers) createInvoice(w http.ResponseWriter, r *http.Request, params *CreateParams) {
	// Parse multipart form (32 MB max).
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid multipart form"})
		return
	}

	// Read the PDF file.
	file, _, err := r.FormFile("pdf")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "pdf file is required"})
		return
	}
	defer file.Close()

	pdfData, err := io.ReadAll(file)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "failed to read pdf"})
		return
	}

	// Parse form fields.
	amountCents, err := ParseAmountCents(r.FormValue("amount_cents"))
	if err != nil || amountCents <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "amount_cents must be a positive integer"})
		return
	}

	currency := r.FormValue("currency")
	if currency == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "currency is required"})
		return
	}

	vendorName := r.FormValue("vendor_name")
	if vendorName == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "vendor_name is required"})
		return
	}

	invoiceDate, err := time.Parse("2006-01-02", r.FormValue("invoice_date"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invoice_date must be YYYY-MM-DD"})
		return
	}

	purchaseRef := r.FormValue("purchase_ref")
	var purchaseRefPtr *string
	if purchaseRef != "" {
		purchaseRefPtr = &purchaseRef
	}

	// Fill remaining params.
	params.AmountCents = amountCents
	params.Currency = currency
	params.VendorName = vendorName
	params.InvoiceDate = invoiceDate
	params.PurchaseRef = purchaseRefPtr
	params.PDFData = pdfData

	// Create the invoice.
	result, err := h.Service.CreateInvoice(r.Context(), *params)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create invoice"})
		return
	}

	writeJSON(w, http.StatusAccepted, result)
}

// GetStatus handles GET /api/v1/invoices/{referenceCode}.
// No auth required — the reference code acts as a capability token.
func (h *Handlers) GetStatus(w http.ResponseWriter, r *http.Request) {
	refCode := chi.URLParam(r, "referenceCode")
	if refCode == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "reference code required"})
		return
	}

	inv, err := h.Service.Store.GetInvoiceByReferenceCode(r.Context(), refCode)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "invoice not found"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"invoice_id":     inv.ID,
		"reference_code": inv.ReferenceCode,
		"status":         inv.Status,
		"ready":          inv.Status == "ready",
		"amount_cents":   inv.AmountCents,
		"currency":       inv.Currency,
		"vendor_name":    inv.VendorName,
	})
}

// Download handles GET /api/v1/invoices/{referenceCode}/download.
// LLD §4.2: look up invoice + latest version, insert billing event (idempotent),
// stream PDF from storage. No auth — the reference code is the capability token.
func (h *Handlers) Download(w http.ResponseWriter, r *http.Request) {
	refCode := chi.URLParam(r, "referenceCode")
	if refCode == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "reference code required"})
		return
	}

	ctx := r.Context()

	// 1. Look up the invoice.
	inv, err := h.Service.Store.GetInvoiceByReferenceCode(ctx, refCode)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "invoice not found"})
		return
	}

	// 2. If not ready, return 202 Accepted.
	if inv.Status != "ready" {
		writeJSON(w, http.StatusAccepted, map[string]any{
			"status":  inv.Status,
			"message": "invoice is still processing, try again shortly",
		})
		return
	}

	// 3. Get the latest invoice_versions row.
	version, err := h.Service.Store.GetLatestInvoiceVersion(ctx, inv.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "version not found"})
		return
	}

	// 4. Insert billing event (idempotent — ON CONFLICT DO NOTHING).
	_, _, err = h.Service.Store.CreateBillingEvent(ctx, inv.ID, inv.AmountCents)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "billing error"})
		return
	}

	// 5. Stream the file from storage.
	obj, err := h.Service.Storage.Get(ctx, version.StorageKey)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "file not found in storage"})
		return
	}
	defer obj.Close()

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="invoice-%s.pdf"`, refCode))
	w.WriteHeader(http.StatusOK)
	io.Copy(w, obj)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
