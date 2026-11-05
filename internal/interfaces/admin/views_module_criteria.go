package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/views"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// viewModuleOption - a module's sort criterion, filter or field offered
// by the View page's modals (views.AddModuleSort…): its target
// ("builtin:<key>"), its label and its params, translated; Operators -
// a filter's, for admin_views.js.
type viewModuleOption struct {
	Target    string
	Key       string
	Label     string
	Operators string
	Params    []entity.ViewModuleParam
	// Options - a choice filter's values (JSON [{value, label}]), "" -
	// a number filter.
	Options string
}

// translatedModuleParams - params with their labels and their options'
// translated.
func translatedModuleParams(ctx context.Context, params []entity.ViewModuleParam) []entity.ViewModuleParam {
	out := make([]entity.ViewModuleParam, len(params))
	for i, p := range params {
		p.Label = i18n.T(ctx, p.Label)
		if len(p.Options) > 0 {
			opts := make([]entity.PluginBlockTarget, len(p.Options))
			for j, o := range p.Options {
				opts[j] = entity.PluginBlockTarget{Value: o.Value, Label: i18n.T(ctx, o.Label)}
			}
			p.Options = opts
		}
		out[i] = p
	}
	return out
}

func moduleOption(ctx context.Context, key, label string, params []entity.ViewModuleParam) viewModuleOption {
	return viewModuleOption{Target: "builtin:" + key, Key: key, Label: i18n.T(ctx, label), Params: translatedModuleParams(ctx, params)}
}

// addViewModuleCriteria adds to data what the modules on add to a content
// View: their sort criteria, filters and fields for the modals and every
// one's label by key for the lists (moduleLabels). A comment View gets
// none.
func addViewModuleCriteria(ctx context.Context, data map[string]any, view *entity.ViewDisplayDef) {
	labels := map[string]string{}
	var sorts, filters, fields []viewModuleOption
	if view == nil || !view.IsComment() {
		for _, s := range views.ModuleSorts() {
			o := moduleOption(ctx, s.Key, s.Label, s.Params)
			sorts = append(sorts, o)
			labels[s.Key] = o.Label
		}
		for _, f := range views.ModuleFilters() {
			o := moduleOption(ctx, f.Key, f.Label, nil)
			o.Operators = operatorsAttr(entity.ViewModuleFilterOperators)
			if f.Choice() {
				o.Operators = operatorsAttr(entity.ViewModuleChoiceOperators)
				if opts, err := f.Options(ctx); err == nil {
					if b, err := json.Marshal(opts); err == nil {
						o.Options = string(b)
					}
				}
			}
			filters = append(filters, o)
			labels[f.Key] = o.Label
		}
		for _, f := range views.ModuleFields() {
			o := moduleOption(ctx, f.Key, f.Label, f.Params)
			fields = append(fields, o)
			labels[f.Key] = o.Label
		}
	}
	data["moduleSorts"] = sorts
	data["moduleFilters"] = filters
	data["moduleFields"] = fields
	data["moduleLabels"] = labels
}

// moduleParamsFromForm - the values of params from the form
// ("param_<key>"); false when one isn't a value its param accepts.
func moduleParamsFromForm(r *http.Request, params []entity.ViewModuleParam) (map[string]string, bool) {
	if len(params) == 0 {
		return nil, true
	}
	out := make(map[string]string, len(params))
	for _, p := range params {
		v := strings.TrimSpace(r.FormValue("param_" + p.Key))
		if !p.Check(v) {
			return nil, false
		}
		out[p.Key] = v
	}
	return out, true
}
