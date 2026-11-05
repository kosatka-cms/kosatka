//go:build dbintegration

package database_test

import (
	"context"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/database"
	"github.com/kosatka-cms/kosatka/internal/testdb"
)

// TestCountNodesByTermID - a term is on a node whether the node is
// published or not, through the index or a taxonomy field's values (one
// or many): what keeps it from being deleted; a node counts once.
func TestCountNodesByTermID(t *testing.T) {
	schemaDB(t, func(t *testing.T, db testdb.DB) {
		ctx := context.Background()
		seedUser(t, db)
		seedNode(t, db, 1, "article", "Published", "a")
		seedNode(t, db, 2, "article", "Draft", "b")
		seedNode(t, db, 3, "article", "Other", "c")
		seedRow(t, db, `UPDATE nodes SET status = 0 WHERE id = 2`)
		seedRow(t, db, `INSERT INTO taxonomy_vocabulary (id, name, slug) VALUES (1, 'Tags', 'tags')`)
		seedRow(t, db, `INSERT INTO taxonomy_term_data (id, vid, name, slug, weight) VALUES (1, 1, 'Used', 'used', 0), (2, 1, 'Free', 'free', 0)`)
		seedRow(t, db, `INSERT INTO field_storages (id, machine_name, field_type, created_at, updated_at) VALUES
			(7, 'topic', 'taxonomy', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP), (8, 'tags', 'taxonomy', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`)
		seedRow(t, db, `INSERT INTO taxonomy_index (nid, tid) VALUES (1, 1)`)
		seedRow(t, db, `INSERT INTO field_value_taxonomy (node_id, field_id, term_id) VALUES (1, 7, 1), (2, 7, 1)`)
		seedRow(t, db, `INSERT INTO field_value_taxonomy_multi (node_id, field_id, term_id) VALUES (3, 8, 1)`)

		repo := database.NewTaxonomyRepository(entClient(db))
		if n, err := repo.CountNodesByTermID(ctx, 1); err != nil || n != 3 {
			t.Errorf("the used term: %d %v, want 3 (an unpublished node and a multiple field too)", n, err)
		}
		if n, err := repo.CountNodesByTermID(ctx, 2); err != nil || n != 0 {
			t.Errorf("the free term: %d %v", n, err)
		}
	})
}

// TestTermNodeCounts - the published materials (not on moderation) on
// each term, of one vocabulary or of all; unused terms are left out.
func TestTermNodeCounts(t *testing.T) {
	schemaDB(t, func(t *testing.T, db testdb.DB) {
		ctx := context.Background()
		seedUser(t, db)
		for i, slug := range []string{"a", "b", "c", "d"} {
			seedNode(t, db, i+1, "article", "Node", slug)
		}
		seedRow(t, db, `UPDATE nodes SET status = 0 WHERE id = 3`)
		seedRow(t, db, `UPDATE nodes SET on_moderation = 1 WHERE id = 4`)
		seedRow(t, db, `INSERT INTO taxonomy_vocabulary (id, name, slug) VALUES (1, 'Tags', 'tags'), (2, 'Kinds', 'kinds')`)
		seedRow(t, db, `INSERT INTO taxonomy_term_data (id, vid, name, slug, weight) VALUES (1, 1, 'Go', 'tags/go', 0), (2, 1, 'Free', 'tags/free', 0), (3, 2, 'Guide', 'kinds/guide', 0)`)
		seedRow(t, db, `INSERT INTO taxonomy_index (nid, tid) VALUES (1, 1), (2, 1), (3, 1), (4, 1), (1, 3), (3, 2)`)

		repo := database.NewTaxonomyRepository(entClient(db))
		all, err := repo.TermNodeCounts(ctx, 0)
		if err != nil {
			t.Fatal(err)
		}
		if len(all) != 2 || all[1] != 2 || all[3] != 1 {
			t.Errorf("every vocabulary: %v, want map[1:2 3:1]", all)
		}
		tags, err := repo.TermNodeCounts(ctx, 1)
		if err != nil {
			t.Fatal(err)
		}
		if len(tags) != 1 || tags[1] != 2 {
			t.Errorf("the tags: %v, want map[1:2]", tags)
		}
	})
}

// TestTermsCarryPublicTermPages - terms read for content pages and
// lists come with their vocabulary's "public term pages" flag, so
// templates link only terms that have a page
// (entity.TaxonomyTerm.HasPublicPage) without a query per term; a term
// read by ID carries its parent, which the service's parent checks walk.
func TestTermsCarryPublicTermPages(t *testing.T) {
	schemaDB(t, func(t *testing.T, db testdb.DB) {
		ctx := context.Background()
		seedUser(t, db)
		seedNode(t, db, 1, "article", "Node", "a")
		seedRow(t, db, `INSERT INTO taxonomy_vocabulary (id, name, slug, public_term_pages) VALUES (1, 'Tags', 'tags', TRUE), (2, 'Kinds', 'kinds', FALSE)`)
		seedRow(t, db, `INSERT INTO taxonomy_term_data (id, vid, name, slug, weight) VALUES (1, 1, 'Go', 'tags/go', 0), (2, 2, 'Guide', 'kinds/guide', 0)`)
		seedRow(t, db, `INSERT INTO taxonomy_term_data (id, vid, name, slug, weight, parent_id) VALUES (3, 1, 'Generics', 'tags/generics', 0, 1)`)
		seedRow(t, db, `INSERT INTO taxonomy_index (nid, tid) VALUES (1, 1), (1, 2)`)

		repo := database.NewTaxonomyRepository(entClient(db))
		byID, err := repo.TermsByIDs(ctx, []int{1, 2})
		if err != nil {
			t.Fatal(err)
		}
		if !byID[1].HasPublicPage() || byID[2].HasPublicPage() {
			t.Errorf("TermsByIDs: public pages %v/%v, want true/false", byID[1].HasPublicPage(), byID[2].HasPublicPage())
		}
		byNode, err := repo.NodeTermsByIDs(ctx, []int{1})
		if err != nil {
			t.Fatal(err)
		}
		if ts := byNode[1]; len(ts) != 2 || !ts[0].HasPublicPage() || ts[1].HasPublicPage() {
			t.Errorf("NodeTermsByIDs: %+v", ts)
		}
		child, err := repo.TermByID(ctx, 3)
		if err != nil || child.ParentID != 1 || child.Vocabulary.ID != 1 {
			t.Errorf("TermByID(3) = %+v, %v; want parent 1, vocabulary 1", child, err)
		}
	})
}
