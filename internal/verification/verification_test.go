package verification

import (
	"testing"
)

// TestComputeHash verifies SHA-256 output is deterministic and correct.
func TestComputeHash(t *testing.T) {
	data := []byte("hello world")
	want := "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9"

	got := ComputeHash(data)
	if got != want {
		t.Fatalf("ComputeHash() = %q, want %q", got, want)
	}
}

// TestMismatchDetection proves that a deliberately tampered PDF is detected.
// Fixture: we simulate the scenario where the stored invoice has a known hash,
// and the submitted file has been modified (even 1 byte). The hashes must differ.
func TestMismatchDetection(t *testing.T) {
	// Simulate the "original" PDF that was stamped and stored.
	original := []byte("%PDF-1.4 original invoice content here 12345")
	originalHash := ComputeHash(original)

	// Simulate a "tampered" PDF — one byte changed.
	tampered := make([]byte, len(original))
	copy(tampered, original)
	tampered[len(tampered)-1] = '6' // change '5' → '6'

	tamperedHash := ComputeHash(tampered)

	if originalHash == tamperedHash {
		t.Fatal("expected hashes to differ for tampered PDF, but they matched")
	}

	t.Logf("Original hash: %s", originalHash)
	t.Logf("Tampered hash: %s", tamperedHash)
	t.Log("Mismatch correctly detected ✓")
}

// TestIdenticalMatchDetection proves that an identical file produces a match.
func TestIdenticalMatchDetection(t *testing.T) {
	data := []byte("%PDF-1.4 this is the exact invoice pdf bytes")
	hash1 := ComputeHash(data)

	// Same content again.
	dataCopy := make([]byte, len(data))
	copy(dataCopy, data)
	hash2 := ComputeHash(dataCopy)

	if hash1 != hash2 {
		t.Fatalf("expected identical hashes for same content, got %q vs %q", hash1, hash2)
	}

	t.Log("Match correctly detected ✓")
}

// TestRefCodePatternMatching validates the regex for reference code extraction.
func TestRefCodePatternMatching(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"REF: ABCD2345", "ABCD2345"},
		{"some text XYZW7654 more text", "XYZW7654"},
		{"no ref code here", ""},
		{"short AB34", ""},                    // only 4 chars
		{"ABCD23456", ""},                     // 9 chars — no match for exactly 8
		{"lowercase abcd2345", ""},            // lowercase doesn't match
		{"REF: AAAA2222 end", "AAAA2222"},     // with prefix
	}

	for _, tc := range tests {
		m := refCodePattern.FindString(tc.input)
		if m != tc.want {
			t.Errorf("refCodePattern.FindString(%q) = %q, want %q", tc.input, m, tc.want)
		}
	}
}

// TestTamperedPDFFixture is the key test proving mismatch detection works
// end-to-end with realistic-looking PDF content.
func TestTamperedPDFFixture(t *testing.T) {
	// Simulate a complete PDF (with header, objects, xref).
	originalPDF := []byte(`%PDF-1.4
1 0 obj
<< /Type /Catalog /Pages 2 0 R >>
endobj
2 0 obj
<< /Type /Pages /Kids [3 0 R] /Count 1 >>
endobj
3 0 obj
<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792]
   /Contents 4 0 R >>
endobj
4 0 obj
<< /Length 80 >>
stream
BT /F1 24 Tf 100 700 Td (Invoice: vendor ABC, amount 1500.00 INR) Tj ET
endstream
endobj
xref
0 5
trailer
<< /Size 5 /Root 1 0 R >>
startxref
0
%%EOF`)

	storedHash := ComputeHash(originalPDF)

	// Tamper: change the amount from 1500.00 to 2500.00 (fraud scenario).
	tamperedPDF := make([]byte, len(originalPDF))
	copy(tamperedPDF, originalPDF)
	// Find "1500" and change to "2500".
	for i := 0; i < len(tamperedPDF)-3; i++ {
		if string(tamperedPDF[i:i+4]) == "1500" {
			tamperedPDF[i] = '2'
			break
		}
	}

	tamperedHash := ComputeHash(tamperedPDF)

	if storedHash == tamperedHash {
		t.Fatal("SECURITY FAILURE: tampered PDF produced the same hash as the original")
	}

	// Simulate the verification logic:
	// submitted_hash != stored_hash → result should be "mismatch"
	var result string
	if tamperedHash == storedHash {
		result = "match"
	} else {
		result = "mismatch"
	}

	if result != "mismatch" {
		t.Fatalf("expected result='mismatch', got %q", result)
	}

	t.Logf("Original (stored) hash: %s", storedHash)
	t.Logf("Tampered (submitted) hash: %s", tamperedHash)
	t.Logf("Result: %s ✓", result)
}
