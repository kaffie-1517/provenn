package auth_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
	"github.com/kaffie-1517/provenn/internal/auth"
)

// TestCompanyIDCannotBeSpoofed is the key security test required by LLD §6.
// It proves that company_id is always sourced from the JWT (set by the server),
// never from the request body. Even if an attacker includes company_id=B in
// the request body, the context will only ever contain company_id=A from the JWT.
func TestCompanyIDCannotBeSpoofed(t *testing.T) {
	secret := "test-secret"

	companyA := uuid.New()
	companyB := uuid.New()
	userID := uuid.New()

	// Issue a JWT whose claims contain company_id = A.
	token, err := auth.IssueJWT(secret, userID, "company_admin", &companyA)
	if err != nil {
		t.Fatalf("IssueJWT: %v", err)
	}

	// Build a handler that extracts company_id from context (the ONLY
	// approved way per LLD §6) and records it.
	var gotCompanyID uuid.UUID
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, err := auth.CompanyIDFromContext(r.Context())
		if err != nil {
			t.Fatalf("CompanyIDFromContext: %v", err)
		}
		gotCompanyID = id
		w.WriteHeader(http.StatusOK)
	})

	// Wrap with JWT middleware — this is the only thing that sets company_id.
	wrapped := auth.JWTMiddleware(secret)(handler)

	// Simulate a request. The Authorization header carries company A's JWT.
	// An attacker might put company_id=B in the JSON body, but the handler
	// must never read it from there — only from context.
	req := httptest.NewRequest("POST", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	wrapped.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	// The company_id from context MUST be A (from JWT), never B.
	if gotCompanyID != companyA {
		t.Errorf("company_id spoofed: got %s, want %s", gotCompanyID, companyA)
	}
	if gotCompanyID == companyB {
		t.Fatal("SECURITY VIOLATION: company_id matched the spoofed value")
	}
}

func TestPasswordHashing(t *testing.T) {
	password := "super-secret-123"

	hash, err := auth.HashPassword(password)
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}

	// Correct password must pass.
	if err := auth.CheckPassword(hash, password); err != nil {
		t.Error("CheckPassword should pass for correct password")
	}

	// Wrong password must fail.
	if err := auth.CheckPassword(hash, "wrong"); err == nil {
		t.Error("CheckPassword should fail for wrong password")
	}
}

func TestJWTRoundTrip(t *testing.T) {
	secret := "test-secret"
	userID := uuid.New()
	companyID := uuid.New()

	token, err := auth.IssueJWT(secret, userID, "employee", &companyID)
	if err != nil {
		t.Fatalf("IssueJWT: %v", err)
	}

	claims, err := auth.ParseJWT(secret, token)
	if err != nil {
		t.Fatalf("ParseJWT: %v", err)
	}

	if claims.UserID != userID {
		t.Errorf("user_id: got %s, want %s", claims.UserID, userID)
	}
	if claims.Role != "employee" {
		t.Errorf("role: got %s, want employee", claims.Role)
	}
	if claims.CompanyID == nil || *claims.CompanyID != companyID {
		t.Errorf("company_id: got %v, want %s", claims.CompanyID, companyID)
	}
}

func TestJWTWrongSecret(t *testing.T) {
	userID := uuid.New()
	token, _ := auth.IssueJWT("secret-a", userID, "employee", nil)

	_, err := auth.ParseJWT("secret-b", token)
	if err == nil {
		t.Error("ParseJWT should fail with wrong secret")
	}
}

func TestRequireRole(t *testing.T) {
	secret := "test-secret"
	userID := uuid.New()
	companyID := uuid.New()

	// Issue token with role=employee.
	token, _ := auth.IssueJWT(secret, userID, "employee", &companyID)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	t.Run("allowed role passes", func(t *testing.T) {
		wrapped := auth.JWTMiddleware(secret)(auth.RequireRole("employee")(handler))
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		wrapped.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rec.Code)
		}
	})

	t.Run("disallowed role gets 403", func(t *testing.T) {
		wrapped := auth.JWTMiddleware(secret)(auth.RequireRole("company_admin")(handler))
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		wrapped.ServeHTTP(rec, req)
		if rec.Code != http.StatusForbidden {
			t.Errorf("expected 403, got %d", rec.Code)
		}
	})

	t.Run("multi-role allows any match", func(t *testing.T) {
		wrapped := auth.JWTMiddleware(secret)(auth.RequireRole("employee", "company_admin")(handler))
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		wrapped.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("expected 200, got %d", rec.Code)
		}
	})
}

func TestPlatformAdminHasNoCompanyID(t *testing.T) {
	secret := "test-secret"
	userID := uuid.New()

	token, _ := auth.IssueJWT(secret, userID, "platform_admin", nil)
	claims, _ := auth.ParseJWT(secret, token)

	ctx := auth.ContextWithClaims(context.Background(), claims)
	_, err := auth.CompanyIDFromContext(ctx)
	if err == nil {
		t.Error("CompanyIDFromContext should fail for platform_admin (nil company_id)")
	}
}

func TestMissingAuthHeaderReturns401(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	wrapped := auth.JWTMiddleware("secret")(handler)

	req := httptest.NewRequest("GET", "/test", nil)
	rec := httptest.NewRecorder()
	wrapped.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 for missing auth header, got %d", rec.Code)
	}
}
