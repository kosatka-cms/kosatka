package languages

import (
	"context"
	"database/sql"
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/kosatka-cms/kosatka/internal/modules/languages/ent/generated"
	"github.com/kosatka-cms/kosatka/internal/modules/languages/ent/generated/languagevisibility"
	"github.com/kosatka-cms/kosatka/internal/modules/languages/ent/generated/texttranslation"
	"github.com/kosatka-cms/kosatka/modulekit"
)

// textKey - a text of a thing: its kind and ref.
type textKey struct{ kind, ref string }

// Texts - the site's own texts in its other languages and the things
// shown only in some languages: read once into memory (every page asks),
// kept until changed here.
type Texts struct {
	db *sql.DB

	mu      sync.RWMutex
	loaded  bool
	texts   map[textKey]map[string]string
	visible map[textKey][]string
}

// NewTexts creates Texts on the site's database.
func NewTexts(db *sql.DB) *Texts { return &Texts{db: db} }

func (t *Texts) client(ctx context.Context) *generated.Client {
	return generated.NewClient(generated.Driver(modulekit.Driver(ctx, t.db)))
}

// load reads the texts and the visibility once; the caller holds no lock.
func (t *Texts) load(ctx context.Context) error {
	t.mu.RLock()
	loaded := t.loaded
	t.mu.RUnlock()
	if loaded {
		return nil
	}
	c := t.client(ctx)
	rows, err := c.TextTranslation.Query().All(ctx)
	if err != nil {
		return fmt.Errorf("loading the texts' translations: %w", err)
	}
	vis, err := c.LanguageVisibility.Query().All(ctx)
	if err != nil {
		return fmt.Errorf("loading the language visibility: %w", err)
	}
	texts := map[textKey]map[string]string{}
	for _, r := range rows {
		k := textKey{r.Kind, r.Ref}
		if texts[k] == nil {
			texts[k] = map[string]string{}
		}
		texts[k][r.Lang] = r.Value
	}
	visible := map[textKey][]string{}
	for _, v := range vis {
		visible[textKey{v.Kind, v.Ref}] = v.Langs
	}
	t.mu.Lock()
	t.texts, t.visible, t.loaded = texts, visible, true
	t.mu.Unlock()
	return nil
}

// Text - kind of ref in lang (false - none).
func (t *Texts) Text(ctx context.Context, lang, kind, ref string) (string, bool, error) {
	if err := t.load(ctx); err != nil {
		return "", false, err
	}
	t.mu.RLock()
	defer t.mu.RUnlock()
	s, ok := t.texts[textKey{kind, ref}][lang]
	return s, ok, nil
}

// Translations - kind of ref in each language it has, by language.
func (t *Texts) Translations(ctx context.Context, kind, ref string) (map[string]string, error) {
	if err := t.load(ctx); err != nil {
		return nil, err
	}
	t.mu.RLock()
	defer t.mu.RUnlock()
	out := map[string]string{}
	for l, s := range t.texts[textKey{kind, ref}] {
		out[l] = s
	}
	return out, nil
}

// SaveTranslations sets kind of ref in the languages of values ("" - none
// in it: the text as entered).
func (t *Texts) SaveTranslations(ctx context.Context, kind, ref string, values map[string]string) error {
	if err := t.load(ctx); err != nil {
		return err
	}
	c := t.client(ctx)
	for lang, v := range values {
		v = strings.TrimSpace(v)
		if _, err := c.TextTranslation.Delete().Where(texttranslation.Kind(kind), texttranslation.Ref(ref), texttranslation.Lang(lang)).Exec(ctx); err != nil {
			return fmt.Errorf("saving the %s of %s in %s: %w", kind, ref, lang, err)
		}
		if v == "" {
			continue
		}
		if err := c.TextTranslation.Create().SetKind(kind).SetRef(ref).SetLang(lang).SetValue(v).Exec(ctx); err != nil {
			return fmt.Errorf("saving the %s of %s in %s: %w", kind, ref, lang, err)
		}
	}
	t.mu.Lock()
	k := textKey{kind, ref}
	if t.texts[k] == nil {
		t.texts[k] = map[string]string{}
	}
	for lang, v := range values {
		if v = strings.TrimSpace(v); v == "" {
			delete(t.texts[k], lang)
		} else {
			t.texts[k][lang] = v
		}
	}
	t.mu.Unlock()
	return nil
}

