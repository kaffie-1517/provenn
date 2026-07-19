package auth

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/kaffie-1517/provenn/internal/db"
)

// Handlers holds dependencies for auth HTTP handlers.
type Handlers struct {
	Store     *db.Store
	JWTSecret string
}

type registerRequest struct {
	Email     string  `json:"email"`
	Password  string  `json:"password"`
	Role      string  `json:"role"`
	CompanyID *string `json:"company_id"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type tokenResponse struct {
	Token string  `json:"token"`
	User  db.User `json:"user"`
}

// Register handles POST /api/v1/auth/register.
func (h *Handlers) Register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	if req.Email == "" || req.Password == "" || req.Role == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "email, password, and role are required"})
		return
	}

	validRoles := map[string]bool{
		"provider": true, "employee": true,
		"company_admin": true, "platform_admin": true,
	}
	if !validRoles[req.Role] {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid role"})
		return
	}

	// Parse optional company_id.
	var companyID *uuid.UUID
	if req.CompanyID != nil && *req.CompanyID != "" {
		id, err := uuid.Parse(*req.CompanyID)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid company_id"})
			return
		}
		companyID = &id
	}

	// platform_admin must NOT have a company_id.
	if req.Role == "platform_admin" && companyID != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "platform_admin must not have a company_id"})
		return
	}
	// All other roles MUST have a company_id.
	if req.Role != "platform_admin" && companyID == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "company_id is required for this role"})
		return
	}

	hash, err := HashPassword(req.Password)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	user, err := h.Store.CreateUser(r.Context(), req.Email, hash, req.Role, companyID)
	if err != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": "email already registered"})
		return
	}

	writeJSON(w, http.StatusCreated, user)
}

// Login handles POST /api/v1/auth/login.
func (h *Handlers) Login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}

	if req.Email == "" || req.Password == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "email and password are required"})
		return
	}

	user, err := h.Store.GetUserByEmail(r.Context(), req.Email)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
		return
	}

	if err := CheckPassword(user.PasswordHash, req.Password); err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
		return
	}

	token, err := IssueJWT(h.JWTSecret, user.ID, user.Role, user.CompanyID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to issue token"})
		return
	}

	writeJSON(w, http.StatusOK, tokenResponse{Token: token, User: user})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
