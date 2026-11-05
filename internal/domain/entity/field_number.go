package entity

import (
	"math"
	"strconv"
)

// NumberKind - what a number field holds: any number as typed (a field
// made before the setting), whole numbers, or numbers with a fixed count
// of decimals (NumberDecimals).
type NumberKind string

// The number kinds: any number, whole numbers, decimals.
const (
	NumberAny     NumberKind = ""
	NumberInteger NumberKind = "integer"
	NumberDecimal NumberKind = "decimal"
)

// NumberKind - FieldTypeNumber: storage config "number_kind"; unknown -
// any.
func (f FieldInstanceDef) NumberKind() NumberKind {
	switch k := NumberKind(stringConfig(f.Config, "number_kind")); k {
	case NumberInteger, NumberDecimal:
		return k
	}
	return NumberAny
}

// NumberDecimals - FieldTypeNumber with NumberDecimal: the decimals kept
// and shown (storage config "decimals", 0-6; unset - 2); 0 for the others.
func (f FieldInstanceDef) NumberDecimals() int {
	if f.NumberKind() != NumberDecimal {
		return 0
	}
	if v, ok := f.Config["decimals"].(float64); ok && v >= 0 && v <= 6 {
		return int(v)
	}
	if v, ok := f.Config["decimals"].(int); ok && v >= 0 && v <= 6 {
		return v
	}
	return 2
}

// NumberMin and NumberMax - FieldTypeNumber: the least and the greatest value
// allowed (storage config "min", "max"); false - no limit.
func (f FieldInstanceDef) NumberMin() (float64, bool) { return floatConfig(f.Config, "min") }

// NumberMax - see NumberMin.
func (f FieldInstanceDef) NumberMax() (float64, bool) { return floatConfig(f.Config, "max") }

// NumberMinAttr and NumberMaxAttr - NumberMin, NumberMax for the number
// input's min and max ("" - none).
func (f FieldInstanceDef) NumberMinAttr() string {
	if v, ok := f.NumberMin(); ok {
		return strconv.FormatFloat(v, 'f', -1, 64)
	}
	return ""
}

// NumberMaxAttr - see NumberMinAttr.
func (f FieldInstanceDef) NumberMaxAttr() string {
	if v, ok := f.NumberMax(); ok {
		return strconv.FormatFloat(v, 'f', -1, 64)
	}
	return ""
}

// NumberPrefix and NumberSuffix - FieldTypeNumber: what is shown before and
// after the value ("$", " ₽", " min"; storage config "prefix", "suffix").
func (f FieldInstanceDef) NumberPrefix() string { return stringConfig(f.Config, "prefix") }

// NumberSuffix - see NumberPrefix.
func (f FieldInstanceDef) NumberSuffix() string { return stringConfig(f.Config, "suffix") }

// NumberStep - the step of the number input: 1 for whole numbers, one
// unit of the last decimal for decimals, "any" otherwise.
func (f FieldInstanceDef) NumberStep() string {
	switch f.NumberKind() {
	case NumberInteger:
		return "1"
	case NumberDecimal:
		return strconv.FormatFloat(math.Pow10(-f.NumberDecimals()), 'f', -1, 64)
	}
	return "any"
}

// RoundNumber - v as the field keeps it: whole, or with its decimals; as
// is for any number.
func (f FieldInstanceDef) RoundNumber(v float64) float64 {
	switch f.NumberKind() {
	case NumberInteger:
		return math.Round(v)
	case NumberDecimal:
		p := math.Pow10(f.NumberDecimals())
		return math.Round(v*p) / p
	}
	return v
}

// EmailPlain - FieldTypeEmail: the address is shown as is, not hidden from
// harvesters (storage config "email_plain").
func (f FieldInstanceDef) EmailPlain() bool {
	b, _ := f.Config["email_plain"].(bool)
	return b
}

func stringConfig(config map[string]any, key string) string {
	s, _ := config[key].(string)
	return s
}

func floatConfig(config map[string]any, key string) (float64, bool) {
	switch v := config[key].(type) {
	case float64:
		return v, true
	case int:
		return float64(v), true
	}
	return 0, false
}
