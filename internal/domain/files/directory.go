package files

import (
	"errors"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/kosatka-cms/kosatka/pkg/fsname"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
	"github.com/kosatka-cms/kosatka/pkg/sitetime"
)

// FileDirectoryTokens - values for the tokens of a field's upload
// directory (the "File directory" setting of a file/image field).
type FileDirectoryTokens struct {
	Now         time.Time
	ContentType string // machine name of the node's content type
	UserID      int    // uploading user
}

// ErrInvalidFileDirectory - a field's upload directory setting is not
// usable (unknown token, "..", absolute path, reserved directory...).
var ErrInvalidFileDirectory = errors.New("invalid file directory")

// fileDirectoryError - an ErrInvalidFileDirectory with the user-facing
// reason (i18n.ErrorText shows it translated).
type fileDirectoryError struct{ reason error }

func (e fileDirectoryError) Error() string   { return e.reason.Error() }
func (e fileDirectoryError) Unwrap() []error { return []error{ErrInvalidFileDirectory, e.reason} }

func invalidFileDirectory(reason error) error { return fileDirectoryError{reason: reason} }

var fileDirectoryTokenRe = regexp.MustCompile(`\[[^\[\]]*\]`)

// reservedFileDirectories - first path segments whose meaning other code
// relies on: image style derivatives, resize cache, default images, user
// avatars (removed on user delete) - plus u<id> user folders, see
// isReservedFileDirectory.
var reservedFileDirectories = map[string]bool{
	"styles":                    true,
	"resize":                    true,
	"imagefield_default_images": true,
	"user_pictures":             true,
}

// fileDirectoryTokenValue returns a token's value, ok=false for an
// unknown token.
func fileDirectoryTokenValue(token string, t FileDirectoryTokens) (string, bool) {
	now := sitetime.In(t.Now)
	switch token {
	case "[date:custom:Y]":
		return now.Format("2006"), true
	case "[date:custom:m]":
		return now.Format("01"), true
	case "[date:custom:d]":
		return now.Format("02"), true
	case "[node:type]":
		return t.ContentType, true
	case "[current-user:uid]":
		return strconv.Itoa(t.UserID), true
	}
	return "", false
}

// ResolveFileDirectory turns a field's upload directory pattern into a
// storage-relative directory: tokens substituted, every segment
// transliterated/sanitized (fsname.NormalizeFolderSegment). Empty
// pattern means defaultDir.
func ResolveFileDirectory(pattern, defaultDir string, t FileDirectoryTokens) (string, error) {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return defaultDir, nil
	}
	var unknown string
	resolved := fileDirectoryTokenRe.ReplaceAllStringFunc(pattern, func(tok string) string {
		v, ok := fileDirectoryTokenValue(tok, t)
		if !ok && unknown == "" {
			unknown = tok
		}
		return v
	})
	if unknown != "" {
		return "", invalidFileDirectory(i18n.Errorf("Invalid file directory: unknown token @token", "@token", unknown))
	}
	return cleanFileDirectory(resolved)
}

// ValidateFileDirectory checks a pattern before it is saved (sample
// token values; the same rules ResolveFileDirectory applies).
func ValidateFileDirectory(pattern string) error {
	_, err := ResolveFileDirectory(pattern, "", FileDirectoryTokens{Now: time.Now(), ContentType: "type", UserID: 1})
	return err
}

func cleanFileDirectory(dir string) (string, error) {
	// Leading/trailing slashes are dropped; a drive or
	// scheme ("C:", "public:") is not a relative directory.
	if strings.Contains(dir, ":") {
		return "", invalidFileDirectory(i18n.Errorf("Invalid file directory: the path must be relative"))
	}
	raw := strings.Split(strings.Trim(strings.ReplaceAll(dir, `\`, "/"), "/"), "/")
	segments := make([]string, 0, len(raw))
	for _, seg := range raw {
		if strings.TrimSpace(seg) == ".." {
			return "", invalidFileDirectory(i18n.Errorf("Invalid file directory: \"..\" is not allowed"))
		}
		clean := fsname.NormalizeFolderSegment(seg)
		if clean == "" {
			return "", invalidFileDirectory(i18n.Errorf("Invalid file directory: empty path segment in \"@dir\"", "@dir", dir))
		}
		segments = append(segments, clean)
	}
	if isReservedFileDirectory(segments[0]) {
		return "", invalidFileDirectory(i18n.Errorf("Invalid file directory: directory \"@dir\" is reserved", "@dir", segments[0]))
	}
	return strings.Join(segments, "/"), nil
}

func isReservedFileDirectory(first string) bool {
	if reservedFileDirectories[strings.ToLower(first)] {
		return true
	}
	// u<id> - personal user folders (UserDirName), removed with the user.
	if rest, ok := strings.CutPrefix(first, UserRichTextFilesPrefix); ok && rest != "" {
		if _, err := strconv.Atoi(rest); err == nil {
			return true
		}
	}
	return false
}
