package invoice

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/google/uuid"
	pdfcpuAPI "github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"
	"github.com/riverqueue/river"
	qrcode "github.com/skip2/go-qrcode"

	"github.com/kaffie-1517/provenn/internal/db"
	"github.com/kaffie-1517/provenn/internal/storage"
)

// StampAndHashWorker is the River worker that processes stamp_and_hash jobs.
// LLD §4.1 step 5: stamp QR + ref code into footer, SHA-256, upload, insert version.
type StampAndHashWorker struct {
	river.WorkerDefaults[StampAndHashArgs]
	Store   *db.Store
	Storage storage.Store
}

func (w *StampAndHashWorker) Work(ctx context.Context, job *river.Job[StampAndHashArgs]) error {
	invoiceID, err := uuid.Parse(job.Args.InvoiceID)
	if err != nil {
		return fmt.Errorf("parse invoice_id: %w", err)
	}

	slog.Info("stamp_and_hash: starting", "invoice_id", invoiceID)

	// 1. Get invoice for reference code.
	inv, err := w.Store.GetInvoiceByID(ctx, invoiceID)
	if err != nil {
		return fmt.Errorf("get invoice: %w", err)
	}

	// 2. Download raw PDF from storage.
	rawKey := fmt.Sprintf("%s/raw.pdf", invoiceID)
	rawReader, err := w.Storage.Get(ctx, rawKey)
	if err != nil {
		return fmt.Errorf("get raw PDF: %w", err)
	}
	rawBytes, err := io.ReadAll(rawReader)
	rawReader.Close()
	if err != nil {
		return fmt.Errorf("read raw PDF: %w", err)
	}

	// 3. Stamp QR code + reference code text into PDF footer.
	stamped, err := stampPDF(rawBytes, inv.ReferenceCode)
	if err != nil {
		// If stamping fails, use the raw PDF as-is (graceful degradation for the demo).
		slog.Warn("stamp_and_hash: PDF stamping failed, using raw PDF", "error", err)
		stamped = rawBytes
	}

	// 4. Compute SHA-256 over the final bytes.
	hash := sha256.Sum256(stamped)
	hashHex := hex.EncodeToString(hash[:])

	// 5. Upload stamped PDF to {invoice_id}/v1.pdf.
	versionKey := fmt.Sprintf("%s/v1.pdf", invoiceID)
	if err := w.Storage.Put(ctx, versionKey, bytes.NewReader(stamped), int64(len(stamped)), "application/pdf"); err != nil {
		return fmt.Errorf("upload stamped PDF: %w", err)
	}

	// 6. Insert invoice_versions row.
	_, err = w.Store.CreateInvoiceVersion(ctx, db.CreateInvoiceVersionParams{
		InvoiceID:     invoiceID,
		VersionNumber: 1,
		StorageKey:    versionKey,
		SHA256Hash:    hashHex,
	})
	if err != nil {
		return fmt.Errorf("create invoice version: %w", err)
	}

	// 7. Flip invoice status to 'ready'.
	if err := w.Store.UpdateInvoiceStatus(ctx, invoiceID, "ready"); err != nil {
		return fmt.Errorf("update invoice status: %w", err)
	}

	slog.Info("stamp_and_hash: completed", "invoice_id", invoiceID, "sha256", hashHex)
	return nil
}

// stampPDF adds a text reference code and a QR code image to the PDF footer.
func stampPDF(pdfBytes []byte, refCode string) ([]byte, error) {
	input := bytes.NewReader(pdfBytes)

	// --- Text stamp: reference code in the bottom-center footer ---
	textDesc := "font:Helvetica, points:10, pos:bc, off:0 25, rot:0, op:0.9"
	textWM, err := pdfcpuAPI.TextWatermark("REF: "+refCode, textDesc, true, false, types.POINTS)
	if err != nil {
		return nil, fmt.Errorf("text watermark config: %w", err)
	}

	var textBuf bytes.Buffer
	if err := pdfcpuAPI.AddWatermarks(input, &textBuf, nil, textWM, nil); err != nil {
		return nil, fmt.Errorf("add text stamp: %w", err)
	}

	// --- QR code image stamp: bottom-left corner ---
	qrPNG, err := qrcode.Encode(refCode, qrcode.Medium, 150)
	if err != nil {
		return nil, fmt.Errorf("generate QR: %w", err)
	}

	// pdfcpu's ImageWatermark takes a filename, so write QR to a temp file.
	tmpFile, err := os.CreateTemp("", "provenn-qr-*.png")
	if err != nil {
		// If temp file fails (e.g., in restricted container), return text-only version.
		slog.Warn("stamp_and_hash: temp file creation failed, skipping QR stamp", "error", err)
		return textBuf.Bytes(), nil
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write(qrPNG); err != nil {
		tmpFile.Close()
		return textBuf.Bytes(), nil
	}
	tmpFile.Close()

	imgDesc := "pos:bl, off:20 20, scalefactor:.25, rot:0, op:0.9"
	imgWM, err := pdfcpuAPI.ImageWatermark(tmpFile.Name(), imgDesc, true, false, types.POINTS)
	if err != nil {
		return textBuf.Bytes(), nil // fallback to text-only
	}

	textReader := bytes.NewReader(textBuf.Bytes())
	var finalBuf bytes.Buffer
	if err := pdfcpuAPI.AddWatermarks(textReader, &finalBuf, nil, imgWM, nil); err != nil {
		return textBuf.Bytes(), nil // fallback to text-only
	}

	return finalBuf.Bytes(), nil
}
