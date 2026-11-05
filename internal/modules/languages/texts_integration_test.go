//go:build dbintegration

package languages

import (
	"context"
	"testing"

	"github.com/kosatka-cms/kosatka/modulekit/testkit"
)

// TestTexts - a text in a language saved, read back (a fresh store too),
// emptied; a thing's languages chosen and cleared; the addresses of
// items in a language: a translation's for an item in a set, none for
// an unpublished one.
func TestTexts(t *testing.T) {
	testkit.SchemaDB(t, func(t *testing.T, db testkit.DB) {
		ctx := context.Background()
		texts := NewTexts(db.DB)
		if err := texts.SaveTranslations(ctx, "menu_link.title", "1", map[string]string{"en": " News ", "de": ""}); err != nil {
			t.Fatal(err)
		}
		if s, ok, err := NewTexts(db.DB).Text(ctx, "en", "menu_link.title", "1"); err != nil || !ok || s != "News" {
			t.Errorf("the English title: %q %v %v", s, ok, err)
		}
		if _, ok, _ := texts.Text(ctx, "de", "menu_link.title", "1"); ok {
			t.Error("an empty German title saved")
		}
		_ = texts.SaveTranslations(ctx, "menu_link.title", "1", map[string]string{"en": ""})
		if _, ok, _ := NewTexts(db.DB).Text(ctx, "en", "menu_link.title", "1"); ok {
			t.Error("the emptied English title kept")
		}
		if err := texts.SaveLanguages(ctx, "menu_link", "1", []string{"ru"}); err != nil {
			t.Fatal(err)
		}
		fresh := NewTexts(db.DB)
		if ok, _ := fresh.Visible(ctx, "menu_link", "1", "en"); ok {
			t.Error("a Russian-only link shown in English")
		}
		if ok, _ := fresh.Visible(ctx, "menu_link", "2", "en"); !ok {
			t.Error("a link without languages hidden")
		}
		_ = texts.SaveLanguages(ctx, "menu_link", "1", nil)
		if ok, _ := NewTexts(db.DB).Visible(ctx, "menu_link", "1", "en"); !ok {
			t.Error("the cleared languages kept")
		}

		for _, q := range []string{
			`INSERT INTO nodes (id, type, title, slug, status, langcode) VALUES (1, 'page', 'О нас', 'page/o-nas', 1, 'ru')`,
			`INSERT INTO nodes (id, type, title, slug, status, langcode) VALUES (2, 'page', 'About', 'page/about', 1, 'en')`,
			`INSERT INTO nodes (id, type, title, slug, status, langcode) VALUES (3, 'page', 'Kontakt', 'page/kontakt', 0, 'de')`,
		} {
			if _, err := db.DB.Exec(q); err != nil {
				t.Fatal(err)
			}
		}
		repo := NewTranslationRepo(db.DB)
		_ = repo.Add(ctx, 1, 2)
		_ = repo.Add(ctx, 1, 3)
		urls, err := nodeURLs(ctx, db.DB, []int{1, 2}, "en")
		if err != nil || len(urls) != 2 || urls[1] != "/page/about" || urls[2] != "/page/about" {
			t.Errorf("in English: %v %v", urls, err)
		}
		if urls, _ := nodeURLs(ctx, db.DB, []int{1}, "de"); len(urls) != 0 {
			t.Errorf("an unpublished German version: %v", urls)
		}
	})
}
