// Package twofactor - two-factor sign-in: time-based codes of an
// authenticator app (TOTP, RFC 6238: HMAC-SHA1, 6 digits, 30 seconds),
// one-time recovery codes, and the service turning it on and off and
// checking a code at sign-in.
package twofactor

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1" //nolint:gosec // TOTP's HMAC-SHA1 (RFC 6238), what every authenticator app computes.
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// The codes' parameters: what an authenticator app assumes.
const (
	period = 30
	digits = 6
	// skew - the steps before and after now a code is accepted for (a
	// phone's clock a little off).
	skew = 1
)

var b32 = base32.StdEncoding.WithPadding(base32.NoPadding)

// NewSecret - a new random secret, base32 (160 bits, RFC 4226's size).
func NewSecret() (string, error) {
	b := make([]byte, 20)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("a two-factor secret: %w", err)
	}
	return b32.EncodeToString(b), nil
}

// Step - the time step of t.
func Step(t time.Time) int64 { return t.Unix() / period }

// Code - the code of secret for step; "" for a secret that isn't base32.
func Code(secret string, step int64) string {
	key, err := b32.DecodeString(strings.ToUpper(strings.ReplaceAll(secret, " ", "")))
	if err != nil {
		return ""
	}
	var msg [8]byte
	binary.BigEndian.PutUint64(msg[:], uint64(step))
	mac := hmac.New(sha1.New, key)
	mac.Write(msg[:])
	sum := mac.Sum(nil)
	offset := sum[len(sum)-1] & 0x0f
	n := binary.BigEndian.Uint32(sum[offset:offset+4]) & 0x7fffffff
	return fmt.Sprintf("%0*d", digits, n%1_000_000)
}

// Match - the step within skew of now whose code is code (spaces and
// dashes aside); false - none.
func Match(secret, code string, now time.Time) (int64, bool) {
	code = strings.NewReplacer(" ", "", "-", "").Replace(code)
	if len(code) != digits {
		return 0, false
	}
	at := Step(now)
	for d := int64(-skew); d <= skew; d++ {
		if hmac.Equal([]byte(Code(secret, at+d)), []byte(code)) {
			return at + d, true
		}
	}
	return 0, false
}

// URI - the otpauth:// address an authenticator app reads (its QR code):
// the issuer (the site) and the account (the user's name) it shows.
func URI(secret, issuer, account string) string {
	label := url.PathEscape(issuer + ":" + account)
	q := url.Values{"secret": {secret}, "issuer": {issuer}, "algorithm": {"SHA1"}, "digits": {fmt.Sprint(digits)}, "period": {fmt.Sprint(period)}}
	return "otpauth://totp/" + label + "?" + q.Encode()
}

// Grouped - secret in groups of four, as typed into an app by hand.
func Grouped(secret string) string {
	var b strings.Builder
	for i, r := range secret {
		if i > 0 && i%4 == 0 {
			b.WriteByte(' ')
		}
		b.WriteRune(r)
	}
	return b.String()
}
