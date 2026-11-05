package languages

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/kosatka-cms/kosatka/modulekit"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// Link - a language of the switcher: its address for the page shown.
type Link struct {
	Code string
	// Name - the language's own name ("Deutsch"); Label - its short
	// one ("DE").
	Name  string
	Label string
	Href  string
	// Current - the page's language.
	Current bool
	// Home - the page has no version in it: Href is its front page.
	Home bool
}

// Alternate - the page's address in a language, for search engines
// (<link rel="alternate" hreflang>): Lang - the language's code, or
// "x-default" for the default language's address.
type Alternate struct {
	Lang string
	Href string
}

// TranslationInfo - a content item's place in its set of translations,
// for its form: Language - its language's name; Original - the original
// it translates (nil - it is the original); Outdated - the original
// changed since it was saved; Translations - the original's translations'
// languages.
type TranslationInfo struct {
	Language     string
	Original     *Version
	Outdated     bool
	Translations []string
}

// pageData - what every page gets of the site's languages.
type pageData struct {
	translations *Translations
	logger       *slog.Logger
}

// add adds the switcher's links (data["languageLinks"]) on a site of
// several languages; the page's addresses in them for search engines
// (data["languageAlternates"]) on the front page and on a content item's
// with public translations, for another page - languagePageAlternates,
// which head_languages shows on a View's or a term's; a content item's
// set of translations (data["translationInfo"]).
func (p pageData) add(r *http.Request, data map[string]any) {
	def, others := modulekit.SiteLanguages()
	if len(others) == 0 {
		return
	}
	ctx := r.Context()
	codes := []string{def}
	for _, l := range others {
		codes = append(codes, l.Code)
	}
	node, onNode := modulekit.RequestNode(r)
	var versions map[string]Version
	if onNode && p.translations != nil {
		vs, err := p.translations.Versions(ctx, node.ID)
		modulekit.LogFallback(p.logger, err, "error loading a node's translations", "nodeID", node.ID)
		versions = ByLanguage(vs)
		if info, ok := translationInfo(node.ID, vs); ok {
			data["translationInfo"] = info
		}
	}
	links := switcherLinks(codes, i18n.FromContext(ctx), r.URL.Path, onNode, versions)
	data["languageLinks"] = links
	base := strings.TrimRight(modulekit.SiteDomain(), "/")
	if base == "" {
		base = modulekit.PublicBaseURL(r)
	}
	var alternates []Alternate
	switch {
	case onNode:
		alternates = nodeAlternates(base, codes, versions)
	default:
		// A page the same in every language (its own content follows the
		// language): the front page here; a View's or a term's - their
		// templates know them (languagePageAlternates, head_languages).
		var all []Alternate
		for _, l := range links {
			all = append(all, Alternate{Lang: l.Code, Href: base + l.Href})
		}
		all = append(all, Alternate{Lang: "x-default", Href: base + modulekit.LocalPath(def, r.URL.Path)})
		if r.URL.Path == "/" {
			alternates = all
		} else {
			data["languagePageAlternates"] = all
		}
	}
	if len(alternates) > 0 {
		data["languageAlternates"] = alternates
	}
}

// switcherLinks - the switcher's links of path (without its prefix) shown
// in current: each of codes at its address. A content item's page
// (onNode) links its public translations (versions); a language without
// one - its front page.
func switcherLinks(codes []string, current, path string, onNode bool, versions map[string]Version) []Link {
	links := make([]Link, 0, len(codes))
	for _, code := range codes {
		l := Link{Code: code, Name: i18n.NativeName(code), Label: strings.ToUpper(code), Current: code == current, Href: modulekit.LocalPath(code, path)}
		if onNode && !l.Current {
			if v, ok := versions[code]; ok && v.Public() {
				l.Href = v.Node.URL()
			} else {
				l.Href, l.Home = modulekit.LocalPath(code, "/"), true
			}
		}
		links = append(links, l)
	}
	return links
}

// nodeAlternates - a content item's public versions' addresses (two at
// least), x-default - the default language's one.
func nodeAlternates(base string, codes []string, versions map[string]Version) []Alternate {
	var out []Alternate
	for _, code := range codes {
		if v, ok := versions[code]; ok && v.Public() {
			out = append(out, Alternate{Lang: code, Href: base + v.Node.URL()})
		}
	}
	if len(out) < 2 {
		return nil
	}
	if v, ok := versions[codes[0]]; ok && v.Public() {
		out = append(out, Alternate{Lang: "x-default", Href: base + v.Node.URL()})
	}
	return out
}

// translationInfo - nid's place in its set (versions); false - no set.
func translationInfo(nid int, versions []Version) (TranslationInfo, bool) {
	var info TranslationInfo
	var self, original *Version
	for i := range versions {
		v := &versions[i]
		switch {
		case v.Node.ID == nid:
			self = v
		case v.Original:
			original = v
		}
	}
	if self == nil || len(versions) < 2 {
		return info, false
	}
	info.Language = i18n.NativeName(self.Node.Langcode)
	if self.Original {
		for _, v := range versions {
			if v.Node.ID != nid {
				info.Translations = append(info.Translations, i18n.NativeName(v.Node.Langcode))
			}
		}
		return info, true
	}
	info.Original, info.Outdated = original, self.Outdated
	return info, true
}
