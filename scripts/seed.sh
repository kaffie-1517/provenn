#!/usr/bin/env bash
# make seed — creates the demo data described in LLD §7:
#   2 companies, each with 1 company_admin + 1 employee
#   1 provider user
#   1 platform_admin (company_id null)
#   1 partner with known API key "demo-partner-key-2024"
set -euo pipefail

API="${API_BASE:-http://localhost:8080}"
PSQL="docker exec deploy-postgres-1 psql -U provenn -d provenn -tAc"

echo "══════════════════════════════════════════════════════════"
echo "  ProveNN Seed Script (LLD §7)"
echo "══════════════════════════════════════════════════════════"

# ── Companies ────────────────────────────────────────────────
CO_A=$($PSQL "INSERT INTO companies (name, plan) VALUES ('Acme Corp', 'pro') ON CONFLICT DO NOTHING RETURNING id" | head -1)
CO_B=$($PSQL "INSERT INTO companies (name, plan) VALUES ('Beta Inc', 'starter') ON CONFLICT DO NOTHING RETURNING id" | head -1)

if [ -z "$CO_A" ]; then
  CO_A=$($PSQL "SELECT id FROM companies WHERE name='Acme Corp'" | head -1)
fi
if [ -z "$CO_B" ]; then
  CO_B=$($PSQL "SELECT id FROM companies WHERE name='Beta Inc'" | head -1)
fi
echo "✓ Companies: Acme Corp ($CO_A), Beta Inc ($CO_B)"

# ── Partner (known API key for demo) ─────────────────────────
# bcrypt hash of "demo-partner-key-2024"
PARTNER_KEY="demo-partner-key-2024"
# Generate bcrypt hash dynamically using Go so it never gets stale
cat << 'EOF' > /tmp/gen_hash.go
package main
import (
	"fmt"
	"golang.org/x/crypto/bcrypt"
)
func main() {
	hash, _ := bcrypt.GenerateFromPassword([]byte("demo-partner-key-2024"), bcrypt.DefaultCost)
	fmt.Print(string(hash))
}
EOF
PARTNER_HASH=$(go run /tmp/gen_hash.go) # No need to escape $ for psql inside single quotes
rm /tmp/gen_hash.go
$PSQL "UPDATE partners SET api_key_hash = '$PARTNER_HASH' WHERE name = 'DemoPartner'" > /dev/null 2>&1 || true
$PSQL "INSERT INTO partners (name, api_key_hash) VALUES ('DemoPartner', '$PARTNER_HASH') ON CONFLICT DO NOTHING" > /dev/null 2>&1 || true
PARTNER_ID=$($PSQL "SELECT id FROM partners WHERE name='DemoPartner'" | head -1)
echo "✓ Partner: DemoPartner ($PARTNER_ID), key=$PARTNER_KEY"

# ── Users via API (passwords hashed properly) ────────────────
register() {
  local email=$1 password=$2 role=$3 company_id=${4:-}
  local body
  if [ -n "$company_id" ]; then
    body="{\"email\":\"$email\",\"password\":\"$password\",\"role\":\"$role\",\"company_id\":\"$company_id\"}"
  else
    body="{\"email\":\"$email\",\"password\":\"$password\",\"role\":\"$role\"}"
  fi
  curl -sf -X POST "$API/api/v1/auth/register" -H 'Content-Type: application/json' -d "$body" > /dev/null 2>&1 || true
}

# Acme Corp
register "admin@acme.com"    "password" "company_admin" "$CO_A"
register "employee@acme.com" "password" "employee"      "$CO_A"

# Beta Inc
register "admin@beta.com"    "password" "company_admin" "$CO_B"
register "employee@beta.com" "password" "employee"      "$CO_B"

# Provider (linked to Acme for the demo)
register "provider@demo.com" "password" "provider"      "$CO_A"

# Platform admin (no company)
register "padmin@provenn.io" "password" "platform_admin"

echo "✓ Users created:"
echo "    admin@acme.com / password  (company_admin, Acme Corp)"
echo "    employee@acme.com / password  (employee, Acme Corp)"
echo "    admin@beta.com / password  (company_admin, Beta Inc)"
echo "    employee@beta.com / password  (employee, Beta Inc)"
echo "    provider@demo.com / password  (provider)"
echo "    padmin@provenn.io / password  (platform_admin)"

echo ""
echo "══════════════════════════════════════════════════════════"
echo "  Seed complete. Partner API key: $PARTNER_KEY"
echo "══════════════════════════════════════════════════════════"
