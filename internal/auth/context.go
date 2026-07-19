package auth

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

type contextKey int

const (
	claimsKey  contextKey = iota
	partnerKey contextKey = iota
)

// ErrNoClaims is returned when no JWT claims are found in the context.
var ErrNoClaims = errors.New("no auth claims in context")

// ContextWithClaims stores JWT claims in the context.
func ContextWithClaims(ctx context.Context, c *Claims) context.Context {
	return context.WithValue(ctx, claimsKey, c)
}

// ClaimsFromContext retrieves JWT claims from the context.
func ClaimsFromContext(ctx context.Context) (*Claims, error) {
	c, ok := ctx.Value(claimsKey).(*Claims)
	if !ok || c == nil {
		return nil, ErrNoClaims
	}
	return c, nil
}

// UserIDFromContext returns the authenticated user's ID.
func UserIDFromContext(ctx context.Context) (uuid.UUID, error) {
	c, err := ClaimsFromContext(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	return c.UserID, nil
}

// CompanyIDFromContext returns the authenticated user's company_id.
//
// This is the ONLY way handlers should obtain a company_id — never from the
// request body. This enforces the rule in LLD §6: "Every repository method
// touching company-scoped data takes that ID as a required parameter, sourced
// only from the authenticated principal."
func CompanyIDFromContext(ctx context.Context) (uuid.UUID, error) {
	c, err := ClaimsFromContext(ctx)
	if err != nil {
		return uuid.Nil, err
	}
	if c.CompanyID == nil {
		return uuid.Nil, errors.New("user has no company_id (platform_admin)")
	}
	return *c.CompanyID, nil
}

// RoleFromContext returns the authenticated user's role.
func RoleFromContext(ctx context.Context) (string, error) {
	c, err := ClaimsFromContext(ctx)
	if err != nil {
		return "", err
	}
	return c.Role, nil
}

// ContextWithPartnerID stores the authenticated partner's ID.
func ContextWithPartnerID(ctx context.Context, id uuid.UUID) context.Context {
	return context.WithValue(ctx, partnerKey, id)
}

// PartnerIDFromContext returns the authenticated partner's ID.
//
// This is the ONLY way handlers should obtain a partner_id — never from the
// request body. Enforces LLD §6.
func PartnerIDFromContext(ctx context.Context) (uuid.UUID, error) {
	id, ok := ctx.Value(partnerKey).(uuid.UUID)
	if !ok {
		return uuid.Nil, errors.New("no partner_id in context")
	}
	return id, nil
}
