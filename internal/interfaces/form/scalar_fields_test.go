package form

import (
	"context"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

// TestScalarFields - a number by the field's kind and limits; an e-mail
// address alone with a domain; a phone of digits and its punctuation.
func TestScalarFields(t *testing.T) {
	ctx := context.Background()
	price := entity.FieldInstanceDef{Label: "Price", FieldType: entity.FieldTypeNumber,
		Config: map[string]any{"number_kind": "integer", "min": float64(0), "max": float64(1000)}}
	for raw, want := range map[string]string{
		"":     "",
		"500":  "",
		"12.5": "Price: a whole number.",
		"-1":   "Price: not less than 0.",
		"1001": "Price: not more than 1000.",
		"x":    "Enter a number",
		"NaN":  "Enter a number",
	} {
		if got := validateNumber(ctx, price, raw); got != want {
			t.Errorf("number %q: %q, want %q", raw, got, want)
		}
	}
	mail := entity.FieldInstanceDef{Label: "Mail", FieldType: entity.FieldTypeEmail}
	for raw, bad := range map[string]bool{"info@example.com": false, " a.b@sub.example.org ": false, "info@localhost": true, "Info <info@example.com>": true, "not an address": true} {
		if got := validateEmail(ctx, mail, raw); (got != "") != bad {
			t.Errorf("e-mail %q: %q", raw, got)
		}
	}
	phone := entity.FieldInstanceDef{Label: "Phone", FieldType: entity.FieldTypePhone}
	for raw, bad := range map[string]bool{"+7 (495) 123-45-67": false, "8.800.555.35.35": false, "1234": true, "+7 495 abc": true, "7+495123456": true} {
		if got := validatePhone(ctx, phone, raw); (got != "") != bad {
			t.Errorf("phone %q: %q", raw, got)
		}
	}
	decimal := entity.FieldInstanceDef{FieldType: entity.FieldTypeNumber, Config: map[string]any{"number_kind": "decimal", "decimals": float64(1)}}
	if got := decimal.RoundNumber(4.76); got != 4.8 || decimal.NumberStep() != "0.1" {
		t.Errorf("decimal: %v, step %s", got, decimal.NumberStep())
	}
}
