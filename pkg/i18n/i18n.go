// Package i18n translates the interface (t()/formatPlural()):
// source strings are English, catalogs give their translations per
// language. A lookup goes down a chain - the active theme's catalog,
// then the engine's, then the modules' - and falls back to the source
// string itself.
//
// Catalog file (JSON, one per language):
//
//	{
//	  "strings": {
//	    "Save": "Speichern",
//	    "@count view": ["@count Aufruf", "@count Aufrufe"]
//	  },
//	  "contexts": {
//	    "Long month name": {"May": "Mai"}
//	  }
//	}
//
// A plural entry is keyed by the singular source string and holds one
// form per plural category of the language (PluralIndex).
//
// Placeholders: @name - the value HTML-escaped; %name -
// escaped and wrapped in <em class="placeholder">; :name - a URL
// (unsafe schemes dropped, non-ASCII percent-encoded, then escaped). @count in plural strings is the number.
package i18n

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"sort"
	"strings"
	"sync"

	"golang.org/x/text/feature/plural"
	"golang.org/x/text/language"
)

// SourceLanguage - the language of the source strings.
const SourceLanguage = "en"

// entry - one translation: a single string or plural forms.
type entry struct {
	text  string
	forms []string
}

func (e *entry) UnmarshalJSON(b []byte) error {
	if len(b) > 0 && b[0] == '[' {
		return json.Unmarshal(b, &e.forms)
	}
	return json.Unmarshal(b, &e.text)
}

type catalogFile struct {
	Strings  map[string]entry            `json:"strings"`
	Contexts map[string]map[string]entry `json:"contexts"`
}

// catalog - one language's translations from one source (engine or
// theme), keyed by context + "\x04" + source.
type catalog map[string]entry

func key(ctx, source string) string {
	if ctx == "" {
		return source
	}
	return ctx + "\x04" + source
}

func parseCatalog(b []byte) (catalog, error) {
	var f catalogFile
	if err := json.Unmarshal(b, &f); err != nil {
		return nil, err
	}
	c := catalog{}
	for s, e := range f.Strings {
		c[key("", s)] = e
	}
	for ctx, m := range f.Contexts {
		for s, e := range m {
			c[key(ctx, s)] = e
		}
	}
	return c, nil
}

// Translator - the catalogs of every language, in lookup order (the
// theme's first, then the engine's, then the modules', then the
// downloaded ones - WithPacks), and the site's own translations over them
// (WithSite).
type Translator struct {
	chain map[string][]catalog
	// site - the site's own translations by language (WithSite): looked
	// up first.
	site map[string]catalog
}

// Entry - a translation kept outside the catalog files (a site's own or
// a downloaded one, in its database): its language, context and source,
// its text or its plural forms (keyed by the singular source, as in a
// catalog).
type Entry struct {
	Lang, Context, Source string
	Text                  string
	Forms                 []string
}

// WithSite - t with the site's own translations over its catalogs: a
// copy, t is left as it is. A source the site rewords in English
// (SourceLanguage) reads so in a language that doesn't translate it.
func (t *Translator) WithSite(entries []Entry) *Translator {
	out := &Translator{chain: map[string][]catalog{}, site: map[string]catalog{}}
	if t != nil {
		out.chain = t.chain
	}
	for _, e := range entries {
		c := out.site[e.Lang]
		if c == nil {
			c = catalog{}
			out.site[e.Lang] = c
		}
		c[key(e.Context, e.Source)] = entry{text: e.Text, forms: e.Forms}
	}
	return out
}

// WithPacks - t with the downloaded translations (packs) as the last
// catalog of their languages: a copy, t is left as it is. A pack is of a
// language the built-in catalogs don't have, so for it the theme's
// catalog comes first and the pack after it.
func (t *Translator) WithPacks(entries []Entry) *Translator {
	out := &Translator{chain: map[string][]catalog{}}
	if t != nil {
		for lang, cs := range t.chain {
			out.chain[lang] = slices.Clone(cs)
		}
		out.site = t.site
	}
	packs := map[string]catalog{}
	for _, e := range entries {
		c := packs[e.Lang]
		if c == nil {
			c = catalog{}
			packs[e.Lang] = c
		}
		c[key(e.Context, e.Source)] = entry{text: e.Text, forms: e.Forms}
	}
	for lang, c := range packs {
		out.chain[lang] = append(out.chain[lang], c)
	}
	return out
}

