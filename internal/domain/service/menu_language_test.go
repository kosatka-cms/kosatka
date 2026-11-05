package service

import (
	"context"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

// TestLocalizeMenuTree - on an English page of a Russian site: a link's
// title in English, a link shown in Russian only left out (its children
// too), a link to content leading to its English version, another of the
// site's addresses under /en/, another site's as is; on a site of one
// language, the tree as is.
func TestLocalizeMenuTree(t *testing.T) {
	entity.SetLanguagesSwitch(func() bool { return true })
	entity.SetSiteLanguages("ru", []entity.SiteLanguage{{Code: "en", Prefix: "en"}})
	entity.SetTextTranslator(func(lang, kind, ref string) (string, bool) {
		return "News", lang == "en" && kind == entity.TextMenuLinkTitle && ref == "1"
	})
	entity.SetLanguageVisibility(func(kind, ref, lang string) bool { return ref != "3" || lang == "ru" })
	entity.SetNodeURLsInLanguage(func(_ context.Context, ids []int, lang string) map[int]string {
		return map[int]string{7: "/en/about-us"}
	})
	t.Cleanup(func() {
		entity.SetSiteLanguages("", nil)
		entity.SetLanguagesSwitch(nil)
		entity.SetTextTranslator(nil)
		entity.SetLanguageVisibility(nil)
		entity.SetNodeURLsInLanguage(nil)
	})
	tree := []MenuTreeItem{
		{ID: 1, Title: "Новости", URL: "/novosti", HasChildren: true, Children: []MenuTreeItem{{ID: 4, Title: "Архив", URL: "https://archive.example.org/"}}},
		{ID: 2, Title: "О нас", URL: "/o-nas", NodeID: 7},
		{ID: 3, Title: "Только по-русски", URL: "/ru-only", HasChildren: true, Children: []MenuTreeItem{{ID: 5, Title: "Ещё", URL: "/x"}}},
	}
	got := LocalizeMenuTree(context.Background(), tree, "en")
	if len(got) != 2 || got[0].Title != "News" || got[0].URL != "/en/novosti" || got[0].Children[0].URL != "https://archive.example.org/" ||
		got[1].URL != "/en/about-us" || !got[1].Last || got[0].Children[0].Title != "Архив" {
		t.Errorf("localized: %+v", got)
	}
	if tree[0].Title != "Новости" {
		t.Error("the shared tree changed")
	}
	entity.SetSiteLanguages("ru", nil)
	if got := LocalizeMenuTree(context.Background(), tree, "en"); len(got) != 3 || got[0].URL != "/novosti" {
		t.Errorf("a site of one language: %+v", got)
	}
}
