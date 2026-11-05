package twofactor

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

// RecoveryCodes - how many recovery codes a user gets.
const RecoveryCodes = 10

// recoveryAlphabet - the letters of a recovery code: no 0/o, 1/l/i to
// confuse (31 of them, 8 in a code: about 40 bits).
const recoveryAlphabet = "23456789abcdefghjkmnpqrstuvwxyz"

// NewRecoveryCodes - n new recovery codes, "xxxx-xxxx".
func NewRecoveryCodes(n int) ([]string, error) {
	codes := make([]string, n)
	for i := range codes {
		b := make([]byte, 8)
		if _, err := rand.Read(b); err != nil {
			return nil, fmt.Errorf("recovery codes: %w", err)
		}
		for j := range b {
			b[j] = recoveryAlphabet[int(b[j])%len(recoveryAlphabet)]
		}
		codes[i] = string(b[:4]) + "-" + string(b[4:])
	}
	return codes, nil
}

// HashRecoveryCode - how a recovery code is kept: SHA-256 hex of it as
// typed any way (letter case, spaces and dashes aside).
func HashRecoveryCode(code string) string {
	code = strings.ToLower(strings.NewReplacer(" ", "", "-", "").Replace(strings.TrimSpace(code)))
	sum := sha256.Sum256([]byte(code))
	return hex.EncodeToString(sum[:])
}

// HashRecoveryCodes - HashRecoveryCode of each.
func HashRecoveryCodes(codes []string) []string {
	out := make([]string, len(codes))
	for i, c := range codes {
		out[i] = HashRecoveryCode(c)
	}
	return out
}
