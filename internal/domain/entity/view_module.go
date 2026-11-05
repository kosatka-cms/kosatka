package entity

import (
	"context"
	"html/template"
	"strconv"
	"strings"
)

// A module adds its own sort criteria, filters and fields to content
// Views (views.AddModuleSort/AddModuleFilter/AddModuleField): a rating, a
// vote count. A View stores one by its Key in the row's BuiltinField, like
// a builtin; the module computes the values for a batch of nodes.

// ViewModuleKeyMaxLen - the longest Key (the builtin_field column).
const ViewModuleKeyMaxLen = 32

// ViewModuleParam - one setting of a module's sort criterion or field in
// a View (the threshold of a weighted rating, how a rating is shown):
// the same kinds as a plugin block's setting. Its value is kept with the
// View's row (ViewSortDef.Params, ViewFieldDef.Params) under Key.
type ViewModuleParam = PluginBlockSetting

// ViewModuleSort - a sort criterion a module adds to content Views.
type ViewModuleSort struct {
	// Module - the module's name: the criterion is offered and applied
	// only while it is on.
	Module string
	// Key - "<module>.<name>", at most ViewModuleKeyMaxLen characters.
	Key string
	// Label - the English name, translated where shown.
	Label  string
	Params []ViewModuleParam
	// Values - each node's sort key among ids by the View's params (every
	// param set: its value, else its Default); a node left out sorts as
	// 0.
	Values func(ctx context.Context, ids []int, params map[string]string) (map[int]float64, error)
}

// ViewModuleFilter - a number a module lets content Views filter by
// (ViewModuleFilterOperators); with Options - one of a list instead
// (ViewModuleChoiceOperators: the value is one, or several
// comma-separated), the nodes' values from Choices.
type ViewModuleFilter struct {
	Module string
	Key    string
	Label  string
	// Values - the nodes' values among ids; a node left out has none
	// (the "empty" operator matches it, the others don't).
	Values func(ctx context.Context, ids []int) (map[int]float64, error)
	// Options - the values to choose from (a choice filter); Choices -
	// the nodes' values among ids, a node left out has none.
	Options func(ctx context.Context) ([]ViewModuleOption, error)
	Choices func(ctx context.Context, ids []int) (map[int]string, error)
}

// ViewModuleOption - a value of a choice filter and its label.
type ViewModuleOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// Choice - the filter is one of a list (Options), not a number.
func (f ViewModuleFilter) Choice() bool { return f.Options != nil && f.Choices != nil }

// ViewModuleChoiceOperators - what a choice filter compares: its value
// with the filter's (eq - one, in - one of a comma-separated list), or
// whether the node has one.
var ViewModuleChoiceOperators = []ViewFilterOperator{
	ViewFilterOperatorEq,
	ViewFilterOperatorIn,
	ViewFilterOperatorEmpty,
	ViewFilterOperatorNotEmpty,
}

// ViewModuleChoiceMatch - whether a node's value v (has - it has one)
// passes a choice filter's operator against value ("" - no constraint).
func ViewModuleChoiceMatch(op ViewFilterOperator, value, v string, has bool) bool {
	switch op {
	case ViewFilterOperatorEmpty:
		return !has
	case ViewFilterOperatorNotEmpty:
		return has
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return true
	}
	if !has {
		return false
	}
	for _, want := range strings.Split(value, ",") {
		if strings.TrimSpace(want) == v {
			return true
		}
	}
	return false
}

// ViewModuleField - a value a module shows in a content View's rows.
type ViewModuleField struct {
	Module string
	Key    string
	Label  string
	Params []ViewModuleParam
	// Render - each node's markup among ids by the View field's params; a
	// node left out shows nothing.
	Render func(ctx context.Context, ids []int, params map[string]string) (map[int]template.HTML, error)
}

// ViewModuleFilterOperators - what a module's filter compares: its number
// with the filter's Value, or whether the node has one.
var ViewModuleFilterOperators = []ViewFilterOperator{
	ViewFilterOperatorEq,
	ViewFilterOperatorGte,
	ViewFilterOperatorLte,
	ViewFilterOperatorEmpty,
	ViewFilterOperatorNotEmpty,
}

// ViewModuleParams - the values of params from stored: each param's own
// when the param accepts it, else its Default. A value of no param is
// dropped.
func ViewModuleParams(params []ViewModuleParam, stored map[string]string) map[string]string {
	out := make(map[string]string, len(params))
	for _, p := range params {
		if v, ok := stored[p.Key]; ok && p.Check(v) {
			out[p.Key] = v
		} else {
			out[p.Key] = p.Default
		}
	}
	return out
}

// ViewModuleFilterMatch - whether a node's value (has - it has one)
// passes a module filter's operator against value (a number; "" or not a
// number - no constraint, the node passes).
func ViewModuleFilterMatch(op ViewFilterOperator, value string, v float64, has bool) bool {
	switch op {
	case ViewFilterOperatorEmpty:
		return !has
	case ViewFilterOperatorNotEmpty:
		return has
	}
	want, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
	if err != nil {
		return true
	}
	if !has {
		return false
	}
	switch op {
	case ViewFilterOperatorEq:
		return v == want
	case ViewFilterOperatorGte:
		return v >= want
	case ViewFilterOperatorLte:
		return v <= want
	}
	return true
}
