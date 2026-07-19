package invoice

import (
	"bytes"
	"fmt"
)

// CreateSamplePDF generates a minimal valid single-page PDF for testing and demos.
func CreateSamplePDF() []byte {
	var buf bytes.Buffer
	offsets := make([]int, 5) // indices 1-4 for objects

	buf.WriteString("%PDF-1.4\n")

	// Object 1: Catalog
	offsets[1] = buf.Len()
	buf.WriteString("1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n")

	// Object 2: Pages
	offsets[2] = buf.Len()
	buf.WriteString("2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj\n")

	// Object 3: Page with a text content stream
	offsets[3] = buf.Len()
	buf.WriteString("3 0 obj\n<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Contents 4 0 R /Resources << /Font << /F1 << /Type /Font /Subtype /Type1 /BaseFont /Helvetica >> >> >> >>\nendobj\n")

	// Object 4: Content stream — "Sample Invoice" at (100, 700)
	content := "BT /F1 24 Tf 100 700 Td (Sample Invoice - Demo Cabs) Tj ET"
	offsets[4] = buf.Len()
	fmt.Fprintf(&buf, "4 0 obj\n<< /Length %d >>\nstream\n%s\nendstream\nendobj\n", len(content), content)

	// Cross-reference table
	xrefOffset := buf.Len()
	fmt.Fprintf(&buf, "xref\n0 5\n")
	fmt.Fprintf(&buf, "0000000000 65535 f \n")
	for i := 1; i <= 4; i++ {
		fmt.Fprintf(&buf, "%010d 00000 n \n", offsets[i])
	}

	// Trailer
	fmt.Fprintf(&buf, "trailer\n<< /Size 5 /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", xrefOffset)

	return buf.Bytes()
}
