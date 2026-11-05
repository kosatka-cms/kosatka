package interfaces

import (
	"strings"

	"github.com/kosatka-cms/kosatka/pkg/textcut"
)

// truncateMeta - s for a meta description: trimmed, cut to n characters
// with "…".
func truncateMeta(s string, n int) string { return textcut.Ellipsis(strings.TrimSpace(s), n) }
