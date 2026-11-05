//go:build dbintegration

package database_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/database"
	"github.com/kosatka-cms/kosatka/internal/testdb"
)

// TestSearchByWords - the search finds every word of the query in any
// order, by its stem in the site's language, the last one as a beginning,
// in the title or a match field (its text, not its HTML), "ё" as "е"; the
// phrase in the title comes before all the words in it (even in a newer
// item), the title before the fields, then the newest.
func TestSearchByWords(t *testing.T) {
	schemaDB(t, func(t *testing.T, db testdb.DB) {
		ctx := context.Background()
		seedUser(t, db)
		setSiteLanguage(t, db, "ru")
		seedRow(t, db, `INSERT INTO content_types (id, machine_name, label, created_by, created_at, updated_at) VALUES (1, 'article', 'Article', 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`)
		seedRow(t, db, `INSERT INTO content_types (id, machine_name, label, created_by, created_at, updated_at) VALUES (2, 'page', 'Page', 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`)
		seedRow(t, db, `INSERT INTO field_storages (id, machine_name, field_type, created_at, updated_at) VALUES (1, 'field_body', 'text', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`)
		// The pages' text fields, all searched: a lead and a note.
		seedRow(t, db, `INSERT INTO field_storages (id, machine_name, field_type, created_at, updated_at) VALUES (2, 'field_lead', 'text', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`)
		seedRow(t, db, `INSERT INTO field_storages (id, machine_name, field_type, created_at, updated_at) VALUES (3, 'field_note', 'text', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`)
		seedRow(t, db, `INSERT INTO field_instances (label, sort_order, created_at, updated_at, content_type_id, field_storage_id) VALUES ('Note', 2, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2, 3)`)
		seedRow(t, db, `INSERT INTO field_instances (label, sort_order, created_at, updated_at, content_type_id, field_storage_id) VALUES ('Lead', 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2, 2)`)
		for id, title := range map[int]string{1: "Синий кот", 2: "Кот синий и пушистый", 3: "Про животных", 4: "Кот", 5: "Ёлка", 6: "off_line", 7: "offxline", 8: "Котлеты по-киевски", 9: "Меню"} {
			typ := "article"
			if id >= 5 {
				typ = "page"
			}
			seedNode(t, db, id, typ, title, fmt.Sprint("n", id))
			seedRow(t, db, fmt.Sprintf(`UPDATE nodes SET created_at = '2026-01-%02d 12:00:00' WHERE id = %d`, 10-id, id))
		}
		seedRow(t, db, `INSERT INTO field_value_text (value, node_id, field_id) VALUES ('<p>Синие коты спят</p>', 3, 1)`)
		seedRow(t, db, `INSERT INTO field_value_text (value, node_id, field_id) VALUES ('<p class="blue">Ничего</p>', 4, 1)`)
		seedRow(t, db, `INSERT INTO field_value_text (value, node_id, field_id) VALUES ('Борщ и вареники', 9, 2)`)
		seedRow(t, db, `INSERT INTO field_value_text (value, node_id, field_id) VALUES ('Компот из вишни, борщ по средам', 9, 3)`)
		client := entClient(db)
		if _, err := database.RebuildSearchIndex(ctx, client); err != nil {
			t.Fatal(err)
		}
		types := entity.SearchTypeDefs{
			{NodeType: "article", Enabled: true, Limit: 5, MatchFields: []string{"field_body"}},
			{NodeType: "page", Enabled: true, Limit: 5, AllFields: true},
		}
		repo := database.NewSearchRepository(client)
		for query, want := range map[string]string{
			"кот синий":   "[2 1 3]",
			"синий кот":   "[1 2 3]",
			"синих котов": "[1 2 3]",
			"КОТ":         "[1 2 4 3 8]",
			"котле":       "[8]",
			"елка":        "[5]",
			"ёлки":        "[5]",
			"off_line":    "[6]",
			"blue":        "[]",
			"сапоги":      "[]",
			"вишня":       "[9]",
		} {
			hits, err := repo.Search(ctx, query, 10, types, entity.SearchSettings{})
			if err != nil {
				t.Fatal(err)
			}
			ids := make([]int, len(hits))
			for i, h := range hits {
				ids[i] = h.ID
			}
			if got := fmt.Sprint(ids); got != want {
				t.Errorf("search %q: %s, want %s", query, got, want)
			}
		}

		// The snippets: the match field's text, the words marked by their
		// stems; none for a match in the title only or a type without
		// match fields.
		hits, err := repo.Search(ctx, "синий кот", 10, types, entity.SearchSettings{})
		if err != nil {
			t.Fatal(err)
		}
		snippets, err := repo.Snippets(ctx, "синий кот", hits, types)
		if err != nil {
			t.Fatal(err)
		}
		var marked []string
		for _, p := range snippets[3].Parts {
			if p.Match {
				marked = append(marked, p.Text)
			}
		}
		if fmt.Sprint(marked) != "[Синие коты]" || snippets[3].Before || snippets[3].After {
			t.Errorf("the snippet of 3: %+v", snippets[3])
		}
		if _, ok := snippets[1]; ok || len(snippets) != 1 {
			t.Errorf("snippets %+v, want only 3's", snippets)
		}
		// All the type's text fields, in its order: the first with the
		// words (the lead), else the next (the note).
		for query, want := range map[string]string{"борщ": "[борщ]", "компот": "[компот]"} {
			hits, err := repo.Search(ctx, query, 10, types, entity.SearchSettings{})
			if err != nil {
				t.Fatal(err)
			}
			snippets, err := repo.Snippets(ctx, query, hits, types)
			if err != nil {
				t.Fatal(err)
			}
			var marked []string
			for _, p := range snippets[9].Parts {
				if p.Match {
					marked = append(marked, strings.ToLower(p.Text))
				}
			}
			if got := fmt.Sprint(marked); got != want || len(snippets[9].Parts) == 0 {
				t.Errorf("the snippet of 9 for %q: %+v, want %s marked", query, snippets[9], want)
			}
			if query == "борщ" && snippets[9].Parts[0].Text != "Борщ" {
				t.Errorf("the snippet for %q isn't the lead's: %+v", query, snippets[9])
			}
		}
	})
}
