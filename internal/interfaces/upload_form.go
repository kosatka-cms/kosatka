package interfaces

import (
	"strconv"
	"strings"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

// parseExtensionsForm splits the space-separated extensions input
// ("jpg jpeg png"), lowercasing and dropping leading dots; forbidden ones
// (entity.ForbiddenUserFileExtensions: pages, scripts, programs) are
// dropped.
func parseExtensionsForm(raw string) []string {
	fields := strings.Fields(raw)
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		f = strings.ToLower(strings.TrimPrefix(f, "."))
		// Pages, scripts and programs can't be allowed anywhere.
		if f != "" && !entity.ForbiddenUserFileExtensions[f] {
			out = append(out, f)
		}
	}
	return out
}

// maxFileSizeBytesFromMBForm parses a megabyte form input into bytes, or 0
// when unset/invalid (callers treat 0 as "use the sitewide default").
func maxFileSizeBytesFromMBForm(raw string) int {
	mb, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil || mb <= 0 {
		return 0
	}
	return int(mb * 1024 * 1024)
}
