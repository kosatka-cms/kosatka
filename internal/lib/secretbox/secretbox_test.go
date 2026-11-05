package secretbox

import (
	"errors"
	"strings"
	"testing"
)

// TestSealOpen - a sealed value opens with its key, differs each time,
// tells nothing of the plain one; a plain one opens as it is (to seal); no
// key seals nothing.
func TestSealOpen(t *testing.T) {
	box, err := New("a-key-of-sixteen-or-more")
	if err != nil {
		t.Fatal(err)
	}
	a, err := box.Seal("re_123_secret")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := box.Seal("re_123_secret")
	if !Sealed(a) || a == b || strings.Contains(a, "secret") {
		t.Errorf("sealed %q, %q", a, b)
	}
	if plain, current, err := box.Open(a); err != nil || plain != "re_123_secret" || !current {
		t.Errorf("open: %q %v %v", plain, current, err)
	}
	if plain, current, err := box.Open("plain"); err != nil || plain != "plain" || current {
		t.Errorf("a plain value with a key: %q %v %v, want it to seal", plain, current, err)
	}
	if s, err := box.Seal(""); err != nil || s != "" {
		t.Errorf("an empty value sealed: %q %v", s, err)
	}

	none, err := New("")
	if err != nil {
		t.Fatal(err)
	}
	if s, _ := none.Seal("plain"); s != "plain" {
		t.Errorf("no key sealed: %q", s)
	}
	if plain, current, err := none.Open("plain"); plain != "plain" || !current || err != nil {
		t.Errorf("no key, a plain value: %q %v %v", plain, current, err)
	}
	if _, _, err := none.Open(a); !errors.Is(err, ErrNoKey) {
		t.Errorf("no key, a sealed value: %v", err)
	}
	var nilBox *Box
	if s, _ := nilBox.Seal("plain"); s != "plain" || nilBox.Enabled() {
		t.Errorf("a nil box sealed: %q", s)
	}
}

// TestKeys - the first key seals, an older one after it still opens (the
// value then to seal again); a wrong key opens nothing; a short key is
// refused.
func TestKeys(t *testing.T) {
	old, _ := New("the-old-key-0123456789")
	sealed, _ := old.Seal("secret")
	both, err := New("the-new-key-0123456789, the-old-key-0123456789")
	if err != nil {
		t.Fatal(err)
	}
	if plain, current, err := both.Open(sealed); err != nil || plain != "secret" || current {
		t.Errorf("by the old key: %q %v %v, want it to seal again", plain, current, err)
	}
	resealed, _ := both.Seal("secret")
	if _, _, err := old.Open(resealed); !errors.Is(err, ErrWrongKey) {
		t.Errorf("sealed by the new key, opened by the old: %v", err)
	}
	other, _ := New("another-key-0123456789")
	if _, _, err := other.Open(sealed); !errors.Is(err, ErrWrongKey) {
		t.Errorf("a wrong key: %v", err)
	}
	if _, err := New("short"); err == nil {
		t.Error("a short key accepted")
	}
}

// TestWeak - a placeholder or a key of few different characters is weak;
// a random one isn't; one weak key of several makes the box weak.
func TestWeak(t *testing.T) {
	for key, want := range map[string]bool{
		"aaaaaaaaaaaaaaaaaaaaaaaa":                                         true,
		"1234567890123456":                                                 true,
		"change-me-local-only-secrets-0123456789":                          true,
		"my-password-is-long-enough":                                       true,
		"9f2c4e7a1b8d3f60c5e2a9b7d4f1c8e3a6b0d5f2e9c7a4b1d8f3e6c0a5b2d9f7": false,
		"Qx7#mP2v!Lr9@tZ4":                                                 false,
	} {
		b, err := New(key)
		if err != nil {
			t.Fatal(err)
		}
		if b.Weak() != want {
			t.Errorf("Weak(%q) = %v, want %v", key, b.Weak(), want)
		}
	}
	b, _ := New("9f2c4e7a1b8d3f60c5e2a9b7d4f1c8e3a6b0d5f2e9c7a4b1d8f3e6c0a5b2d9f7,aaaaaaaaaaaaaaaaaaaa")
	if !b.Weak() {
		t.Error("an old weak key after a strong one")
	}
	if none, _ := New(""); none.Weak() {
		t.Error("no key is weak")
	}
}
