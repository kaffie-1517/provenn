package verification

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"

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

// Approve handles PATCH /api/v1/verifications/{id}/approve (role=company_admin).
// LLD §4.4: sets approval_status + approved_by + approved_at. Never touches result.
// Scoped to the admin's company_id — can't approve another company's verification.
func (h *Handlers) Approve(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// LLD §6: company_id from the authenticated principal only.
	adminID, err := auth.UserIDFromContext(ctx)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	companyID, err := auth.CompanyIDFromContext(ctx)
	if err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "no company_id in token"})
		return
	}

	// Parse verification ID from URL.
	verifIDStr := chi.URLParam(r, "id")
	verifID, err := uuid.Parse(verifIDStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid verification id"})
		return
	}

	// Parse decision from body.
	var body struct {
		Decision string `json:"decision"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if body.Decision != "approved" && body.Decision != "rejected" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "decision must be 'approved' or 'rejected'"})
		return
	}

	// Fetch the verification and enforce company scoping.
	v, err := h.Service.Store.GetVerificationByID(ctx, verifID)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "verification not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to fetch verification"})
		return
	}

	// KEY CHECK: the verification must belong to the admin's company.
	if v.CompanyID != companyID {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "verification does not belong to your company"})
		return
	}

	// Update the approval — never touches result (LLD §4.4 step 3).
	updated, err := h.Service.Store.UpdateVerificationApproval(ctx, verifID, body.Decision, adminID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to update approval"})
		return
	}

	writeJSON(w, http.StatusOK, updated)
}

// Export handles GET /api/v1/verifications/export (role=company_admin).
// LLD §4.5: streams an .xlsx containing only approval_status='approved' rows
// for the caller's company. Don't write to disk — stream directly.
func (h *Handlers) Export(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	companyID, err := auth.CompanyIDFromContext(ctx)
	if err != nil {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "no company_id in token"})
		return
	}

	rows, err := h.Service.Store.ListApprovedForExport(ctx, companyID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to query export data"})
		return
	}

	// Build the .xlsx in memory and stream it.
	f := excelize.NewFile()
	defer f.Close()

	sheet := "Approved Verifications"
	f.SetSheetName("Sheet1", sheet)

	// Header row.
	headers := []string{
		"Employee Email", "Vendor", "Amount", "Currency",
		"Invoice Date", "Result", "Approved By", "Approved At",
	}
	for i, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheet, cell, h)
	}

	// Style the header row.
	headerStyle, _ := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"#E0E7FF"}, Pattern: 1},
	})
	f.SetRowStyle(sheet, 1, 1, headerStyle)

	// Data rows.
	for i, row := range rows {
		rowNum := i + 2
		amount := fmt.Sprintf("%.2f", float64(row.AmountCents)/100)

		vals := []any{
			row.EmployeeEmail,
			row.VendorName,
			amount,
			row.Currency,
			row.InvoiceDate.Format("2006-01-02"),
			row.Result,
			row.ApprovedByEmail,
			row.ApprovedAt.Format("2006-01-02 15:04:05"),
		}

		for j, v := range vals {
			cell, _ := excelize.CoordinatesToCellName(j+1, rowNum)
			f.SetCellValue(sheet, cell, v)
		}
	}

	// Auto-width columns.
	for i := range headers {
		col, _ := excelize.ColumnNumberToName(i + 1)
		f.SetColWidth(sheet, col, col, 20)
	}

	// Stream the response.
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", "attachment; filename=approved_verifications.xlsx")
	w.WriteHeader(http.StatusOK)

	if _, err := f.WriteTo(w); err != nil {
		// Too late to change the status code, just log.
		fmt.Printf("export: write error: %v\n", err)
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