// ParseCatalog - the entries of a catalog file (a <lang>.json) of lang,
// sorted by context and source.
func ParseCatalog(lang string, b []byte) ([]Entry, error) {
	c, err := parseCatalog(b)
	if err != nil {
		return nil, fmt.Errorf("i18n: the %s catalog: %w", lang, err)
	}
	out := make([]Entry, 0, len(c))
	for k, e := range c {
		ctx, source, ok := strings.Cut(k, "\x04")
		if !ok {
			ctx, source = "", k
		}
		out = append(out, Entry{Lang: lang, Context: ctx, Source: source, Text: e.text, Forms: e.forms})
	}
	slices.SortFunc(out, func(a, b Entry) int {
		if c := strings.Compare(a.Context, b.Context); c != 0 {
			return c
		}
		return strings.Compare(a.Source, b.Source)
	})
	return out, nil
}

// Default - the translation of source (in context ctx) to lang by the
// catalogs alone, the site's own left out: its text or plural forms;
// false - none.
func (t *Translator) Default(lang, ctx, source string) (string, []string, bool) {
	if t == nil {
		return "", nil, false
	}
	for _, c := range t.chain[lang] {
		if e, ok := c[key(ctx, source)]; ok {
			return e.text, e.forms, true
		}
	}
	return "", nil, false
}

// Site - the site's own translation of source to lang (WithSite).
func (t *Translator) Site(lang, ctx, source string) (string, []string, bool) {
	if t == nil {
		return "", nil, false
	}
	e, ok := t.site[lang][key(ctx, source)]
	return e.text, e.forms, ok
}

// Load builds a Translator from catalog directories in lookup order
// (e.g. the theme's, the engine's, the modules'): each holds <lang>.json files;
// a missing directory is skipped.
func Load(dirs ...fs.FS) (*Translator, error) {
	t := &Translator{chain: map[string][]catalog{}}
	for _, dir := range dirs {
		if dir == nil {
			continue
		}
		names, err := fs.Glob(dir, "*.json")
		if err != nil {
			return nil, err
		}
		for _, name := range names {
			b, err := fs.ReadFile(dir, name)
			if err != nil {
				return nil, err
			}
			c, err := parseCatalog(b)
			if err != nil {
				return nil, fmt.Errorf("i18n: %s: %w", name, err)
			}
			lang := strings.TrimSuffix(name, ".json")
			t.chain[lang] = append(t.chain[lang], c)
		}
	}
	return t, nil
}

// DirFS - a catalog directory on disk (nil when it doesn't exist).
func DirFS(dir string) fs.FS {
	if dir == "" {
		return nil
	}
	if st, err := os.Stat(dir); err != nil || !st.IsDir() {
		return nil
	}
	return os.DirFS(filepath.Clean(dir))
}

// Languages - the languages with a catalog, sorted.
func (t *Translator) Languages() []string {
	out := make([]string, 0, len(t.chain))
	for l := range t.chain {
		out = append(out, l)
	}
	sort.Strings(out)
	return out
}

func (t *Translator) lookup(lang, ctx, source string) (entry, bool) {
	if t == nil {
		return entry{}, false
	}
	if e, ok := t.site[lang][key(ctx, source)]; ok {
		return e, true
	}
	for _, c := range t.chain[lang] {
		if e, ok := c[key(ctx, source)]; ok {
			return e, true
		}
	}
	return entry{}, false
}

// lookupOrSite - lookup, else the site's English wording of the source
// (it rewords it for every language that doesn't translate it).
func (t *Translator) lookupOrSite(lang, ctx, source string) (entry, bool) {
	if e, ok := t.lookup(lang, ctx, source); ok {
		return e, true
	}
	if t == nil || lang == SourceLanguage {
		return entry{}, false
	}
	e, ok := t.site[SourceLanguage][key(ctx, source)]
	return e, ok
}

// Has - source (in context ctx) has a translation for lang.
func (t *Translator) Has(lang, ctx, source string) bool {
	_, ok := t.lookup(lang, ctx, source)
	return ok
}

// T - source translated to lang, placeholders filled from args
// (name/value pairs: "@name", value, …). The result is HTML (values are
// escaped; the catalogs are trusted) - for templates.
func (t *Translator) T(lang, ctx, source string, args ...any) string {
	return Format(t.text(lang, ctx, source), args...)
}

// Text - T as plain text: values are not escaped, %name is not wrapped -
// for Go code, whose strings are escaped where they are output.
func (t *Translator) Text(lang, ctx, source string, args ...any) string {
	return FormatText(t.text(lang, ctx, source), args...)
}

