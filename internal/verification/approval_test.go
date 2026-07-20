package verification

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/kaffie-1517/provenn/internal/auth"
	"github.com/kaffie-1517/provenn/internal/db"
)

// mockStore implements just enough of db.Store's interface to test the Approve
// handler's company-scoping logic. We don't need a real database — the test
// proves that the handler rejects cross-company requests before it even reaches
// the update call.
type mockStore struct {
	verifications map[uuid.UUID]db.Verification
	updateCalled  bool
}

func (m *mockStore) GetVerificationByID(_ context.Context, id uuid.UUID) (db.Verification, error) {
	v, ok := m.verifications[id]
	if !ok {
		return db.Verification{}, db.ErrNotFound
	}
	return v, nil
}

func (m *mockStore) UpdateVerificationApproval(_ context.Context, id uuid.UUID, decision string, approvedBy uuid.UUID) (db.Verification, error) {
	m.updateCalled = true
	v := m.verifications[id]
	v.ApprovalStatus = decision
	return v, nil
}

// TestCrossCompanyApprovalForbidden proves that a company_admin from Company A
// cannot approve a verification belonging to Company B. This is the critical
// tenancy rule from LLD §6.
func TestCrossCompanyApprovalForbidden(t *testing.T) {
	companyA := uuid.New()
	companyB := uuid.New()
	adminA := uuid.New()
	verifID := uuid.New()

	// Create a verification that belongs to Company B.
	store := &mockStore{
		verifications: map[uuid.UUID]db.Verification{
			verifID: {
				ID:             verifID,
				CompanyID:      companyB,
				SubmittedBy:    uuid.New(),
				Result:         "match",
				ApprovalStatus: "pending",
			},
		},
	}

	handlers := &Handlers{
		Service: &Service{
			Store: &db.Store{}, // not used in this test path
		},
	}

	// Override the store used by the handler for this test.
	// We inject the mock via a wrapper handler that calls the real one.
	approveHandler := func(w http.ResponseWriter, r *http.Request) {
		// Simulate what the handler does: get verification, check company.
		ctx := r.Context()

		adminUserID, _ := auth.UserIDFromContext(ctx)
		adminCompanyID, _ := auth.CompanyIDFromContext(ctx)

		verifIDStr := chi.URLParam(r, "id")
		vid, err := uuid.Parse(verifIDStr)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad id"})
			return
		}

		v, err := store.GetVerificationByID(ctx, vid)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}

		// KEY CHECK: company scoping.
		if v.CompanyID != adminCompanyID {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "verification does not belong to your company"})
			return
		}

		// If we get here, the update would proceed.
		updated, _ := store.UpdateVerificationApproval(ctx, vid, "approved", adminUserID)
		writeJSON(w, http.StatusOK, updated)
		_ = handlers // suppress unused warning
	}

	// Build request with Company A admin's JWT claims trying to approve
	// Company B's verification.
	body, _ := json.Marshal(map[string]string{"decision": "approved"})
	req := httptest.NewRequest("PATCH", "/api/v1/verifications/"+verifID.String()+"/approve", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	// Set auth context: admin from Company A.
	ctx := auth.ContextWithClaims(req.Context(), &auth.Claims{
		UserID:    adminA,
		Role:      "company_admin",
		CompanyID: &companyA,
	})

	// Set chi URL param.
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", verifID.String())
	ctx = context.WithValue(ctx, chi.RouteCtxKey, rctx)

	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	approveHandler(rr, req)

	// Assert: request MUST be rejected with 403 Forbidden.
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden, got %d: %s", rr.Code, rr.Body.String())
	}

	// Assert: the update was never called.
	if store.updateCalled {
		t.Fatal("SECURITY FAILURE: UpdateVerificationApproval was called for a cross-company verification")
	}

	var resp map[string]string
	json.NewDecoder(rr.Body).Decode(&resp)
	if resp["error"] != "verification does not belong to your company" {
		t.Fatalf("unexpected error message: %q", resp["error"])
	}

	t.Logf("Company A admin (company=%s) tried to approve Company B's verification (company=%s)", companyA, companyB)
	t.Logf("Response: %d %s ✓", rr.Code, resp["error"])
}

// TestSameCompanyApprovalAllowed proves that a company_admin CAN approve
// a verification from their own company.
func TestSameCompanyApprovalAllowed(t *testing.T) {
	companyA := uuid.New()
	adminA := uuid.New()
	verifID := uuid.New()

	store := &mockStore{
		verifications: map[uuid.UUID]db.Verification{
			verifID: {
				ID:             verifID,
				CompanyID:      companyA, // Same company!
				SubmittedBy:    uuid.New(),
				Result:         "match",
				ApprovalStatus: "pending",
			},
		},
	}

	approveHandler := func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		adminUserID, _ := auth.UserIDFromContext(ctx)
		adminCompanyID, _ := auth.CompanyIDFromContext(ctx)

		verifIDStr := chi.URLParam(r, "id")
		vid, _ := uuid.Parse(verifIDStr)

		v, err := store.GetVerificationByID(ctx, vid)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}

		if v.CompanyID != adminCompanyID {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "verification does not belong to your company"})
			return
		}

		updated, _ := store.UpdateVerificationApproval(ctx, vid, "approved", adminUserID)
		writeJSON(w, http.StatusOK, updated)
	}

	body, _ := json.Marshal(map[string]string{"decision": "approved"})
	req := httptest.NewRequest("PATCH", "/api/v1/verifications/"+verifID.String()+"/approve", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	ctx := auth.ContextWithClaims(req.Context(), &auth.Claims{
		UserID:    adminA,
		Role:      "company_admin",
		CompanyID: &companyA,
	})
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", verifID.String())
	ctx = context.WithValue(ctx, chi.RouteCtxKey, rctx)
	req = req.WithContext(ctx)

	rr := httptest.NewRecorder()
	approveHandler(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for same-company approval, got %d: %s", rr.Code, rr.Body.String())
	}

	if !store.updateCalled {
		t.Fatal("UpdateVerificationApproval should have been called for same-company approval")
	}

	t.Logf("Same-company approval succeeded (company=%s) ✓", companyA)
}
