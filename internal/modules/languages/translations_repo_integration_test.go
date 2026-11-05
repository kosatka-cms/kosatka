//go:build dbintegration

package languages

import (
	"context"
	"testing"

	"github.com/kosatka-cms/kosatka/modulekit/testkit"
)

// TestTranslationSets - a translation starts its original's set, the
// next joins it (from a translation too); Versions lists them with their
// languages, the original first; the original saved marks the others
// outdated, a translation saved is up to date; the original deleted,
// the translations stay a set; the settings are kept.
func TestTranslationSets(t *testing.T) {
	testkit.SchemaDB(t, func(t *testing.T, db testkit.DB) {
		ctx := context.Background()
		node := func(id int, slug, lang string) {
			t.Helper()
			if _, err := db.DB.Exec(db.Dialect.Rebind(`INSERT INTO nodes (id, type, title, slug, status, langcode) VALUES (?, 'article', ?, ?, 1, ?)`), id, slug, "article/"+slug, lang); err != nil {
				t.Fatal(err)
			}
		}
		node(1, "sea", "en")
		node(2, "more", "ru")
		node(3, "meer", "de")
		repo := NewTranslationRepo(db.DB)
		tr := NewTranslations(repo, testkit.Deps(db).NodeRepo)
		if vs, err := tr.Versions(ctx, 1); err != nil || vs != nil {
			t.Fatalf("no set yet: %v %v", vs, err)
		}
		if err := repo.Add(ctx, 1, 2); err != nil {
			t.Fatal(err)
		}
		if err := repo.Add(ctx, 2, 3); err != nil {
			t.Fatal(err)
		}
		vs, err := tr.Versions(ctx, 3)
		if err != nil || len(vs) != 3 || !vs[0].Original || vs[0].Node.ID != 1 || vs[1].Node.Langcode != "ru" || vs[2].Node.URL() != "/article/meer" {
			t.Fatalf("the set: %+v %v", vs, err)
		}
		if err := repo.Saved(ctx, 1); err != nil {
			t.Fatal(err)
		}
		if err := repo.Saved(ctx, 3); err != nil {
			t.Fatal(err)
		}
		vs, _ = tr.Versions(ctx, 1)
		if vs[0].Outdated || !vs[1].Outdated || vs[2].Outdated {
			t.Errorf("outdated marks: %+v", vs)
		}
		if _, err := db.DB.Exec(`DELETE FROM nodes WHERE id = 1`); err != nil {
			t.Fatal(err)
		}
		if vs, err := tr.Versions(ctx, 2); err != nil || len(vs) != 2 || vs[0].Original {
			t.Errorf("without the original: %+v %v", vs, err)
		}
		if st, err := tr.Settings(ctx); err != nil || st.Untranslated != UntranslatedOriginal || len(st.TranslatableTypes) != 0 {
			t.Errorf("the default settings: %+v %v", st, err)
		}
		if err := tr.SaveSettings(ctx, Settings{TranslatableTypes: []string{"article"}, Untranslated: UntranslatedHide}); err != nil {
			t.Fatal(err)
		}
		fresh := NewTranslations(repo, nil)
		if ok, err := fresh.Translatable(ctx, "article"); err != nil || !ok {
			t.Errorf("translatable types: %v %v", ok, err)
		}
		if st, _ := fresh.Settings(ctx); st.Untranslated != UntranslatedHide {
			t.Errorf("untranslated: %q", st.Untranslated)
		}
	})
}
