//go:build dbintegration

package videoembed

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/kosatka-cms/kosatka/modulekit/testkit"
)

// TestRepo - the settings are the defaults until saved, then the saved
// ones; a video's answer is stored by its link and replaced by a later
// one; links not asked about are absent.
func TestRepo(t *testing.T) {
	testkit.SchemaDB(t, func(t *testing.T, db testkit.DB) {
		ctx := context.Background()
		repo := NewRepo(db.DB)
		if st, err := repo.Settings(ctx); err != nil || !st.ClickToPlay || !slices.Equal(st.Providers, defaultProviders()) {
			t.Fatalf("defaults: %+v %v", st, err)
		}
		for _, want := range []Settings{{Providers: []string{"vk"}, LocalPreviews: true}, {Providers: []string{"youtube", "rutube"}, ClickToPlay: true}} {
			if err := repo.SaveSettings(ctx, want); err != nil {
				t.Fatal(err)
			}
			if st, err := repo.Settings(ctx); err != nil || !slices.Equal(st.Providers, want.Providers) || st.ClickToPlay != want.ClickToPlay || st.LocalPreviews != want.LocalPreviews {
				t.Errorf("saved %+v, read %+v %v", want, st, err)
			}
		}

		at := time.Now().UTC().Truncate(time.Second)
		if err := repo.SaveEmbed(ctx, Embed{URL: testVideo, Provider: "youtube", Failed: true, FetchedAt: at}); err != nil {
			t.Fatal(err)
		}
		if err := repo.SaveEmbed(ctx, Embed{URL: testVideo, Provider: "youtube", Title: "Trailer", ThumbnailURL: "https://i.ytimg.com/x.jpg", ThumbnailFile: "video_embed/ab/ab.jpg", Width: 16, Height: 9, FetchedAt: at}); err != nil {
			t.Fatal(err)
		}
		got, err := repo.Embeds(ctx, []string{testVideo, testShorts})
		if err != nil || len(got) != 1 {
			t.Fatalf("embeds: %+v %v", got, err)
		}
		if e := got[testVideo]; e.Failed || e.Title != "Trailer" || e.ThumbnailFile != "video_embed/ab/ab.jpg" || e.Width != 16 || e.Height != 9 || !e.FetchedAt.Equal(at) {
			t.Errorf("embed: %+v", e)
		}
	})
}