func (t *Translator) text(lang, ctx, source string) string {
	if e, ok := t.lookupOrSite(lang, ctx, source); ok {
		switch {
		case e.text != "":
			return e.text
		case len(e.forms) > 0:
			return e.forms[0]
		}
	}
	return source
}

// Plural - the form of singular/plural for count in lang: the catalog's
// forms of singular (PluralIndex), else the English source forms.
// @count is count; args as for T. The result is HTML.
func (t *Translator) Plural(lang, ctx string, count int, singular, plural string, args ...any) string {
	return Format(t.pluralText(lang, ctx, count, singular, plural), append([]any{"@count", count}, args...)...)
}

// PluralText - Plural as plain text (see Text).
func (t *Translator) PluralText(lang, ctx string, count int, singular, plural string, args ...any) string {
	return FormatText(t.pluralText(lang, ctx, count, singular, plural), append([]any{"@count", count}, args...)...)
}

func (t *Translator) pluralText(lang, ctx string, count int, singular, plural string) string {
	text := plural
	if count == 1 {
		text = singular
	}
	if e, ok := t.lookupOrSite(lang, ctx, singular); ok && len(e.forms) > 0 {
		formsLang := lang
		if _, own := t.lookup(lang, ctx, singular); !own {
			// The site's English forms: English plural rules.
			formsLang = SourceLanguage
		}
		i := PluralIndex(formsLang, count)
		if i >= len(e.forms) {
			i = len(e.forms) - 1
		}
		text = e.forms[i]
	}
	return text
}

// PluralIndex - the plural form index of count in lang: its CLDR plural
// category's place among the categories lang uses for whole numbers
// (PluralCategories) - Russian 1 / 2-4 / 5+ (with 11-14), Polish, Czech,
// French and the rest by their own CLDR rules.
func PluralIndex(lang string, count int) int {
	n := count
	if n < 0 {
		n = -n
	}
	r := pluralRulesFor(lang)
	form := plural.Cardinal.MatchPlural(r.tag, n, 0, 0, 0, 0)
	for i, f := range r.forms {
		if f == form {
			return i
		}
	}
	return len(r.forms) - 1
}

// PluralCategories - the CLDR plural categories lang uses for whole
// numbers, in CLDR order (zero, one, two, few, many, other): the order of
// a plural entry's forms in its catalog ("one", "few", "many" for
// Russian; "one", "other" for English).
func PluralCategories(lang string) []string {
	r := pluralRulesFor(lang)
	out := make([]string, len(r.forms))
	for i, f := range r.forms {
		out[i] = pluralFormNames[f]
	}
	return out
}

var pluralFormNames = map[plural.Form]string{plural.Zero: "zero", plural.One: "one", plural.Two: "two", plural.Few: "few", plural.Many: "many", plural.Other: "other"}

// pluralRules - a language's tag and the categories it uses for whole
// numbers.
type pluralRules struct {
	tag   language.Tag
	forms []plural.Form
}

var (
	pluralMu    sync.Mutex
	pluralCache = map[string]pluralRules{}
)

// pluralRulesFor - lang's rules, worked out once: the categories whole
// numbers fall in (a sample wide enough for every CLDR rule's integer
// cases), in CLDR order. An unknown language - English rules.
func pluralRulesFor(lang string) pluralRules {
	key := base(lang)
	pluralMu.Lock()
	defer pluralMu.Unlock()
	if r, ok := pluralCache[key]; ok {
		return r
	}
	tag, err := language.Parse(key)
	if err != nil || key == "" {
		tag = language.English
	}
	seen := map[plural.Form]bool{}
	sample := func(n int) { seen[plural.Cardinal.MatchPlural(tag, n, 0, 0, 0, 0)] = true }
	for n := 0; n <= 1000; n++ {
		sample(n)
	}
	for _, n := range []int{10000, 100000, 1000000, 2000000, 10000000, 1000000000} {
		sample(n)
	}
	var forms []plural.Form
	for _, f := range []plural.Form{plural.Zero, plural.One, plural.Two, plural.Few, plural.Many, plural.Other} {
		if seen[f] {
			forms = append(forms, f)
		}
	}
	r := pluralRules{tag: tag, forms: forms}
	pluralCache[key] = r
	return r
}

func base(lang string) string {
	if i := strings.IndexAny(lang, "-_"); i > 0 {
		return strings.ToLower(lang[:i])
	}
	return strings.ToLower(lang)
}

// Format fills the @name/%name/:name placeholders in text from args (name/value
// pairs) for HTML: values escaped, %name wrapped in <em class="placeholder">.
func Format(text string, args ...any) string {
	return format(text, true, args)
}