// Visible - the thing (kind, ref) shows on a page in lang: no languages
// chosen for it, or lang among them.
func (t *Texts) Visible(ctx context.Context, kind, ref, lang string) (bool, error) {
	langs, err := t.Languages(ctx, kind, ref)
	return len(langs) == 0 || slices.Contains(langs, lang), err
}

// Languages - the languages chosen for the thing (none - every one).
func (t *Texts) Languages(ctx context.Context, kind, ref string) ([]string, error) {
	if err := t.load(ctx); err != nil {
		return nil, err
	}
	t.mu.RLock()
	defer t.mu.RUnlock()
	return slices.Clone(t.visible[textKey{kind, ref}]), nil
}

// SaveLanguages sets the languages whose pages show the thing (none -
// every one's).
func (t *Texts) SaveLanguages(ctx context.Context, kind, ref string, langs []string) error {
	if err := t.load(ctx); err != nil {
		return err
	}
	c := t.client(ctx)
	if _, err := c.LanguageVisibility.Delete().Where(languagevisibility.Kind(kind), languagevisibility.Ref(ref)).Exec(ctx); err != nil {
		return fmt.Errorf("saving the languages of %s %s: %w", kind, ref, err)
	}
	if len(langs) > 0 {
		if err := c.LanguageVisibility.Create().SetKind(kind).SetRef(ref).SetLangs(langs).Exec(ctx); err != nil {
			return fmt.Errorf("saving the languages of %s %s: %w", kind, ref, err)
		}
	}
	t.mu.Lock()
	if len(langs) > 0 {
		t.visible[textKey{kind, ref}] = slices.Clone(langs)
	} else {
		delete(t.visible, textKey{kind, ref})
	}
	t.mu.Unlock()
	return nil
}

// nodeURLs - the addresses of the items among ids with a public version
// in lang (themselves, or a translation), by id.
func nodeURLs(ctx context.Context, db *sql.DB, ids []int, lang string) (map[int]string, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	args := make([]any, 0, len(ids)+1)
	marks := make([]string, len(ids))
	for i, id := range ids {
		marks[i] = "?"
		args = append(args, id)
	}
	args = append(args, lang)
	// An item itself in lang keeps its address; one in a set - its
	// version's in lang (the core's nodes read: see Repo).
	q := `SELECT t1.nid, n.type, n.slug, n.langcode FROM node_translations t1
		JOIN node_translations t2 ON t2.tnid = t1.tnid
		JOIN nodes n ON n.id = t2.nid
		WHERE t1.nid IN (` + strings.Join(marks, ", ") + `) AND n.langcode = ?
		AND n.status = 1 AND n.on_moderation = 0 AND n.deleted_at IS NULL`
	rows, err := db.QueryContext(ctx, modulekit.Rebind(q), args...)
	if err != nil {
		return nil, fmt.Errorf("the addresses of nodes in %s: %w", lang, err)
	}
	defer func() { _ = rows.Close() }()
	out := map[int]string{}
	for rows.Next() {
		var nid int
		var n modulekit.Node
		var typ string
		if err := rows.Scan(&nid, &typ, &n.Slug, &n.Langcode); err != nil {
			return nil, fmt.Errorf("the addresses of nodes in %s: %w", lang, err)
		}
		n.Type = modulekit.ContentType(typ)
		out[nid] = n.URL()
	}
	return out, rows.Err()
}
