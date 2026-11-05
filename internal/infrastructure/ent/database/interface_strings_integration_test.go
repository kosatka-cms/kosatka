//go:build dbintegration

package database_test

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/database"
	"github.com/kosatka-cms/kosatka/internal/testdb"
)

// TestInterfaceStringRepository - the site's translations of strings: a
// text, plural forms, a context; saved again - rewritten in place (to
// the same value too, and from forms to a text), one per language, a
// long source with HTML kept whole; deleted - only that language's.
func TestInterfaceStringRepository(t *testing.T) {
	schemaDB(t, func(t *testing.T, db testdb.DB) {
		ctx := context.Background()
		repo := database.NewInterfaceStringRepository(db.DB)
		at := time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC)
		long := `<a href=":login">Log in</a> ` + strings.Repeat("and more ", 60)
		for _, s := range []entity.InterfaceString{
			{Lang: "ru", Source: "Comments", Text: "Отзывы"},
			{Lang: "ru", Source: "@count comment", Forms: []string{"@count отзыв", "@count отзыва", "@count отзывов"}},
			{Lang: "ru", Context: "menu", Source: "Open", Text: "Раскрыть"},
			{Lang: "en", Source: "Comments", Text: "Reviews"},
			{Lang: "ru", Source: long, Text: "Войдите"},
		} {
			s.UpdatedAt = at
			if err := repo.Save(ctx, s); err != nil {
				t.Fatal(err)
			}
		}
		// The same value again, then a plural string turned into a text.
		for _, s := range []entity.InterfaceString{
			{Lang: "ru", Source: "Comments", Text: "Отзывы", UpdatedAt: at},
			{Lang: "ru", Source: "@count comment", Text: "отзывы", UpdatedAt: at.Add(time.Hour)},
		} {
			if err := repo.Save(ctx, s); err != nil {
				t.Fatal(err)
			}
		}
		all := func() map[string]entity.InterfaceString {
			t.Helper()
			rows, err := repo.All(ctx)
			if err != nil {
				t.Fatal(err)
			}
			out := map[string]entity.InterfaceString{}
			for _, r := range rows {
				out[r.Lang+"|"+r.Context+"|"+r.Source] = r
			}
			if len(out) != len(rows) {
				t.Errorf("a string saved twice: %+v", rows)
			}
			return out
		}
		got := all()
		if len(got) != 5 {
			t.Fatalf("got %d strings, want 5: %+v", len(got), got)
		}
		if g := got["ru||Comments"]; g.Text != "Отзывы" || len(g.Forms) != 0 {
			t.Errorf("ru Comments = %+v", g)
		}
		if g := got["en||Comments"]; g.Text != "Reviews" {
			t.Errorf("en Comments = %+v", g)
		}
		if g := got["ru|menu|Open"]; g.Text != "Раскрыть" {
			t.Errorf("ru menu Open = %+v", g)
		}
		if g := got["ru||@count comment"]; g.Text != "отзывы" || len(g.Forms) != 0 || !g.UpdatedAt.Equal(at.Add(time.Hour)) {
			t.Errorf("ru @count comment = %+v", g)
		}
		if g, ok := got["ru||"+long]; !ok || g.Text != "Войдите" {
			t.Errorf("the long source = %+v (found %v)", g, ok)
		}

		if err := repo.Save(ctx, entity.InterfaceString{Lang: "ru", Source: "@count comment",
			Forms: []string{"@count отзыв", "@count отзыва", "@count отзывов"}, UpdatedAt: at}); err != nil {
			t.Fatal(err)
		}
		if g := all()["ru||@count comment"]; g.Text != "" || !slices.Equal(g.Forms, []string{"@count отзыв", "@count отзыва", "@count отзывов"}) {
			t.Errorf("forms again = %+v", g)
		}

		if err := repo.Delete(ctx, "ru", "", "Comments"); err != nil {
			t.Fatal(err)
		}
		if err := repo.Delete(ctx, "ru", "", "Not there"); err != nil {
			t.Fatal(err)
		}
		got = all()
		if _, ok := got["ru||Comments"]; ok {
			t.Error("ru Comments not deleted")
		}
		if _, ok := got["en||Comments"]; !ok {
			t.Error("en Comments deleted with ru")
		}
	})
}
