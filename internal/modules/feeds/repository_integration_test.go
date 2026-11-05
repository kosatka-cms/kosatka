//go:build dbintegration

package feeds

import (
	"context"
	"reflect"
	"testing"

	"github.com/kosatka-cms/kosatka/modulekit"
	"github.com/kosatka-cms/kosatka/modulekit/testkit"
)

func TestFeedRepository(t *testing.T) {
	testkit.SchemaDB(t, func(t *testing.T, db testkit.DB) {
		ctx := context.Background()
		feeds := NewFeedRepository(db.DB)
		f := Feed{Label: "News", Path: "feeds/news.xml", Format: FeedFormatRSS, Status: 1, ContentTypes: []string{"news", "review"},
			TermIDs: []int{3, 4}, ItemLimit: 20, Days: 7, ImageStyle: "large",
			FieldMaps: FeedFieldMaps{{ContentType: "news", BodyField: "body"}, {ContentType: "review", ImageField: "cover"}}}
		id, err := feeds.CreateFeed(ctx, f)
		if err != nil {
			t.Fatal(err)
		}
		got, err := feeds.FeedByPath(ctx, "feeds/news.xml")
		if err != nil || got.ID != id || !reflect.DeepEqual(got.ContentTypes, f.ContentTypes) || !reflect.DeepEqual(got.TermIDs, f.TermIDs) || !reflect.DeepEqual(got.FieldMaps, f.FieldMaps) {
			t.Errorf("feed by path: %+v %v", got, err)
		}
		f.ID, f.Label, f.FieldMaps = id, "Latest", FeedFieldMaps{{ContentType: "news", CategoryField: "tags"}}
		if err := feeds.UpdateFeed(ctx, f); err != nil {
			t.Fatal(err)
		}
		if got, err := feeds.FeedByID(ctx, id); err != nil || got.Label != "Latest" || !reflect.DeepEqual(got.FieldMaps, f.FieldMaps) {
			t.Errorf("feed by id: %+v %v", got, err)
		}
		if list, err := feeds.Feeds(ctx); err != nil || len(list) != 1 || len(list[0].FieldMaps) != 1 {
			t.Errorf("feeds: %+v %v", list, err)
		}
		if err := feeds.DeleteFeed(ctx, id); err != nil {
			t.Fatal(err)
		}
		if _, err := feeds.FeedByID(ctx, id); !modulekit.IsNotFound(err) {
			t.Errorf("deleted feed: %v", err)
		}
	})
}
