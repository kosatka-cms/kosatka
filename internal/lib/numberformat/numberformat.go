// Package numberformat shows numbers by a language's separators: the
// decimal one and the one between thousands, each a translation of its
// own (message contexts ContextDecimal, ContextThousands: "." and ","
// in English).
package numberformat

import (
	"math"
	"strconv"
	"strings"

	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// Message contexts of the separators.
const (
	ContextDecimal   = "Decimal separator"
	ContextThousands = "Thousands separator"
)

var (
	decimalSep   = i18n.NC("Decimal separator", ".")
	thousandsSep = i18n.NC("Thousands separator", ",")
)

// groupFrom - the digits of the whole part from which thousands are
// separated: 12 500, but 2015 (a year) as is.
const groupFrom = 5

// Format - v in lang with decimals digits after the separator (-1 - as
// many as it has): "12 500", "4,7" in Russian.
func Format(v float64, decimals int, lang string) string {
	if lang == "" {
		lang = i18n.Default()
	}
	s := strconv.FormatFloat(math.Abs(v), 'f', decimals, 64)
	whole, frac, _ := strings.Cut(s, ".")
	if len(whole) >= groupFrom {
		sep := i18n.Current().Text(lang, ContextThousands, thousandsSep)
		var b strings.Builder
		for i, r := range whole {
			if i > 0 && (len(whole)-i)%3 == 0 {
				b.WriteString(sep)
			}
			b.WriteRune(r)
		}
		whole = b.String()
	}
	out := whole
	if frac != "" {
		out += i18n.Current().Text(lang, ContextDecimal, decimalSep) + frac
	}
	if v < 0 {
		out = "−" + out
	}
	return out
}
