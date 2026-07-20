package admin

import (
	"encoding/json"
	"net/http"
)

// Handlers exposes platform_admin HTTP handlers.
type Handlers struct {
	Store *Store
}

// NewHandlers creates admin handlers backed by the admin store.
func NewHandlers(store *Store) *Handlers {
	return &Handlers{Store: store}
}

// ListPartners handles GET /api/v1/admin/partners.
func (h *Handlers) ListPartners(w http.ResponseWriter, r *http.Request) {
	partners, err := h.Store.ListPartners(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list partners"})
		return
	}
	if partners == nil {
		partners = []PartnerRow{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"partners": partners,
		"total":    len(partners),
	})
}

// ListCompanies handles GET /api/v1/admin/companies.
func (h *Handlers) ListCompanies(w http.ResponseWriter, r *http.Request) {
	companies, err := h.Store.ListCompanies(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list companies"})
		return
	}
	if companies == nil {
		companies = []CompanyRow{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"companies": companies,
		"total":     len(companies),
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
