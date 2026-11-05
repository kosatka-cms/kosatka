//go:build dbintegration

package galleries

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	entsql "entgo.io/ent/dialect/sql"

	"github.com/kosatka-cms/kosatka/internal/modules/galleries/ent/generated"
	"github.com/kosatka-cms/kosatka/modulekit/testkit"
)

// TestGalleryLifecycle - a node's gallery through the service and the
// repository: created with its preview styles (the defaults, or chosen;
// a style the site lacks refused), listed and counted
// by filter (the node's page: published only), its items added, reordered, given a cover and removed, the
// gallery updated, then deleted with its items; the node's galleries go
// all at once.
func TestGalleryLifecycle(t *testing.T) {
	testkit.SchemaDB(t, func(t *testing.T, db testkit.DB) {
		ctx := context.Background()
		d := testkit.Deps(db) // the core's, as Wire gets them on a site
		own := generated.NewClient(generated.Driver(entsql.OpenDB(string(db.Dialect), db.DB)))
		svc := NewGalleryService(NewGalleryRepository(d.DB, d.NodeRepo, d.Files, d.ImageStyles), d.ImageStyles, d.Logger)
		nodeID := insertRow(t, db, "INSERT INTO nodes (type, title, status) VALUES (?, ?, ?)", "place", "Harbour", 1)
		now := time.Now()
		var files []int
		for _, uri := range []string{"public://a.jpg", "public://b.jpg", "public://c.jpg"} {
			files = append(files, insertRow(t, db, "INSERT INTO files (uri, created_at, updated_at) VALUES (?, ?, ?)", uri, now, now))
		}

		g, err := svc.Create(ctx, GalleryCreateInput{NodeID: nodeID, Title: new("Covers"), Status: true})
		if err != nil {
			t.Fatal(err)
		}
		// The defaults, with the grid style's size.
		if g.PreviewSmallStyle != "gallery_thumbnail" || g.PreviewMediumStyle != "gallery_medium" || g.PreviewWidth != 215 || g.PreviewHeight != 144 {
			t.Errorf("the preview styles: %+v", g)
		}
		if _, err := svc.Create(ctx, GalleryCreateInput{NodeID: nodeID, PreviewSmallStyle: "no_such_style"}); !errors.Is(err, ErrGalleryStyle) {
			t.Errorf("an unknown style: %v", err)
		}
		hidden, err := svc.Create(ctx, GalleryCreateInput{NodeID: nodeID, PreviewSmallStyle: "gallery_medium", Weight: 5})
		if err != nil {
			t.Fatal(err)
		}

		published := true
		if list, err := svc.Find(ctx, GalleriesFilter{NodeID: nodeID, Status: &published}); err != nil || len(list) != 1 || list[0].ID != g.ID {
			t.Errorf("published galleries: %v %v", list, err)
		}
		if n, err := svc.TotalCount(ctx, GalleriesFilter{NodeID: nodeID}); err != nil || n != 2 {
			t.Errorf("count: %d %v", n, err)
		}
		// A search counts what it finds: the title, or the nodes given.
		for _, f := range []GalleriesFilter{{Query: "cove"}, {Query: "harbour", NodeIDs: []int{nodeID}}, {Query: "nothing"}} {
			list, err := svc.Find(ctx, f)
			n, cerr := svc.TotalCount(ctx, f)
			if err != nil || cerr != nil || n != len(list) {
				t.Errorf("search %+v: found %d, counted %d (%v %v)", f, len(list), n, err, cerr)
			}
		}
		// A node's page lists its published galleries only.
		if list, err := svc.ByNodeID(ctx, nodeID); err != nil || len(list) != 1 || list[0].ID != g.ID {
			t.Errorf("by node: %v %v", list, err)
		}
		if list, err := svc.SummaryByNodeID(ctx, nodeID); err != nil || len(list) != 1 || list[0].ID != g.ID {
			t.Errorf("summary by node: %v %v", list, err)
		}

		var items []*GalleryItem
		for i, fid := range files {
			it, err := svc.AddItem(ctx, GalleryItemCreateInput{GalleryID: g.ID, FileID: fid, Weight: i})
			if err != nil {
				t.Fatal(err)
			}
			items = append(items, it)
		}
		if cover, err := svc.GetCover(ctx, g.ID); err != nil || cover.ID != items[0].ID {
			t.Errorf("no cover marked: the first item, got %v %v", cover, err)
		}
		if err := svc.SetCover(ctx, g.ID, items[2].ID); err != nil {
			t.Fatal(err)
		}
		if err := svc.SetCover(ctx, g.ID, items[1].ID); err != nil { // moves, not adds
			t.Fatal(err)
		}
		if cover, err := svc.GetCover(ctx, g.ID); err != nil || cover.ID != items[1].ID {
			t.Errorf("cover: %v %v", cover, err)
		}
		if n, _ := own.GalleryItem.Query().Count(ctx); n != 3 {
			t.Errorf("items: %d", n)
		}

		if err := svc.ReorderItems(ctx, []GalleryItemReorder{{ID: items[2].ID, Weight: -1}}); err == nil {
			t.Error("a negative weight accepted")
		}
		if err := svc.ReorderItems(ctx, []GalleryItemReorder{{ID: items[0].ID, Weight: 9}, {ID: items[2].ID, Weight: 0}}); err != nil {
			t.Fatal(err)
		}
		got, err := svc.GetItems(ctx, g.ID)
		if err != nil || len(got) != 3 || got[0].ID != items[2].ID || got[2].ID != items[0].ID {
			t.Errorf("reordered items: %v %v", got, err)
		}
		if _, err := svc.UpdateItem(ctx, items[0].ID, GalleryItemUpdateInput{Title: new("First")}); err != nil {
			t.Fatal(err)
		}
		// The previews use the gallery type's image styles.
		if it, err := svc.GetItemByID(ctx, items[0].ID); err != nil || it.GetTitle() != "First" || it.FileID != files[0] || it.File == nil ||
			it.PreviewSmall == nil || !strings.Contains(it.PreviewSmall.URI, "gallery_thumbnail") {
			t.Errorf("updated item: %+v %v", it, err)
		}
		if err := svc.RemoveItem(ctx, items[0].ID); err != nil {
			t.Fatal(err)
		}

		if _, err := svc.Update(ctx, g.ID, GalleryUpdateInput{Title: new("Renamed"), Status: new(false)}); err != nil {
			t.Fatal(err)
		}
		if read, err := svc.ByID(ctx, g.ID); err != nil || read.GetTitle() != "Renamed" || read.Status || len(read.Items) != 2 {
			t.Errorf("updated gallery: %+v %v", read, err)
		}

		if err := svc.DeleteWithItems(ctx, hidden.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := svc.ByID(ctx, hidden.ID); err == nil {
			t.Error("the deleted gallery is still there")
		}
		if err := svc.DeleteByNodeIDWithItems(ctx, nodeID); err != nil {
			t.Fatal(err)
		}
		if n, _ := own.Gallery.Query().Count(ctx); n != 0 {
			t.Errorf("galleries left: %d", n)
		}
		if n, _ := own.GalleryItem.Query().Count(ctx); n != 0 {
			t.Errorf("items left: %d", n)
		}
	})
}
