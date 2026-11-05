//go:build dbintegration

package database_test

import (
	"context"
	"slices"
	"testing"
	"time"

	entsql "entgo.io/ent/dialect/sql"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/database"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/blockplacementdef"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/migrations"
	galleriesent "github.com/kosatka-cms/kosatka/internal/modules/galleries/ent/generated"
	"github.com/kosatka-cms/kosatka/internal/testdb"
)

// TestModuleUninstallRepository - the core side of an uninstall: node IDs
// of the module's types, the files its tables reference, its types'
// settings (a block shown only on them is deleted, one shown elsewhere too
// keeps the rest) and a vocabulary deleted only when nothing uses it; the
// the galleries' switches of nodes and types go with its tables.
func TestModuleUninstallRepository(t *testing.T) {
	schemaDB(t, func(t *testing.T, db testdb.DB) {
		ctx := context.Background()
		client := generated.NewClient(generated.Driver(entsql.OpenDB(string(db.Dialect), db.DB)))
		repo := database.NewModuleUninstallRepository(db.DB)
		now := time.Now()

		poll, err := client.Node.Create().SetType("poll").SetTitle("Poll").Save(ctx)
		if err != nil {
			t.Fatal(err)
		}
		article, err := client.Node.Create().SetType("article").SetTitle("Article").Save(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if ids, err := repo.NodeIDsOfTypes(ctx, []string{"poll", "webform"}, 10); err != nil || !slices.Equal(ids, []int{poll.ID}) {
			t.Errorf("node ids: %v %v", ids, err)
		}

		file, err := client.File.Create().SetURI("public://g.jpg").SetMimeType("image/jpeg").Save(ctx)
		if err != nil {
			t.Fatal(err)
		}
		// The galleries module's rows, with its own client (its own schema).
		galleries := galleriesent.NewClient(galleriesent.Driver(entsql.OpenDB(string(db.Dialect), db.DB)))
		gal, err := galleries.Gallery.Create().SetNid(article.ID).SetTitle("G").Save(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := galleries.GalleryItem.Create().SetGalleryID(gal.ID).SetFid(file.ID).Save(ctx); err != nil {
			t.Fatal(err)
		}
		var cols [][2]string
		for _, c := range migrations.FileColumns("galleries") {
			cols = append(cols, [2]string{c.Table, c.Column})
		}
		if ids, err := repo.FileIDs(ctx, cols); err != nil || !slices.Equal(ids, []int{file.ID}) {
			t.Errorf("file ids %v (columns %v): %v", ids, cols, err)
		}

		if _, err := client.SearchType.Create().SetID("poll").Save(ctx); err != nil {
			t.Fatal(err)
		}
		if _, err := client.SeoRule.Create().SetContentType("poll").SetCreatedAt(now).SetUpdatedAt(now).Save(ctx); err != nil {
			t.Fatal(err)
		}
		place := func(types string) int {
			p, err := client.BlockPlacementDef.Create().SetPlacementType("plugin").SetPluginID("x").SetTheme("default").SetRegion("sidebar").
				SetVisibilityScope(string(entity.BlockVisibilityContentType)).SetVisibilityContentType(types).SetCreatedAt(now).SetUpdatedAt(now).Save(ctx)
			if err != nil {
				t.Fatal(err)
			}
			return p.ID
		}
		only, mixed := place("poll"), place("article,poll")
		if err := repo.DeleteNodeTypeSettings(ctx, []string{"poll"}); err != nil {
			t.Fatal(err)
		}
		if n, _ := client.SearchType.Query().Count(ctx); n != 0 {
			t.Errorf("search types left: %d", n)
		}
		if n, _ := client.SeoRule.Query().Count(ctx); n != 0 {
			t.Errorf("seo rules left: %d", n)
		}
		if n, _ := client.BlockPlacementDef.Query().Where(blockplacementdef.ID(only)).Count(ctx); n != 0 {
			t.Error("a block shown only on the type must go")
		}
		if p, err := client.BlockPlacementDef.Get(ctx, mixed); err != nil || p.VisibilityContentType == nil || *p.VisibilityContentType != "article" {
			t.Errorf("mixed block: %+v %v", p, err)
		}

		used, err := client.TaxonomyVocabulary.Create().SetName("Used").SetSlug("used").SetCreatedAt(now).SetUpdatedAt(now).Save(ctx)
		if err != nil {
			t.Fatal(err)
		}
		free, err := client.TaxonomyVocabulary.Create().SetName("Free").SetSlug("free").SetCreatedAt(now).SetUpdatedAt(now).Save(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := client.TaxonomyTermData.Create().SetName("Section").SetSlug("section").SetVid(free.ID).SetCreatedAt(now).SetUpdatedAt(now).Save(ctx); err != nil {
			t.Fatal(err)
		}
		if _, err := client.FieldStorageDef.Create().SetMachineName("field_tags").SetFieldType(string(entity.FieldTypeTaxonomy)).
			SetConfig(map[string]any{"vocabulary_id": used.ID}).SetCreatedAt(now).SetUpdatedAt(now).Save(ctx); err != nil {
			t.Fatal(err)
		}
		if deleted, err := repo.DeleteVocabularyIfUnused(ctx, used.ID); err != nil || deleted {
			t.Errorf("used vocabulary: %v %v", deleted, err)
		}
		if deleted, err := repo.DeleteVocabularyIfUnused(ctx, free.ID); err != nil || !deleted {
			t.Errorf("free vocabulary: %v %v", deleted, err)
		}
		if n, _ := client.TaxonomyTermData.Query().Count(ctx); n != 0 {
			t.Errorf("terms left: %d", n)
		}

		r := migrations.Runner{DB: db.DB, Dialect: db.Dialect}
		if err := r.Uninstall(ctx, "galleries"); err != nil {
			t.Fatal(err)
		}
		if n, _ := client.Node.Query().Where().Count(ctx); n != 2 {
			t.Fatalf("nodes: %d", n)
		}
		// The nodes' galleries switches went with the module's tables.
		if err := galleries.GalleryNode.Create().SetNid(poll.ID).Exec(ctx); err == nil {
			t.Error("gallery_nodes is still there")
		}
		if ids, err := repo.FileIDs(ctx, cols); err != nil || len(ids) != 0 {
			t.Errorf("files of an uninstalled module: %v %v", ids, err)
		}
	})
}

// TestClearFieldFormatters - a link field's formatter and layout are kept
// on the detail page and in a View field; an uninstall clears the
// module's formatter in both (the layout and another formatter staying).
func TestClearFieldFormatters(t *testing.T) {
	schemaDB(t, func(t *testing.T, db testdb.DB) {
		ctx := context.Background()
		seedUser(t, db)
		client := entClient(db)
		types := database.NewContentTypeRepository(client)
		views := database.NewViewRepository(client)
		now := time.Now()
		ct, err := client.ContentTypeDef.Create().SetMachineName("film").SetLabel("Film").
			SetStatus(1).SetCreatedBy(1).SetCreatedAt(now).SetUpdatedAt(now).Save(ctx)
		if err != nil {
			t.Fatal(err)
		}
		st, err := client.FieldStorageDef.Create().SetMachineName("field_trailer").SetFieldType(string(entity.FieldTypeLink)).
			SetConfig(map[string]any{}).SetCreatedAt(now).SetUpdatedAt(now).Save(ctx)
		if err != nil {
			t.Fatal(err)
		}
		in, err := client.FieldInstanceDef.Create().SetContentTypeID(ct.ID).SetFieldStorageID(st.ID).SetLabel("Trailer").
			SetCreatedAt(now).SetUpdatedAt(now).Save(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if err := types.UpdateFieldDetailDisplay(ctx, in.ID, entity.FieldDetailDisplay{ShowOnDetail: true, Formatter: "video_player", Layout: entity.FieldLayoutFeatured}); err != nil {
			t.Fatal(err)
		}
		display, err := views.CreateView(ctx, entity.ViewDisplayDef{Label: "Trailers", MachineName: "trailers", DisplayType: entity.ViewDisplayTypePage, URLPath: "trailers", BaseEntity: entity.ViewBaseEntityContent})
		if err != nil {
			t.Fatal(err)
		}
		storageID := st.ID
		vf, err := views.CreateViewField(ctx, entity.ViewFieldDef{ViewDisplayID: display.ID, FieldStorageID: &storageID, Formatter: "video_preview", Layout: entity.FieldLayoutFirst})
		if err != nil || vf.Formatter != "video_preview" || vf.Layout != entity.FieldLayoutFirst {
			t.Fatalf("view field %+v %v", vf, err)
		}
		other, err := views.CreateViewField(ctx, entity.ViewFieldDef{ViewDisplayID: display.ID, FieldStorageID: &storageID, Formatter: "other_module"})
		if err != nil {
			t.Fatal(err)
		}
		fields, err := types.FieldsByContentTypeID(ctx, ct.ID)
		if err != nil || fields[0].DetailFormatter != "video_player" || fields[0].DetailLayout != entity.FieldLayoutFeatured {
			t.Fatalf("detail display %+v %v", fields, err)
		}

		if err := database.NewModuleUninstallRepository(db.DB).ClearFieldFormatters(ctx, []string{"video_player", "video_preview"}); err != nil {
			t.Fatal(err)
		}
		fields, _ = types.FieldsByContentTypeID(ctx, ct.ID)
		if fields[0].DetailFormatter != "" || fields[0].DetailLayout != entity.FieldLayoutFeatured {
			t.Errorf("detail display after: %q %q", fields[0].DetailFormatter, fields[0].DetailLayout)
		}
		if got, _ := views.ViewFieldByID(ctx, vf.ID); got.Formatter != "" || got.Layout != entity.FieldLayoutFirst {
			t.Errorf("view field after: %+v", got)
		}
		if got, _ := views.ViewFieldByID(ctx, other.ID); got.Formatter != "other_module" {
			t.Errorf("another module's formatter cleared: %+v", got)
		}
	})
}
