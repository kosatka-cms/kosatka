package languages

import (
	"context"
	"log/slog"

	entsql "entgo.io/ent/dialect/sql"

	"github.com/kosatka-cms/kosatka/modulekit"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// listFilter - the content lists' narrowing to the request's language
// (modulekit.SetContentListFilter) on a site of several, by the setting
// for the items without a version in it (Settings.Untranslated).
func listFilter(translations *Translations, logger *slog.Logger) func(ctx context.Context) func(*entsql.Selector) {
	return func(ctx context.Context) func(*entsql.Selector) {
		def, others := modulekit.SiteLanguages()
		if len(others) == 0 {
			return nil
		}
		st, err := translations.Settings(ctx)
		if err != nil {
			modulekit.LogFallback(logger, err, "error loading the languages settings for a list")
			return nil
		}
		if st.Untranslated == UntranslatedAll {
			return nil
		}
		return languageCondition(i18n.FromContext(ctx), def, st.Untranslated == UntranslatedHide)
	}
}

// languageCondition - the nodes of lang (and, for the default language,
// those without one); unless only, also the items of other languages
// with no public version in lang - an original or an untranslated item,
// never another language's translation.
func languageCondition(lang, def string, only bool) func(*entsql.Selector) {
	return func(s *entsql.Selector) {
		own := entsql.EQ(s.C("langcode"), lang)
		if lang == def {
			own = entsql.Or(own, entsql.EQ(s.C("langcode"), ""))
		}
		if only {
			s.Where(own)
			return
		}
		// The translations (not originals).
		t1 := entsql.Table("node_translations").As("lt1")
		translations := entsql.Select(t1.C("nid")).From(t1).Where(entsql.ColumnsNEQ(t1.C("nid"), t1.C("tnid")))
		// The items of the sets with a public version in lang.
		t2 := entsql.Table("node_translations").As("lt2")
		n2 := entsql.Table("nodes").As("ln2")
		setsInLang := entsql.Select(t2.C("tnid")).From(t2).Join(n2).On(n2.C("id"), t2.C("nid")).
			Where(entsql.And(entsql.EQ(n2.C("langcode"), lang), entsql.EQ(n2.C("status"), 1), entsql.EQ(n2.C("on_moderation"), 0), entsql.IsNull(n2.C("deleted_at"))))
		t3 := entsql.Table("node_translations").As("lt3")
		inSets := entsql.Select(t3.C("nid")).From(t3).Where(entsql.In(t3.C("tnid"), setsInLang))
		s.Where(entsql.Or(own, entsql.And(entsql.NotIn(s.C("id"), translations), entsql.NotIn(s.C("id"), inSets))))
	}
}
