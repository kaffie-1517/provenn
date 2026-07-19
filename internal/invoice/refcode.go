package invoice

import (
	"crypto/rand"
	"encoding/base32"
)

// GenerateReferenceCode produces an 8-character uppercase base32 string
// (A-Z, 2-7) from 5 random bytes.  Collision is handled by the caller
// retrying — the DB UNIQUE constraint on reference_code is the safeguard.
func GenerateReferenceCode() (string, error) {
	b := make([]byte, 5) // 5 bytes → 40 bits → 8 base32 chars
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b), nil
}
