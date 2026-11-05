package service

import (
	"net/url"
	"strings"
)

// SafeRedirectPath returns a same-site relative path or an empty string -
// for every redirect to a user-supplied ?destination= (login, signup,
// OAuth). A browser reads "//host" and "/\host" as another site and drops
// tabs and line breaks before parsing ("/\t/host" is "//host"), so those
// are rejected, and the parsed URL must have no scheme or host.
func SafeRedirectPath(destination string) string {
	dest := strings.TrimSpace(destination)
	if !strings.HasPrefix(dest, "/") || strings.HasPrefix(dest, "//") || strings.HasPrefix(dest, `/\`) {
		return ""
	}
	if strings.ContainsFunc(dest, func(r rune) bool { return r < 0x20 || r == 0x7f }) {
		return ""
	}
	u, err := url.Parse(dest)
	if err != nil || u.Scheme != "" || u.Host != "" {
		return ""
	}
	return dest
}
