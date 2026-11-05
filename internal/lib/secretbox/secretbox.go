// Package secretbox seals the secrets a site keeps in its database (the
// keys and passwords of outside services, the two-factor secrets) with a
// key from its environment, so a dump of the database doesn't give them
// away: AES-256-GCM, the key derived from the given one with HKDF-SHA256.
package secretbox

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"math"
	"strings"
)

// prefix marks a sealed value (its version: how it was sealed).
const prefix = "enc:v1:"

// MinKeyLen - the shortest key accepted, in bytes.
const MinKeyLen = 16

// ErrNoKey - a sealed value with no key to open it.
var ErrNoKey = errors.New("a sealed secret, but no key (SECRETS_KEY) to open it")

// ErrWrongKey - a sealed value none of the keys opens.
var ErrWrongKey = errors.New("a sealed secret none of the keys (SECRETS_KEY) opens")

// minKeyBits - the least a key's characters must carry (Weak).
const minKeyBits = 64

// placeholders - words of a key that is an example, not a secret.
var placeholders = []string{"change-me", "changeme", "example", "password"}

// Box - the keys: the first seals, each opens (a key being replaced is
// kept after the new one until the values are sealed again). A nil or
// empty Box seals nothing.
type Box struct {
	aeads []cipher.AEAD
	weak  bool
}

// New - the box of the keys, separated by commas; "" - none.
func New(keys string) (*Box, error) {
	b := &Box{}
	for i, k := range strings.Split(keys, ",") {
		k = strings.TrimSpace(k)
		if k == "" {
			continue
		}
		if len(k) < MinKeyLen {
			return nil, fmt.Errorf("secrets key %d: at least %d characters", i+1, MinKeyLen)
		}
		key, err := hkdf.Key(sha256.New, []byte(k), nil, "kosatka secretbox v1", 32)
		if err != nil {
			return nil, err
		}
		block, err := aes.NewCipher(key)
		if err != nil {
			return nil, err
		}
		aead, err := cipher.NewGCM(block)
		if err != nil {
			return nil, err
		}
		b.aeads = append(b.aeads, aead)
		b.weak = b.weak || weakKey(k)
	}
	return b, nil
}

// Weak - whether a key is guessable: a placeholder, or too few different
// characters for its length (fewer than minKeyBits by their Shannon
// entropy) - a copy of the database would then open by trying keys.
func (b *Box) Weak() bool { return b != nil && b.weak }

// weakKey - see Box.Weak.
func weakKey(k string) bool {
	lower := strings.ToLower(k)
	for _, p := range placeholders {
		if strings.Contains(lower, p) {
			return true
		}
	}
	counts := map[byte]int{}
	for i := 0; i < len(k); i++ {
		counts[k[i]]++
	}
	bits := 0.0
	for _, n := range counts {
		p := float64(n) / float64(len(k))
		bits -= p * math.Log2(p) * float64(len(k))
	}
	return bits < minKeyBits
}

// Enabled - whether the box has a key (seals).
func (b *Box) Enabled() bool { return b != nil && len(b.aeads) > 0 }

// Sealed - whether stored is a sealed value.
func Sealed(stored string) bool { return strings.HasPrefix(stored, prefix) }

// Seal - plain sealed with the first key; "" or a box without keys - as
// it is.
func (b *Box) Seal(plain string) (string, error) {
	if plain == "" || !b.Enabled() {
		return plain, nil
	}
	aead := b.aeads[0]
	nonce := make([]byte, aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	return prefix + base64.RawStdEncoding.EncodeToString(aead.Seal(nonce, nonce, []byte(plain), nil)), nil
}

// Open - the plain value of stored: a sealed one opened, a plain one as
// it is. current - it is as Seal would store it now: sealed with the first
// key, or plain with no key; false - to seal again.
func (b *Box) Open(stored string) (plain string, current bool, err error) {
	if !Sealed(stored) {
		return stored, stored == "" || !b.Enabled(), nil
	}
	if !b.Enabled() {
		return "", false, ErrNoKey
	}
	data, err := base64.RawStdEncoding.DecodeString(strings.TrimPrefix(stored, prefix))
	if err != nil {
		return "", false, fmt.Errorf("a sealed secret: %w", err)
	}
	for i, aead := range b.aeads {
		n := aead.NonceSize()
		if len(data) < n {
			break
		}
		if out, err := aead.Open(nil, data[:n], data[n:], nil); err == nil {
			return string(out), i == 0, nil
		}
	}
	return "", false, ErrWrongKey
}
