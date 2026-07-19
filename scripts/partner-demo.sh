#!/usr/bin/env bash
# partner-demo.sh — Simulates a partner (e.g., Demo Cabs) issuing an invoice
# via the API-key-authenticated endpoint.
#
# Prerequisites:
#   1. `make up && make migrate` (services + schema)
#   2. `make seed` (creates the seeded partner with known API key)
#      OR: insert a partner + known API key hash manually (see below)
#
# Usage:
#   ./scripts/partner-demo.sh [API_KEY]
#
# If no API_KEY is provided, it defaults to "demo-partner-key-1" which is the
# seeded partner's plaintext key (you must seed first).

set -euo pipefail

API_BASE="${API_BASE:-http://localhost:8080}"
API_KEY="${1:-demo-partner-key-1}"

echo "=== ProveNN Partner Demo ==="
echo "API: $API_BASE"
echo ""

# 1. Generate a sample PDF.
echo "→ Generating sample PDF..."
SAMPLE_PDF=$(mktemp /tmp/provenn-demo-XXXXXX.pdf)

# Use Go to generate a valid PDF.
go run -C "$(dirname "$0")/.." ./internal/invoice/cmd/genpdf "$SAMPLE_PDF" 2>/dev/null || {
  # Fallback: create a minimal PDF inline.
  printf '%%PDF-1.4\n1 0 obj<</Type/Catalog/Pages 2 0 R>>endobj\n2 0 obj<</Type/Pages/Kids[3 0 R]/Count 1>>endobj\n3 0 obj<</Type/Page/Parent 2 0 R/MediaBox[0 0 612 792]>>endobj\nxref\n0 4\n0000000000 65535 f \n0000000009 00000 n \n0000000058 00000 n \n0000000115 00000 n \ntrailer<</Size 4/Root 1 0 R>>\nstartxref\n190\n%%%%EOF' > "$SAMPLE_PDF"
  echo "  (using fallback minimal PDF)"
}
echo "  PDF: $SAMPLE_PDF"
echo ""

# 2. Issue the invoice via the partner endpoint.
echo "→ Issuing invoice..."
RESPONSE=$(curl -s -w '\n%{http_code}' \
  -X POST "${API_BASE}/api/v1/partner/invoices" \
  -H "X-Partner-Key: ${API_KEY}" \
  -F "pdf=@${SAMPLE_PDF}" \
  -F "amount_cents=150000" \
  -F "currency=INR" \
  -F "vendor_name=Demo Cabs" \
  -F "invoice_date=2024-07-15" \
  -F "purchase_ref=ride-$(date +%s)")

HTTP_CODE=$(echo "$RESPONSE" | tail -1)
BODY=$(echo "$RESPONSE" | sed '$d')

echo "  HTTP $HTTP_CODE"
echo "  $BODY"
echo ""

if [ "$HTTP_CODE" != "202" ]; then
  echo "✗ Invoice creation failed"
  rm -f "$SAMPLE_PDF"
  exit 1
fi

REF_CODE=$(echo "$BODY" | grep -o '"reference_code":"[^"]*"' | cut -d'"' -f4)
echo "  Reference code: $REF_CODE"
echo ""

# 3. Poll until ready.
echo "→ Polling for status..."
for i in $(seq 1 20); do
  sleep 1
  STATUS_RESP=$(curl -s "${API_BASE}/api/v1/invoices/${REF_CODE}")
  READY=$(echo "$STATUS_RESP" | grep -o '"ready":true' || true)
  if [ -n "$READY" ]; then
    echo "  ✓ Invoice is READY"
    echo "  $STATUS_RESP"
    break
  fi
  echo "  attempt $i: still processing..."
done

if [ -z "$READY" ]; then
  echo "  ⚠ Invoice did not become ready within 20 seconds"
fi

# Cleanup
rm -f "$SAMPLE_PDF"
echo ""
echo "=== Done ==="
