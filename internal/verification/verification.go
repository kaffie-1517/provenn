// Package verification implements LLD §4.3: employee submission, reference-code
// extraction (QR decode → text fallback), SHA-256 comparison, and result logic.
package verification

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"image"
	"io"
	"log/slog"
	"regexp"
	"strings"

	// QR decode
	"github.com/makiuchi-d/gozxing"
	"github.com/makiuchi-d/gozxing/qrcode"

	// PDF text extraction
	pdfcpuAPI "github.com/pdfcpu/pdfcpu/pkg/api"

	"github.com/google/uuid"

	"github.com/kaffie-1517/provenn/internal/db"
	"github.com/kaffie-1517/provenn/internal/storage"

	// image decoders for QR (PNG/JPEG)
	_ "image/jpeg"
	_ "image/png"
)

// refCodePattern matches an 8-char uppercase base32 reference code.
var refCodePattern = regexp.MustCompile(`\b([A-Z2-7]{8})\b`)

// Service contains the dependencies for the verification logic.
type Service struct {
	Store   *db.Store
	Storage storage.Store
}

// SubmitParams groups the inputs for a verification submission.
type SubmitParams struct {
	PDFData   []byte
	UserID    uuid.UUID
	CompanyID uuid.UUID
}

// SubmitResult is returned to the handler after processing.
type SubmitResult struct {
	Verification db.Verification `json:"verification"`
	RefCode      string          `json:"reference_code,omitempty"`
}

// Submit implements LLD §4.3:
//  1. Try to decode an embedded QR code first; fall back to text extraction.
//  2. If no reference code found → result='not_found'.
//  3. If found → fetch latest invoice_version, recompute SHA-256, compare.
//  4. Insert verifications row with approval_status='pending'.
func (s *Service) Submit(ctx context.Context, p SubmitParams) (SubmitResult, error) {
	// Compute SHA-256 of the submitted file.
	hash := sha256.Sum256(p.PDFData)
	submittedHash := hex.EncodeToString(hash[:])

	// Step 1+2: Extract reference code.
	refCode := ExtractRefCode(p.PDFData)

	var (
		invoiceID        *uuid.UUID
		matchedVersionID *uuid.UUID
		result           string
	)

	if refCode == "" {
		// Step 3: No reference code found.
		result = "not_found"
		slog.Info("verification: no reference code found in submitted PDF")
	} else {
		// Step 4: Look up the invoice + latest version, compare hashes.
		inv, err := s.Store.GetInvoiceByReferenceCode(ctx, refCode)
		if err != nil {
			// Reference code looks valid but doesn't match any invoice.
			result = "not_found"
			slog.Warn("verification: ref code not in DB", "ref", refCode)
		} else {
			invoiceID = &inv.ID

			version, err := s.Store.GetLatestInvoiceVersion(ctx, inv.ID)
			if err != nil {
				return SubmitResult{}, fmt.Errorf("get invoice version: %w", err)
			}

			if submittedHash == version.SHA256Hash {
				result = "match"
				matchedVersionID = &version.ID
			} else {
				result = "mismatch"
				matchedVersionID = &version.ID
			}

			slog.Info("verification: hash comparison",
				"ref", refCode, "result", result,
				"submitted", submittedHash[:16]+"...",
				"expected", version.SHA256Hash[:16]+"...",
			)
		}
	}

	// Step 5: Insert the verification row.
	v, err := s.Store.CreateVerification(ctx, db.CreateVerificationParams{
		InvoiceID:        invoiceID,
		CompanyID:        p.CompanyID,
		SubmittedBy:      p.UserID,
		SubmittedHash:    submittedHash,
		MatchedVersionID: matchedVersionID,
		Result:           result,
	})
	if err != nil {
		return SubmitResult{}, fmt.Errorf("create verification: %w", err)
	}

	return SubmitResult{
		Verification: v,
		RefCode:      refCode,
	}, nil
}

// ExtractRefCode tries QR decode first, then pdfcpu text extraction, then
// raw byte scan. Exported for testing.
func ExtractRefCode(pdfData []byte) string {
	// Attempt 1: QR decode from image data.
	if code := extractQRFromImage(pdfData); code != "" {
		slog.Info("verification: ref code extracted via QR", "code", code)
		return code
	}

	// Attempt 2: Extract text from PDF via pdfcpu and pattern-match.
	if code := ExtractRefFromText(pdfData); code != "" {
		slog.Info("verification: ref code extracted via pdfcpu text", "code", code)
		return code
	}

	// Attempt 3: Scan raw bytes for "REF: XXXXXXXX" pattern (handles PDF
	// comments injected by the stamp fallback, or any embedded text).
	if code := extractRefFromRawBytes(pdfData); code != "" {
		slog.Info("verification: ref code extracted via raw byte scan", "code", code)
		return code
	}

	return ""
}

// extractRefFromRawBytes scans the raw file bytes for the REF pattern.
// This catches ref codes injected as PDF comments (% REF: XXXXXXXX).
func extractRefFromRawBytes(data []byte) string {
	text := string(data)

	// Look for "REF: XXXXXXXX" first.
	refPattern := regexp.MustCompile(`REF:\s*([A-Z2-7]{8})`)
	if m := refPattern.FindStringSubmatch(text); len(m) > 1 {
		return m[1]
	}

	return ""
}

// extractQRFromImage tries to decode a QR code from raw image data.
// Works when the submitted file is a rasterized scan or a plain image.
func extractQRFromImage(data []byte) string {
	reader := bytes.NewReader(data)
	img, _, err := image.Decode(reader)
	if err != nil {
		return "" // Not a raw image — expected for real PDFs.
	}

	bmp, err := gozxing.NewBinaryBitmapFromImage(img)
	if err != nil {
		return ""
	}

	qrReader := qrcode.NewQRCodeReader()
	decoded, err := qrReader.Decode(bmp, nil)
	if err != nil {
		return ""
	}

	text := strings.TrimSpace(decoded.GetText())
	if refCodePattern.MatchString(text) {
		return text
	}
	return ""
}

// ExtractRefFromText uses pdfcpu to extract text content and pattern-matches
// for the 8-char base32 reference code. Exported for testing.
func ExtractRefFromText(pdfData []byte) string {
	reader := bytes.NewReader(pdfData)

	var buf bytes.Buffer
	digestFn := func(r io.Reader, pageNr int) error {
		_, err := io.Copy(&buf, r)
		return err
	}

	if err := pdfcpuAPI.ExtractContent(reader, nil, digestFn, nil); err != nil {
		slog.Debug("verification: pdfcpu text extraction failed", "error", err)
		return ""
	}

	text := buf.String()

	// Look for "REF: XXXXXXXX" pattern first (our stamp format).
	refPattern := regexp.MustCompile(`REF:\s*([A-Z2-7]{8})`)
	if m := refPattern.FindStringSubmatch(text); len(m) > 1 {
		return m[1]
	}

	// Generic pattern: any standalone 8-char base32 string.
	if m := refCodePattern.FindString(text); m != "" {
		return m
	}

	return ""
}

// ComputeHash computes the SHA-256 hex digest of the given data.
// Exported for use in tests.
func ComputeHash(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// ReadAll reads the full body from a reader and returns bytes.
func ReadAll(r io.Reader) ([]byte, error) {
	return io.ReadAll(r)
}
