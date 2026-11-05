//go:build dbintegration

package database_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/database"
	"github.com/kosatka-cms/kosatka/internal/testdb"
)

// TestSiteInformationKept - the site information is read once and kept
// until the repository writes it: a change behind its back isn't seen
// until then; a caller's change to what it got stays out.
func TestSiteInformationKept(t *testing.T) {
	schemaDB(t, func(t *testing.T, db testdb.DB) {
		ctx := context.Background()
		seedRow(t, db, `UPDATE site_information SET site_name = 'First' WHERE id = 1`)
		repo := database.NewSiteInformationRepository(db.DB)
		site, err := repo.Get(ctx)
		if err != nil || site.SiteName != "First" {
			t.Fatalf("get: %+v %v", site, err)
		}
		site.SiteName = "Changed by the caller"
		seedRow(t, db, `UPDATE site_information SET site_name = 'Second' WHERE id = 1`)
		if again, _ := repo.Get(ctx); again.SiteName != "First" {
			t.Errorf("kept: %q, want First", again.SiteName)
		}
		if err := repo.SetTheme(ctx, "kosatka"); err != nil {
			t.Fatal(err)
		}
		if after, _ := repo.Get(ctx); after.SiteName != "Second" || after.Theme == nil || *after.Theme != "kosatka" {
			t.Errorf("after a write: %+v", after)
		}
	})
}

// TestSiteSchemaOrg - who runs the site is stored, read back whole, kept
// by a save of the rest of the information, and a caller's change to
// its profiles stays out of the kept copy.
func TestSiteSchemaOrg(t *testing.T) {
	schemaDB(t, func(t *testing.T, db testdb.DB) {
		ctx := context.Background()
		repo := database.NewSiteInformationRepository(db.DB)
		o := entity.SiteSchemaOrg{Kind: entity.SchemaOrgLocalBusiness, Name: "Bakery", SameAs: []string{"https://t.me/bakery"},
			Phone: "+1 555", Email: "hello@example.com", Address: "Main st. 1", SearchBox: true}
		if err := repo.SetSchemaOrg(ctx, o); err != nil {
			t.Fatal(err)
		}
		site, err := repo.Get(ctx)
		if err != nil || !reflect.DeepEqual(site.SchemaOrg, o) {
			t.Fatalf("read back: %+v %v", site.SchemaOrg, err)
		}
		site.SchemaOrg.SameAs[0] = "changed"
		site.SiteName = "Renamed"
		if err := repo.Update(ctx, site); err != nil {
			t.Fatal(err)
		}
		if after, _ := repo.Get(ctx); after.SchemaOrg.SameAs[0] != "https://t.me/bakery" || after.SchemaOrg.Name != "Bakery" {
			t.Errorf("after saving the rest: %+v", after.SchemaOrg)
		}
	})
}