// FormatText fills the placeholders as plain text (values as is).
func FormatText(text string, args ...any) string {
	return format(text, false, args)
}

func format(text string, asHTML bool, args []any) string {
	if len(args) < 2 {
		return text
	}
	pairs := make([]string, 0, len(args))
	for i := 0; i+1 < len(args); i += 2 {
		name, ok := args[i].(string)
		if !ok || name == "" {
			continue
		}
		value := printValue(args[i+1])
		if name[0] == ':' {
			value = safeURL(value)
		}
		if asHTML {
			value = html.EscapeString(value)
		}
		switch name[0] {
		case '%':
			if asHTML {
				value = `<em class="placeholder">` + value + `</em>`
			}
		case '@', ':':
		default:
			continue
		}
		pairs = append(pairs, name, value)
	}
	if len(pairs) == 0 {
		return text
	}
	// Longest names first, so @count isn't replaced inside @countdown.
	type kv struct{ k, v string }
	kvs := make([]kv, 0, len(pairs)/2)
	for i := 0; i < len(pairs); i += 2 {
		kvs = append(kvs, kv{pairs[i], pairs[i+1]})
	}
	sort.Slice(kvs, func(i, j int) bool { return len(kvs[i].k) > len(kvs[j].k) })
	flat := make([]string, 0, len(pairs))
	for _, p := range kvs {
		flat = append(flat, p.k, p.v)
	}
	return strings.NewReplacer(flat...).Replace(text)
}

// The site's translator and default language - set at startup.
var (
	mu          sync.RWMutex
	current     = &Translator{chain: map[string][]catalog{}}
	defaultLang = SourceLanguage
)

// SetTranslator makes t the site's translator.
func SetTranslator(t *Translator) {
	mu.Lock()
	current = t
	mu.Unlock()
	resetJS()
}

// Current - the site's translator.
func Current() *Translator {
	mu.RLock()
	defer mu.RUnlock()
	return current
}

// SetDefault makes lang the site's default language (from the site settings).
func SetDefault(lang string) {
	if lang == "" {
		lang = SourceLanguage
	}
	mu.Lock()
	defaultLang = lang
	mu.Unlock()
}

// Default - the site's default language.
func Default() string {
	mu.RLock()
	defer mu.RUnlock()
	return defaultLang
}

type ctxKey struct{}

// WithLang - ctx carrying lang as the request's language.
func WithLang(ctx context.Context, lang string) context.Context {
	return context.WithValue(ctx, ctxKey{}, lang)
}

// FromContext - the request's language, else the site's default.
func FromContext(ctx context.Context) string {
	if ctx != nil {
		if l, ok := ctx.Value(ctxKey{}).(string); ok && l != "" {
			return l
		}
	}
	return Default()
}

// T - source in the request's language as plain text, for Go code
// (Translator.Text with the site's translator); escaped where output.
func T(ctx context.Context, source string, args ...any) string {
	return Current().Text(FromContext(ctx), "", source, args...)
}

// TC - T with a context (msgctxt), for strings that read differently in
// different places.
func TC(ctx context.Context, msgctx, source string, args ...any) string {
	return Current().Text(FromContext(ctx), msgctx, source, args...)
}

// Plural - Translator.PluralText in the request's language.
func Plural(ctx context.Context, count int, singular, plural string, args ...any) string {
	return Current().PluralText(FromContext(ctx), "", count, singular, plural, args...)
}

// N marks source as an interface string without translating it (like
// gettext_noop): for labels kept in data - maps, struct fields, defaults -
// and translated where shown, with T(ctx, label) or {{t .Label}}.
// cmd/i18n-extract finds i18n.N("…") like i18n.T.
func N(source string) string { return source }

// NC - N with a message context: i18n.NC("Abbreviated month name", "May").
func NC(msgctx, source string) string { return source }

// Error - an error whose message is an interface string: Error() is the
// English text (logs), ErrorText translates it for the user.
type Error struct {
	Source string
	Args   []any
}

func (e *Error) Error() string { return FormatText(e.Source, e.Args...) }

// Errorf - an Error with placeholders, for messages a service returns to
// be shown to the user: i18n.Errorf("User @name already exists.",
// "@name", name).
func Errorf(source string, args ...any) error { return &Error{Source: source, Args: args} }

// ErrorText - err's message for the user in the request's language: the
// translation of an Error anywhere in err's chain; any other error gets a
// message of its own - its text (paths, queries, addresses) is for the
// log, not for visitors.
func ErrorText(ctx context.Context, err error) string {
	if err == nil {
		return ""
	}
	var e *Error
	if errors.As(err, &e) {
		return T(ctx, e.Source, e.Args...)
	}
	return T(ctx, "The operation failed. Try again.")
}

