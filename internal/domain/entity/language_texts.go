package entity

import (
	"context"
	"strconv"
	"sync"
)

// The site's own texts in its languages (the languages module sets
// these): a menu link's title, a block's - each a kind of text of a thing
// (ref, its id); whether a thing shows on a page in a language; a
// content item's address in a language (its translation's).

// Kinds of the site's own texts and things shown by language.
const (
	// TextMenuLinkTitle - a menu link's title (ref - its id).
	TextMenuLinkTitle = "menu_link.title"
	// VisibilityMenuLink - a menu link (ref - its id).
	VisibilityMenuLink = "menu_link"
	// TextBlockTitle - a custom block's title (ref - the block's id).
	TextBlockTitle = "block.title"
	// TextPlacementTitle - a block placement's title (ref - its id): a
	// View's, a menu's, a module's block.
	TextPlacementTitle = "block_placement.title"
	// VisibilityBlockPlacement - a block placement (ref - its id).
	VisibilityBlockPlacement = "block_placement"
	// TextTermName, TextTermDescription - a taxonomy term's name and
	// description (ref - its id).
	TextTermName        = "term.name"
	TextTermDescription = "term.description"
	// TextSiteName, TextSiteSlogan - the site's name and slogan (ref -
	// "site").
	TextSiteName   = "site.name"
	TextSiteSlogan = "site.slogan"
	// TextViewTitle, TextViewDescription - a View display's title (its
	// page's heading, its block's default title) and meta description
	// (ref - its id).
	TextViewTitle       = "view.title"
	TextViewDescription = "view.description"
	// TextFieldLabel - a content type's field's label (ref - the field
	// instance's id).
	TextFieldLabel = "field.label"
)

// SiteTextRef - the ref of the site's own texts (TextSiteName…).
const SiteTextRef = "site"

// LocalizeView - view in lang: its title and meta description
// translated; with its title translated, its meta title - the translated
// title (the meta title is one, in the default language).
func LocalizeView(lang string, view ViewDisplayDef) ViewDisplayDef {
	ref := strconv.Itoa(view.ID)
	label := TranslatedText(lang, TextViewTitle, ref, view.Label)
	if label != view.Label && view.MetaTitle != "" {
		view.MetaTitle = label
	}
	view.Label = label
	view.MetaDescription = TranslatedText(lang, TextViewDescription, ref, view.MetaDescription)
	return view
}

// LocalizeFieldLabels - fields with their labels in lang (a copy).
func LocalizeFieldLabels(lang string, fields FieldInstanceDefs) FieldInstanceDefs {
	out := make(FieldInstanceDefs, len(fields))
	for i, f := range fields {
		f.Label = TranslatedText(lang, TextFieldLabel, strconv.Itoa(f.ID), f.Label)
		out[i] = f
	}
	return out
}

// LocalizeTerm - term in lang: its name and description translated
// (TranslatedText); with its name translated, its page title - the
// translated name (the page title is one, in the default language).
func LocalizeTerm(lang string, term TaxonomyTerm) TaxonomyTerm {
	ref := strconv.Itoa(term.ID)
	name := TranslatedText(lang, TextTermName, ref, term.Name)
	if name != term.Name && term.PageTitle != "" {
		term.PageTitle = name
	}
	term.Name = name
	term.Desc = TranslatedText(lang, TextTermDescription, ref, term.Desc)
	return term
}

var languageHooks struct {
	mu       sync.RWMutex
	text     func(lang, kind, ref string) (string, bool)
	visible  func(kind, ref, lang string) bool
	nodeURLs func(ctx context.Context, ids []int, lang string) map[int]string
}

// SetTextTranslator sets what gives a text of the site in a language
// (false - none: the text as entered).
func SetTextTranslator(fn func(lang, kind, ref string) (string, bool)) {
	languageHooks.mu.Lock()
	defer languageHooks.mu.Unlock()
	languageHooks.text = fn
}

// TranslatedText - the text kind of ref in lang (SetTextTranslator), else
// fallback (the text as entered).
func TranslatedText(lang, kind, ref, fallback string) string {
	languageHooks.mu.RLock()
	fn := languageHooks.text
	languageHooks.mu.RUnlock()
	if fn != nil {
		if s, ok := fn(lang, kind, ref); ok && s != "" {
			return s
		}
	}
	return fallback
}

// SetLanguageVisibility sets what says whether a thing (kind, ref) shows
// on a page in lang.
func SetLanguageVisibility(fn func(kind, ref, lang string) bool) {
	languageHooks.mu.Lock()
	defer languageHooks.mu.Unlock()
	languageHooks.visible = fn
}

// VisibleInLanguage - the thing (kind, ref) shows on a page in lang
// (SetLanguageVisibility; none set - everywhere).
func VisibleInLanguage(kind, ref, lang string) bool {
	languageHooks.mu.RLock()
	fn := languageHooks.visible
	languageHooks.mu.RUnlock()
	return fn == nil || fn(kind, ref, lang)
}

// SetNodeURLsInLanguage sets what gives content items' addresses in a
// language: their public versions in it (translations).
func SetNodeURLsInLanguage(fn func(ctx context.Context, ids []int, lang string) map[int]string) {
	languageHooks.mu.Lock()
	defer languageHooks.mu.Unlock()
	languageHooks.nodeURLs = fn
}

// NodeURLsInLanguage - the addresses of the items among ids that have a
// public version in lang (SetNodeURLsInLanguage), by id; the others are
// left out.
func NodeURLsInLanguage(ctx context.Context, ids []int, lang string) map[int]string {
	languageHooks.mu.RLock()
	fn := languageHooks.nodeURLs
	languageHooks.mu.RUnlock()
	if fn == nil || len(ids) == 0 {
		return nil
	}
	return fn(ctx, ids, lang)
}
