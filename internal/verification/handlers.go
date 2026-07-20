package verification

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/kaffie-1517/provenn/internal/auth"
	"github.com/kaffie-1517/provenn/internal/db"
)

// Handlers groups the HTTP handlers for verifications.
type Handlers struct {
	Service *Service
}

// Submit handles POST /api/v1/verifications (role=employee).
// Accepts a multipart file upload. Returns the verification result.
func (h *Handlers) Submit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// LLD §6: IDs from the authenticated principal only.
	userID, err := auth.UserIDFromContext(ctx)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	companyID, err := auth.CompanyIDFromContext(ctx)
	if err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "no company_id in token"})
		return
	}

	// Parse the multipart form — up to 32 MB.
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid multipart form"})
		return
	}

	file, _, err := r.FormFile("pdf")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "pdf file required"})
		return
	}
	defer file.Close()

	pdfData, err := io.ReadAll(file)
	if err != nil || len(pdfData) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "failed to read pdf"})
		return
	}

	result, err := h.Service.Submit(ctx, SubmitParams{
		PDFData:   pdfData,
		UserID:    userID,
		CompanyID: companyID,
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "verification failed"})
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"verification_id":   result.Verification.ID,
		"invoice_id":        result.Verification.InvoiceID,
		"reference_code":    result.RefCode,
		"result":            result.Verification.Result,
		"submitted_hash":    result.Verification.SubmittedHash,
		"matched_version_id": result.Verification.MatchedVersionID,
		"approval_status":   result.Verification.ApprovalStatus,
		"submitted_at":      result.Verification.SubmittedAt,
	})
}

// List handles GET /api/v1/verifications.
// employee: returns only their own submissions.
// company_admin: returns all in their company.
func (h *Handlers) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	role, err := auth.RoleFromContext(ctx)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	// Build filter from query params.
	f := db.VerificationFilter{Limit: 50}
	if v := r.URL.Query().Get("result"); v != "" {
		f.Result = &v
	}
	if v := r.URL.Query().Get("approval_status"); v != "" {
		f.ApprovalStatus = &v
	}

	var verifications []db.Verification

	switch role {
	case "employee":
		// Employee sees only their own submissions.
		userID, err := auth.UserIDFromContext(ctx)
		if err != nil {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		verifications, err = h.Service.Store.ListVerificationsByUser(ctx, userID, f)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list"})
			return
		}

	case "company_admin":
		// Admin sees all in their company.
		companyID, err := auth.CompanyIDFromContext(ctx)
		if err != nil {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "no company_id"})
			return
		}
		verifications, err = h.Service.Store.ListVerificationsByCompany(ctx, companyID, f)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list"})
			return
		}

	default:
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		return
	}

	if verifications == nil {
		verifications = []db.Verification{}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"verifications": verifications,
		"total":         len(verifications),
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
