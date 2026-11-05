package form

import (
	"context"
	"strings"
	"time"

	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// validateSlug applies the shared node slug rules: max length, allowed
// charset, no leading/trailing or double hyphen. Returns the error message
// and true if the slug fails a rule; "" and false if it is empty or valid
// (slugs are optional). If several rules fail, the last one checked wins.
func validateSlug(ctx context.Context, slug string) (string, bool) {
	if slug == "" {
		return "", false
	}

	message, failed := "", false

	if len(slug) > 255 {
		message, failed = i18n.T(ctx, "The URL alias must not exceed 255 characters"), true
	}
	for _, r := range slug {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '-' && r != '/' {
			message, failed = i18n.T(ctx, "The URL alias may contain only lowercase Latin letters, digits, hyphens and slashes"), true
			break
		}
	}
	if strings.HasPrefix(slug, "-") || strings.HasSuffix(slug, "-") {
		message, failed = i18n.T(ctx, "The URL alias cannot start or end with a hyphen"), true
	}
	if strings.Contains(slug, "--") {
		message, failed = i18n.T(ctx, "The URL alias cannot contain consecutive hyphens"), true
	}

	return message, failed
}

// validateRequiredTrimmed reports whether value is empty after trimming
// whitespace, returning message verbatim if so. The caller supplies the
// full message since wording differs per form.
func validateRequiredTrimmed(value, message string) (string, bool) {
	if strings.TrimSpace(value) == "" {
		return message, true
	}
	return "", false
}

// validateMaxLen reports whether value exceeds limit bytes, returning
// message verbatim if so.
func validateMaxLen(value string, limit int, message string) (string, bool) {
	if len(value) > limit {
		return message, true
	}
	return "", false
}

// validateDateYMD reports whether a non-empty value fails to parse as
// "2006-01-02", returning message verbatim if so. Empty values pass -
// every date field this is used for is optional.
func validateDateYMD(value, message string) (string, bool) {
	if value == "" {
		return "", false
	}
	if _, err := time.Parse("2006-01-02", value); err != nil {
		return message, true
	}
	return "", false
}
