package form

import (
	"context"
	"math"
	"net/mail"
	"strconv"
	"strings"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/lib/numberformat"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// validateNumber - what's wrong with a number field's input ("" -
// nothing): not a number, not whole where the field holds whole numbers,
// below its least or above its greatest value.
func validateNumber(ctx context.Context, field entity.FieldInstanceDef, raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	n, err := strconv.ParseFloat(raw, 64)
	if err != nil || math.IsNaN(n) || math.IsInf(n, 0) {
		return i18n.T(ctx, "Enter a number")
	}
	if field.NumberKind() == entity.NumberInteger && n != math.Trunc(n) {
		return i18n.T(ctx, "@field: a whole number.", "@field", field.Label)
	}
	lang := i18n.FromContext(ctx)
	if lo, ok := field.NumberMin(); ok && n < lo {
		return i18n.T(ctx, "@field: not less than @min.", "@field", field.Label, "@min", numberformat.Format(lo, -1, lang))
	}
	if hi, ok := field.NumberMax(); ok && n > hi {
		return i18n.T(ctx, "@field: not more than @max.", "@field", field.Label, "@max", numberformat.Format(hi, -1, lang))
	}
	return ""
}

// validateEmail - what's wrong with an e-mail field's input ("" -
// nothing): an address alone (no name), with a domain that has a dot.
func validateEmail(ctx context.Context, field entity.FieldInstanceDef, raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	a, err := mail.ParseAddress(raw)
	_, domain, _ := strings.Cut(raw, "@")
	if err != nil || a.Address != raw || a.Name != "" || !strings.Contains(domain, ".") || len(raw) > 254 {
		return i18n.T(ctx, "@field: \"@value\" is not an e-mail address.", "@field", field.Label, "@value", raw)
	}
	return ""
}

// phoneMinDigits - the least digits of a phone number.
const phoneMinDigits = 5

// validatePhone - what's wrong with a phone field's input ("" -
// nothing): only digits, spaces, "+", "-", "." and brackets, a "+" only
// first, at least phoneMinDigits digits.
func validatePhone(ctx context.Context, field entity.FieldInstanceDef, raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	digits := 0
	for i, r := range raw {
		switch {
		case r >= '0' && r <= '9':
			digits++
		case r == ' ' || r == '-' || r == '(' || r == ')' || r == '.':
		case r == '+' && i == 0:
		default:
			digits = -1
		}
		if digits < 0 {
			break
		}
	}
	if digits < phoneMinDigits || len(raw) > 40 {
		return i18n.T(ctx, "@field: \"@value\" is not a phone number.", "@field", field.Label, "@value", raw)
	}
	return ""
}
