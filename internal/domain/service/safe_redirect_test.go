package service

import (
	"testing"
)

func TestSafeRedirectPath(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"/forum", "/forum"},
		{"forum", ""},
		{"//evil.com", ""},
		{`/\evil.com`, ""},
		{"/\t/evil.com", ""},
		{"/\n/evil.com", ""},
		{"/forum?page=2#top", "/forum?page=2#top"},
		{"", ""},
		{"  /me  ", "/me"},
	}
	for _, tc := range tests {
		if got := SafeRedirectPath(tc.in); got != tc.want {
			t.Errorf("SafeRedirectPath(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
