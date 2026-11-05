package twofactor

import (
	"encoding/base32"
	"strings"
	"testing"
	"time"
)

// rfcSecret - RFC 6238's SHA-1 test key, "12345678901234567890", base32.
var rfcSecret = base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString([]byte("12345678901234567890"))

// TestCode - RFC 6238's SHA-1 vectors (their last six digits).
func TestCode(t *testing.T) {
	for unix, want := range map[int64]string{
		59: "287082", 1111111109: "081804", 1111111111: "050471", 1234567890: "005924", 2000000000: "279037",
	} {
		if got := Code(rfcSecret, Step(time.Unix(unix, 0))); got != want {
			t.Errorf("%d: %s, want %s", unix, got, want)
		}
	}
	if Code("not base32!", 1) != "" {
		t.Error("a broken secret gives no code")
	}
	if Code(strings.ToLower(Grouped(rfcSecret)), 1) != Code(rfcSecret, 1) {
		t.Error("the secret as typed by hand (groups, lower case) is the same")
	}
}

// TestMatch - the code of now or a step around it, spaces aside; not two
// steps off, not a short one.
func TestMatch(t *testing.T) {
	now := time.Unix(1111111111, 0)
	at := Step(now)
	for name, tc := range map[string]struct {
		code string
		step int64
		ok   bool
	}{
		"now":        {Code(rfcSecret, at), at, true},
		"before":     {Code(rfcSecret, at-1), at - 1, true},
		"after":      {Code(rfcSecret, at+1), at + 1, true},
		"spaced":     {Code(rfcSecret, at)[:3] + " " + Code(rfcSecret, at)[3:], at, true},
		"two before": {Code(rfcSecret, at-2), 0, false},
		"short":      {"12345", 0, false},
	} {
		step, ok := Match(rfcSecret, tc.code, now)
		if ok != tc.ok || ok && step != tc.step {
			t.Errorf("%s: %d %v", name, step, ok)
		}
	}
}

// TestSecretAndURI - a new secret is 32 base32 letters; the address names
// the site and the account and carries the secret.
func TestSecretAndURI(t *testing.T) {
	s, err := NewSecret()
	if err != nil || len(s) != 32 || Code(s, 1) == "" {
		t.Fatalf("secret %q %v", s, err)
	}
	if other, _ := NewSecret(); other == s {
		t.Error("two secrets alike")
	}
	uri := URI("ABC", "Example Site", "ann")
	if !strings.HasPrefix(uri, "otpauth://totp/Example%20Site:ann?") || !strings.Contains(uri, "secret=ABC") || !strings.Contains(uri, "issuer=Example+Site") {
		t.Errorf("uri %s", uri)
	}
	if g := Grouped("ABCDEFGHIJ"); g != "ABCD EFGH IJ" {
		t.Errorf("grouped %q", g)
	}
	svg, err := QRSVG(uri)
	if err != nil || !strings.HasPrefix(svg, "<svg ") || !strings.Contains(svg, "M") {
		t.Errorf("qr %q %v", svg, err)
	}
}

// TestRecoveryCodes - ten distinct "xxxx-xxxx" codes of the alphabet;
// the hash ignores case, spaces and the dash.
func TestRecoveryCodes(t *testing.T) {
	codes, err := NewRecoveryCodes(RecoveryCodes)
	if err != nil || len(codes) != RecoveryCodes {
		t.Fatalf("codes %v %v", codes, err)
	}
	seen := map[string]bool{}
	for _, c := range codes {
		if len(c) != 9 || c[4] != '-' || strings.Trim(strings.ReplaceAll(c, "-", ""), recoveryAlphabet) != "" || seen[c] {
			t.Errorf("code %q", c)
		}
		seen[c] = true
	}
	if HashRecoveryCode("ab2c-de3f") != HashRecoveryCode(" AB2C DE3F ") || HashRecoveryCode("ab2c-de3f") == HashRecoveryCode("ab2c-de3g") {
		t.Error("hash")
	}
}
