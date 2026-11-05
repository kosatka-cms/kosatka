package languages

import (
	"context"
	"database/sql"
	"fmt"
	"slices"

	"github.com/kosatka-cms/kosatka/internal/modules/languages/ent/generated"
	"github.com/kosatka-cms/kosatka/internal/modules/languages/ent/generated/nodetranslation"
	"github.com/kosatka-cms/kosatka/modulekit"
)

// Member - a content item of a set of translations: Original - the
// set's original; Outdated - the original changed after it was last
// saved.
type Member struct {
	Nid      int
	Original bool
	Outdated bool
}

// TranslationRepo - the module's tables: the sets of translations and
// the settings.
type TranslationRepo struct {
	db *sql.DB
}

// NewTranslationRepo creates a TranslationRepo on the site's database.
func NewTranslationRepo(db *sql.DB) *TranslationRepo { return &TranslationRepo{db: db} }

// client - the module's client in ctx's transaction (modulekit.InTx),
// else on the database.
func (r *TranslationRepo) client(ctx context.Context) *generated.Client {
	return generated.NewClient(generated.Driver(modulekit.Driver(ctx, r.db)))
}

// Set - the set of translations nid is in: its members, the original
// first; none - nid has no translations.
func (r *TranslationRepo) Set(ctx context.Context, nid int) ([]Member, error) {
	c := r.client(ctx)
	row, err := c.NodeTranslation.Query().Where(nodetranslation.Nid(nid)).Only(ctx)
	if generated.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("loading the translations of node %d: %w", nid, err)
	}
	rows, err := c.NodeTranslation.Query().Where(nodetranslation.Tnid(row.Tnid)).Order(nodetranslation.ByID()).All(ctx)
	if err != nil {
		return nil, fmt.Errorf("loading the translations of node %d: %w", nid, err)
	}
	out := make([]Member, 0, len(rows))
	for _, t := range rows {
		out = append(out, Member{Nid: t.Nid, Original: t.Nid == t.Tnid, Outdated: t.Outdated})
	}
	slices.SortStableFunc(out, func(a, b Member) int {
		switch {
		case a.Original == b.Original:
			return 0
		case a.Original:
			return -1
		}
		return 1
	})
	return out, nil
}

// Add puts translation in the set of source (source's set - a new one
// with source as its original when source has none).
func (r *TranslationRepo) Add(ctx context.Context, source, translation int) error {
	c := r.client(ctx)
	tnid := source
	row, err := c.NodeTranslation.Query().Where(nodetranslation.Nid(source)).Only(ctx)
	switch {
	case err == nil:
		tnid = row.Tnid
	case generated.IsNotFound(err):
		if err := c.NodeTranslation.Create().SetNid(source).SetTnid(source).Exec(ctx); err != nil {
			return fmt.Errorf("starting the translations of node %d: %w", source, err)
		}
	default:
		return fmt.Errorf("loading the translations of node %d: %w", source, err)
	}
	if err := c.NodeTranslation.Create().SetNid(translation).SetTnid(tnid).Exec(ctx); err != nil {
		return fmt.Errorf("adding translation %d of node %d: %w", translation, source, err)
	}
	return nil
}

// Saved records that nid's own form saved it: an original's
// translations become outdated, a translation is up to date again.
func (r *TranslationRepo) Saved(ctx context.Context, nid int) error {
	c := r.client(ctx)
	row, err := c.NodeTranslation.Query().Where(nodetranslation.Nid(nid)).Only(ctx)
	if generated.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("loading the translations of node %d: %w", nid, err)
	}
	if row.Nid != row.Tnid {
		err = c.NodeTranslation.UpdateOne(row).SetOutdated(false).Exec(ctx)
	} else {
		err = c.NodeTranslation.Update().Where(nodetranslation.Tnid(row.Tnid), nodetranslation.NidNEQ(nid)).SetOutdated(true).Exec(ctx)
	}
	if err != nil {
		return fmt.Errorf("marking the translations of node %d: %w", nid, err)
	}
	return nil
}

// Settings - the module's settings: the content types whose items are
// translated; what a list in a language shows of the items without a
// version in it (Untranslated*).
type Settings struct {
	TranslatableTypes []string
	Untranslated      string
}

// What a list in a language shows of the items without a version in it.
const (
	// UntranslatedOriginal - their original (the default).
	UntranslatedOriginal = "original"
	// UntranslatedHide - nothing: the language's items only.
	UntranslatedHide = "hide"
	// UntranslatedAll - every item: lists aren't narrowed by language.
	UntranslatedAll = "all"
)

// untranslatedChoices - Settings.Untranslated's values.
var untranslatedChoices = []string{UntranslatedOriginal, UntranslatedHide, UntranslatedAll}

// Settings - the module's settings (the defaults while none are saved).
func (r *TranslationRepo) Settings(ctx context.Context) (Settings, error) {
	row, err := r.client(ctx).LanguageSettings.Get(ctx, 1)
	if generated.IsNotFound(err) {
		return Settings{Untranslated: UntranslatedOriginal}, nil
	}
	if err != nil {
		return Settings{}, fmt.Errorf("loading the languages settings: %w", err)
	}
	return Settings{TranslatableTypes: row.TranslatableTypes, Untranslated: row.Untranslated}, nil
}

// SaveSettings saves the module's settings.
func (r *TranslationRepo) SaveSettings(ctx context.Context, st Settings) error {
	c := r.client(ctx)
	err := c.LanguageSettings.UpdateOneID(1).SetTranslatableTypes(st.TranslatableTypes).SetUntranslated(st.Untranslated).Exec(ctx)
	if generated.IsNotFound(err) {
		err = c.LanguageSettings.Create().SetID(1).SetTranslatableTypes(st.TranslatableTypes).SetUntranslated(st.Untranslated).Exec(ctx)
	}
	if err != nil {
		return fmt.Errorf("saving the languages settings: %w", err)
	}
	return nil
}
