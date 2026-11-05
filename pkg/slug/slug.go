package slug

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"github.com/kosatka-cms/kosatka/pkg/translit"
	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

// Generate creates a URL-safe slug from the input string
// It transliterates Cyrillic to Latin, converts to lowercase, and replaces spaces with hyphens
func Generate(input string) string {
	if input == "" {
		return ""
	}

	// Transliterate Cyrillic to Latin
	var result strings.Builder
	for _, r := range input {
		if latin, ok := translit.Latin(r); ok {
			result.WriteString(latin)
		} else {
			result.WriteRune(r)
		}
	}

	slug := result.String()

	// Normalize unicode (NFD decomposition)
	t := transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)
	slug, _, _ = transform.String(t, slug)

	slug = strings.ToLower(slug)

	// Replace spaces and underscores with hyphens
	slug = strings.ReplaceAll(slug, " ", "-")
	slug = strings.ReplaceAll(slug, "_", "-")

	// Remove all non-alphanumeric characters except hyphens
	reg := regexp.MustCompile("[^a-z0-9-]+")
	slug = reg.ReplaceAllString(slug, "")

	// Replace multiple consecutive hyphens with a single hyphen
	reg = regexp.MustCompile("-+")
	slug = reg.ReplaceAllString(slug, "-")

	// Trim hyphens from start and end
	slug = strings.Trim(slug, "-")

	return slug
}

// machineNameMax - how long MachineName makes a name: room for a
// "_<n>" suffix within the usual 64.
const machineNameMax = 56

// MachineName - a machine name ([a-z][a-z0-9_]*) from input, a title:
// Generate's transliteration with underscores; one not starting with a
// letter gets prefix (and prefix alone stands for an empty one).
func MachineName(input, prefix string) string {
	name := strings.ReplaceAll(Generate(input), "-", "_")
	if len(name) > machineNameMax {
		name = strings.TrimRight(name[:machineNameMax], "_")
	}
	if name == "" {
		return prefix
	}
	if name[0] < 'a' || name[0] > 'z' {
		name = prefix + "_" + name
	}
	return name
}

// SlugChecker is an interface for checking if a slug exists
type SlugChecker interface {
	SlugExists(ctx context.Context, slug, contentType string) (bool, error)
}

// EnsureUnique ensures the slug is unique by appending a numeric suffix if needed
func EnsureUnique(ctx context.Context, baseSlug, contentType string, checker SlugChecker) (string, error) {
	if baseSlug == "" {
		return "", fmt.Errorf("base slug cannot be empty")
	}

	// Check if the base slug is available
	exists, err := checker.SlugExists(ctx, baseSlug, contentType)
	if err != nil {
		return "", fmt.Errorf("error checking slug existence: %w", err)
	}

	if !exists {
		return baseSlug, nil
	}

	// Try appending numbers until we find a unique slug
	for i := 2; i < 1000; i++ {
		candidate := fmt.Sprintf("%s-%d", baseSlug, i)
		exists, err := checker.SlugExists(ctx, candidate, contentType)
		if err != nil {
			return "", fmt.Errorf("error checking slug existence: %w", err)
		}
		if !exists {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("could not generate unique slug after 1000 attempts")
}

// Validate checks if a slug is valid
func Validate(slug string) error {
	if slug == "" {
		return fmt.Errorf("slug cannot be empty")
	}

	if len(slug) > 255 {
		return fmt.Errorf("slug cannot be longer than 255 characters")
	}

	// Check if slug contains only valid characters
	reg := regexp.MustCompile("^[a-z0-9-]+$")
	if !reg.MatchString(slug) {
		return fmt.Errorf("slug can only contain lowercase letters, numbers, and hyphens")
	}

	// Check if slug starts or ends with hyphen
	if strings.HasPrefix(slug, "-") || strings.HasSuffix(slug, "-") {
		return fmt.Errorf("slug cannot start or end with a hyphen")
	}

	if strings.Contains(slug, "--") {
		return fmt.Errorf("slug cannot contain consecutive hyphens")
	}

	return nil
}
