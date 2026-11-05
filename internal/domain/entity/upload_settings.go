package entity

import (
	"context"
	"strings"
)

// UploadSettings - site-wide upload validation policy (single DB row),
// applied by FileUploadService.ValidateAndProcess to every upload path.
// The avatar output size (MaxAvatarDimension) is a fixed constant instead:
// it is a crop target tied to the layout, not a site policy.
type UploadSettings struct {
	MaxFileSizeBytes  int
	MaxImageDimension int
	// AllowedMimeTypes - e.g. []string{"image/jpeg", "image/png"}.
	AllowedMimeTypes []string
	// UserFileExtensions - document extensions (lower case, no dot) users
	// may upload to their own files besides images, e.g. "pdf", "docx".
	UserFileExtensions []string
}

// UploadSettingsRepository persists the single upload-policy row.
type UploadSettingsRepository interface {
	Get(ctx context.Context) (*UploadSettings, error)
	Update(ctx context.Context, s *UploadSettings) error
}

// ForbiddenUserFileExtensions - extensions never accepted for an upload
// (a browser or the server could execute them), whatever the policy says.
var ForbiddenUserFileExtensions = map[string]bool{
	"htm": true, "html": true, "xhtml": true, "shtml": true, "svg": true, "svgz": true, "xml": true,
	"js": true, "mjs": true, "css": true, "php": true, "phtml": true, "phar": true, "cgi": true, "pl": true,
	"py": true, "sh": true, "exe": true, "dll": true, "bat": true, "cmd": true, "com": true, "msi": true,
	"jar": true, "swf": true, "htaccess": true,
}

// ParseFileExtensions splits a list of extensions (spaces or commas, an
// optional leading dot) into lower-case names; empty items are dropped.
func ParseFileExtensions(s string) []string {
	var out []string
	seen := map[string]bool{}
	for _, f := range strings.FieldsFunc(strings.ToLower(s), func(r rune) bool { return r == ' ' || r == ',' || r == '\n' || r == '\t' || r == ';' }) {
		f = strings.TrimPrefix(strings.TrimSpace(f), ".")
		if f == "" || seen[f] {
			continue
		}
		seen[f] = true
		out = append(out, f)
	}
	return out
}
