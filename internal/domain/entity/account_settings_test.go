package entity

import (
	"slices"
	"testing"
)

func TestAccountSettingsBlockedDomains(t *testing.T) {
	a := AccountSettings{RegistrationBlockedDomains: " comcast.net \n\n# comment\n@Foo.bar"}
	if got := a.BlockedDomains(); !slices.Equal(got, []string{"comcast.net", "foo.bar"}) {
		t.Fatalf("got %v", got)
	}
	if got := (AccountSettings{}).BlockedDomains(); len(got) != 0 {
		t.Fatalf("empty: %v", got)
	}
}

func TestAccountSettingsEmailDomainBlocked(t *testing.T) {
	a := AccountSettings{RegistrationBlockedDomains: "comcast.net\n"}
	for email, want := range map[string]bool{
		"user@comcast.net":      true,
		"User@COMCAST.NET":      true,
		"":                      false,
		"invalid":               false,
		"a@":                    false,
		"user@mail.comcast.net": true,
		"user@notcomcast.net":   false,
		"user@evilcomcast.net":  false,
	} {
		if got := a.EmailDomainBlocked(email); got != want {
			t.Errorf("%q: got %v want %v", email, got, want)
		}
	}
	if (AccountSettings{}).EmailDomainBlocked("u@bad.com") {
		t.Error("an empty list blocks nothing")
	}
}
