package languages

import (
	"context"
	"net/http"
	"slices"

	"github.com/kosatka-cms/kosatka/modulekit"
	"github.com/kosatka-cms/kosatka/pkg/htmlsanitize"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// textField - a text of a thing a section translates: its kind, its
// inputs' name (<Name>_<code>), its label (a translation source, shown
// when the section has several), a multi-line one; HTML - output as is,
// so sanitized on save as the core's own.
type textField struct {
	Kind      string
	Name      string
	Label     string
	Multiline bool
	HTML      bool
}

// textValue - a field's text in a language, as a form shows it.
type textValue struct {
	Field textField
	Value string
}

// textRow - a thing's texts in one of the site's other languages.
type textRow struct {
	Code, Name, Prefix string
	Values             []textValue
}

// languageOption - one of the site's languages a thing may be shown in.
type languageOption struct {
	Code, Name string
	On         bool
}

// textSection - a module section of a core admin form: texts of the
// thing (fields) in the site's other languages and, with visibility, the
// languages whose pages show it.
type textSection struct {
	texts      *Texts
	fields     []textField
	visibility string
}

// section - the form's section (modulekit.AddFormSection).
func (s textSection) section(module, form string) modulekit.FormSection {
	return modulekit.FormSection{Module: module, Form: form, Load: s.load, Save: s.save}
}

// load - the section's data: Texts - a row per other language, Field -
// the first field's name (the section's ids), Multi - several fields;
// Languages - every site language (with visibility), Only - some are
// chosen; nil on a site of one.
func (s textSection) load(ctx context.Context, ref string) (any, error) {
	def, others := modulekit.SiteLanguages()
	if len(others) == 0 {
		return nil, nil
	}
	values := make([]map[string]string, len(s.fields))
	for i, f := range s.fields {
		v, err := s.texts.Translations(ctx, f.Kind, ref)
		if err != nil {
			return nil, err
		}
		values[i] = v
	}
	rows := make([]textRow, 0, len(others))
	for _, l := range others {
		row := textRow{Code: l.Code, Name: i18n.NativeName(l.Code), Prefix: l.Prefix}
		for i, f := range s.fields {
			row.Values = append(row.Values, textValue{Field: f, Value: values[i][l.Code]})
		}
		rows = append(rows, row)
	}
	data := map[string]any{"Texts": rows, "Field": s.fields[0].Name, "Multi": len(s.fields) > 1}
	if s.visibility != "" {
		chosen, err := s.texts.Languages(ctx, s.visibility, ref)
		if err != nil {
			return nil, err
		}
		opts := []languageOption{{Code: def, Name: i18n.NativeName(def), On: slices.Contains(chosen, def)}}
		for _, l := range others {
			opts = append(opts, languageOption{Code: l.Code, Name: i18n.NativeName(l.Code), On: slices.Contains(chosen, l.Code)})
		}
		data["Languages"], data["Only"] = opts, len(chosen) > 0
	}
	return data, nil
}

// save stores the submitted texts (<field>_<code>) and languages
// (languages_only, languages).
func (s textSection) save(ctx context.Context, ref string, r *http.Request) error {
	def, others := modulekit.SiteLanguages()
	if len(others) == 0 {
		return nil
	}
	for _, f := range s.fields {
		values := make(map[string]string, len(others))
		for _, l := range others {
			v := r.FormValue(f.Name + "_" + l.Code)
			if f.HTML {
				v = htmlsanitize.SanitizeAdmin(v)
			}
			values[l.Code] = v
		}
		if err := s.texts.SaveTranslations(ctx, f.Kind, ref, values); err != nil {
			return err
		}
	}
	if s.visibility == "" {
		return nil
	}
	var langs []string
	if r.FormValue("languages_only") == "1" {
		for _, c := range r.Form["languages"] {
			if c == def || slices.ContainsFunc(others, func(l modulekit.SiteLanguage) bool { return l.Code == c }) {
				langs = append(langs, c)
			}
		}
	}
	return s.texts.SaveLanguages(ctx, s.visibility, ref, langs)
}
