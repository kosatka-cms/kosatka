package service

import (
	"html/template"
	"strconv"
	"strings"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/lib/numberformat"
)

// FieldNumber - a number field's value as the field_number_* templates
// show it: Value, and Formatted - by the field's kind (whole, its
// decimals) in the page's language, its prefix and suffix around; a
// number field without a kind (made before the setting) as stored.
type FieldNumber struct {
	Value     float64
	Formatted string
}

// newFieldNumber - v (a float64, or an int of a builtin count) of field
// in lang.
func newFieldNumber(field entity.FieldInstanceDef, v any, lang string) FieldNumber {
	var n float64
	switch x := v.(type) {
	case float64:
		n = x
	case int:
		n = float64(x)
	default:
		return FieldNumber{}
	}
	out := FieldNumber{Value: n}
	switch field.NumberKind() {
	case entity.NumberInteger:
		out.Formatted = numberformat.Format(n, 0, lang)
	case entity.NumberDecimal:
		out.Formatted = numberformat.Format(n, field.NumberDecimals(), lang)
	default:
		out.Formatted = strconv.FormatFloat(n, 'f', -1, 64)
	}
	if out.Formatted != "" {
		out.Formatted = field.NumberPrefix() + out.Formatted + field.NumberSuffix()
	}
	return out
}

// FieldEmail - an e-mail field's value as the field_email_* templates
// show it: Address; Hidden - shown by a script (app.js, from Reversed:
// the address backwards) as a mailto link, Masked ("info [at]
// example.com") without it.
type FieldEmail struct {
	Address  string
	Hidden   bool
	Reversed string
	Masked   string
}

func newFieldEmail(field entity.FieldInstanceDef, v any) FieldEmail {
	s, _ := v.(string)
	out := FieldEmail{Address: s, Hidden: s != "" && !field.EmailPlain()}
	if out.Hidden {
		r := []rune(s)
		for i, j := 0, len(r)-1; i < j; i, j = i+1, j-1 {
			r[i], r[j] = r[j], r[i]
		}
		out.Reversed = string(r)
		out.Masked = strings.Replace(s, "@", " [at] ", 1)
	}
	return out
}

// FieldPhone - a phone field's value as the field_phone_* templates show
// it: Number as typed, Tel - the tel: link of its digits (and a leading
// "+"; none without digits): a template.URL, html/template allowing
// http(s) and mailto only.
type FieldPhone struct {
	Number string
	Tel    template.URL
}

func newFieldPhone(v any) FieldPhone {
	s, _ := v.(string)
	var b strings.Builder
	if strings.HasPrefix(strings.TrimSpace(s), "+") {
		b.WriteByte('+')
	}
	digits := 0
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
			digits++
		}
	}
	out := FieldPhone{Number: s}
	if digits > 0 {
		// Only "+" and digits: safe in a link.
		out.Tel = template.URL("tel:" + b.String())
	}
	return out
}