// IsUserError - whether err carries a message for the user (an Error in
// its chain): a check the input failed rather than a failure of the site.
func IsUserError(err error) bool {
	var e *Error
	return errors.As(err, &e)
}

// nativeNames - language names in the language itself, for pickers.
var nativeNames = map[string]string{
	"ar": "العربية", "az": "Azərbaycan", "be": "Беларуская", "bg": "Български",
	"bn": "বাংলা", "ca": "Català", "cs": "Čeština", "da": "Dansk",
	"de": "Deutsch", "el": "Ελληνικά", "en": "English", "es": "Español",
	"et": "Eesti", "fa": "فارسی", "fi": "Suomi", "fr": "Français",
	"ga": "Gaeilge", "he": "עברית", "hi": "हिन्दी", "hr": "Hrvatski",
	"hu": "Magyar", "hy": "Հայերեն", "id": "Bahasa Indonesia", "is": "Íslenska",
	"it": "Italiano", "ja": "日本語", "ka": "ქართული", "kk": "Қазақ тілі",
	"ko": "한국어", "ky": "Кыргызча", "lt": "Lietuvių", "lv": "Latviešu",
	"mk": "Македонски", "mn": "Монгол", "ms": "Bahasa Melayu", "nl": "Nederlands",
	"no": "Norsk", "pl": "Polski", "pt": "Português", "ro": "Română",
	"ru": "Русский", "sk": "Slovenčina", "sl": "Slovenščina", "sq": "Shqip",
	"sr": "Српски", "sv": "Svenska", "sw": "Kiswahili", "tg": "Тоҷикӣ",
	"th": "ไทย", "tk": "Türkmençe", "tr": "Türkçe", "tt": "Татарча",
	"ur": "اردو", "uz": "Oʻzbekcha", "vi": "Tiếng Việt",
	"zh": "中文",
}

// NativeName - the language's name in the language itself ("Deutsch");
// the code for one it doesn't know.
func NativeName(code string) string {
	if n := nativeNames[base(code)]; n != "" {
		return n
	}
	return code
}

// KnownLanguages - the languages it has names for, by code: what a
// site's content may be in (its interface - only Options').
func KnownLanguages() []LanguageOption {
	out := make([]LanguageOption, 0, len(nativeNames))
	for c, n := range nativeNames {
		out = append(out, LanguageOption{Code: c, Label: n})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Code < out[j].Code })
	return out
}

// LanguageOption - a language for a select: its code and native name.
type LanguageOption struct {
	Code  string
	Label string
}

// Options - the languages the interface can be shown in: the source
// language and every language with a catalog.
func (t *Translator) Options() []LanguageOption {
	codes := []string{SourceLanguage}
	for _, l := range t.Languages() {
		if l != SourceLanguage {
			codes = append(codes, l)
		}
	}
	out := make([]LanguageOption, 0, len(codes))
	for _, c := range codes {
		out = append(out, LanguageOption{Code: c, Label: NativeName(c) + " (" + c + ")"})
	}
	return out
}

// Available - lang is one of Options.
func (t *Translator) Available(lang string) bool {
	for _, o := range t.Options() {
		if o.Code == lang {
			return true
		}
	}
	return false
}

// safeURL - a URL for an href: "" for a scheme other than http, https
// or mailto (javascript: etc.), non-ASCII bytes and spaces
// percent-encoded, like html/template does for URL attributes.
func safeURL(u string) string {
	if i := strings.IndexAny(u, ":/?#"); i > 0 && u[i] == ':' {
		switch strings.ToLower(u[:i]) {
		case "http", "https", "mailto":
		default:
			return ""
		}
	}
	var b strings.Builder
	for i := 0; i < len(u); i++ {
		c := u[i]
		if c >= 0x80 || c <= ' ' {
			fmt.Fprintf(&b, "%%%02X", c)
			continue
		}
		b.WriteByte(c)
	}
	return b.String()
}

// printValue - v as text the way templates print it: pointers are
// dereferenced (a nil one prints nothing), not printed as addresses.
func printValue(v any) string {
	rv := reflect.ValueOf(v)
	for rv.Kind() == reflect.Pointer || rv.Kind() == reflect.Interface {
		if rv.IsNil() {
			return ""
		}
		rv = rv.Elem()
	}
	if !rv.IsValid() {
		return ""
	}
	return fmt.Sprint(rv.Interface())
}
