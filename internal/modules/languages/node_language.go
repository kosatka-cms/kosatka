package languages

import (
	"context"
	"net/http"
	"net/url"

	"github.com/kosatka-cms/kosatka/modulekit"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// languageChoice - a language a content item can be in.
type languageChoice struct {
	Code, Name string
}

// nodeLanguage - the Language section of a content item's form: its
// language, of the site's; one another version of its set has is not
// offered. A new item is in the address's language.
type nodeLanguage struct {
	nodes        modulekit.NodesRepository
	translations *Translations
}

// section - the form's section (modulekit.AddNodeSettings).
func (l nodeLanguage) section(module string) modulekit.NodeSettings {
	return modulekit.NodeSettings{Module: module, Load: l.load, Save: l.save}
}

// load - the section's data: Current - the item's language (submitted,
// else its own, else the address's), Choices; nil on a site of one.
func (l nodeLanguage) load(ctx context.Context, _ string, node *modulekit.Node, submitted url.Values) (any, error) {
	choices, err := l.choices(ctx, node)
	if err != nil || len(choices) < 2 {
		return nil, err
	}
	current := i18n.FromContext(ctx)
	if node != nil && node.Langcode != "" {
		current = node.Langcode
	}
	if v := submitted.Get("langcode"); v != "" {
		current = v
	}
	return map[string]any{"Current": current, "CurrentName": i18n.NativeName(current), "Choices": choices}, nil
}

// choices - the site's languages, the default first, but those another
// version of node's set has.
func (l nodeLanguage) choices(ctx context.Context, node *modulekit.Node) ([]languageChoice, error) {
	def, others := modulekit.SiteLanguages()
	if len(others) == 0 {
		return nil, nil
	}
	taken := map[string]bool{}
	if node != nil && node.ID > 0 {
		versions, err := l.translations.Versions(ctx, node.ID)
		if err != nil {
			return nil, err
		}
		for _, v := range versions {
			if v.Node.ID != node.ID {
				taken[v.Node.Langcode] = true
			}
		}
	}
	codes := []string{def}
	for _, o := range others {
		codes = append(codes, o.Code)
	}
	var out []languageChoice
	for _, c := range codes {
		if !taken[c] {
			out = append(out, languageChoice{Code: c, Name: i18n.NativeName(c)})
		}
	}
	return out, nil
}

// save sets the item's language to the submitted one when it may be in
// it (else it keeps its own).
func (l nodeLanguage) save(ctx context.Context, node modulekit.Node, r *http.Request) error {
	code := r.FormValue("langcode")
	if code == "" {
		return nil
	}
	choices, err := l.choices(ctx, &node)
	if err != nil {
		return err
	}
	ok := false
	for _, c := range choices {
		ok = ok || c.Code == code
	}
	if !ok {
		return nil
	}
	// Through the core: the item's words for the search follow its
	// language.
	return l.nodes.SetLangcode(ctx, node.ID, code)
}
