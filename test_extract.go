package main

import (
	"bytes"
	"fmt"

	"github.com/kaffie-1517/provenn/internal/verification"
	pdfcpuAPI "github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"
)

func minimalPDF() []byte {
	return []byte("%PDF-1.4\n1 0 obj<</Type/Catalog/Pages 2 0 R>>endobj 2 0 obj<</Type/Pages/Kids[3 0 R]/Count 1>>endobj 3 0 obj<</Type/Page/Parent 2 0 R/MediaBox[0 0 612 792]>>endobj xref\n0 4\n0000000000 65535 f \n0000000009 00000 n \n0000000052 00000 n \n0000000101 00000 n \ntrailer<</Size 4/Root 1 0 R>>startxref 190\n%%EOF")
}

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

	return textBuf.Bytes(), nil
}

func main() {
	pdf := minimalPDF()
	stamped, err := stampPDF(pdf, "J57YIH2D")
	if err != nil {
		fmt.Println("Stamp error:", err)
		return
	}

	fmt.Println("Is REF: J57YIH2D in raw bytes?", bytes.Contains(stamped, []byte("REF: J57YIH2D")))
	
	extracted := verification.ExtractRefCode(stamped)
	fmt.Printf("Extracted: '%s'\n", extracted)
}
