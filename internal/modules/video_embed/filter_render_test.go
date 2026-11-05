package videoembed

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/kosatka-cms/kosatka/internal/modules/video_embed/manifest"
	"github.com/kosatka-cms/kosatka/modulekit/testkit"
)

const (
	testVideo  = "https://www.youtube.com/watch?v=dQw4w9WgXcQ"
	testShorts = "https://www.youtube.com/shorts/aqz-KE-bpKQ"
)

// testFilter - a Filter whose service knows the videos (no table: what's
// known is in memory) and has the settings st.
func testFilter(t *testing.T, st Settings, known ...Embed) *Filter {
	t.Helper()
	testkit.UseEngineRoot(t)
	testkit.WithModules(t, map[string]bool{manifest.Name: true})
	s := NewService(nil, nil, testkit.Logger())
	s.settings = &st
	for _, e := range known {
		s.known[e.URL] = e
		s.loaded[e.URL] = true
	}
	return NewFilter(s, testkit.Templates(), testkit.Logger())
}

// TestFilterApply - a video link alone in a paragraph (its text or a link
// element, at the top or nested) becomes the video: a preview with the
// title, linking to the video, the player's address to start on a click,
// the proportions; a link inside a sentence, another site's, and a text
// without links stay as they are.
func TestFilterApply(t *testing.T) {
	f := testFilter(t, defaultSettings(),
		Embed{URL: testVideo, Provider: "youtube", Title: "Trailer <1>", ThumbnailFile: "video_embed/ab/ab.jpg", Width: 200, Height: 113},
		Embed{URL: testShorts, Provider: "youtube", Title: "Short"})
	ctx := context.Background()
	in := `<p>Intro</p><p>` + testVideo + `</p><blockquote><p><a href="` + testShorts + `">watch</a></p></blockquote><p>See ` + testVideo + ` here.</p><p>https://example.com/video/1</p>`
	out := f.Apply(ctx, in)
	for _, want := range []string{
		`<p>Intro</p>`,
		`<figure class="video-embed" style="--video-embed-w: 200; --video-embed-h: 113">`,
		`href="` + testVideo + `" data-video-player="https://www.youtube-nocookie.com/embed/dQw4w9WgXcQ?autoplay=1"`,
		`<span class="video-embed-title">Trailer &lt;1&gt;</span>`,
		`/styles/` + manifest.PreviewStyle + `/video_embed/ab/ab.webp`,
		`<figure class="video-embed video-embed--portrait" style="--video-embed-w: 9; --video-embed-h: 16">`,
		`<p>See ` + testVideo + ` here.</p>`,
		`<p>https://example.com/video/1</p>`,
		`<!--kosatka:attach video_embed-->`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("no %s in %s", want, out)
		}
	}
	if strings.Count(out, "<figure") != 2 || strings.Contains(out, "<p>"+testVideo+"</p>") || !strings.Contains(out, "<blockquote>") || strings.Index(out, "<blockquote>") > strings.Index(out, "video-embed--portrait") {
		t.Errorf("want 2 videos in place of their paragraphs: %s", out)
	}
	if plain := `<p>No links here.</p>`; f.Apply(ctx, plain) != plain {
		t.Error("a text without links must stay as is")
	}
	if other := `<p>https://example.com/x</p>`; f.Apply(ctx, other) != other {
		t.Error("a text without video links must stay as is")
	}
}

// TestFilterSettings - a service off leaves its links; without "player on
// a click" the player itself, not starting; without local previews the
// service's picture; a link not known yet shows the service's name and
// waits for the background job.
func TestFilterSettings(t *testing.T) {
	ctx := context.Background()
	known := Embed{URL: testVideo, Provider: "youtube", Title: "Trailer", ThumbnailURL: "https://i.ytimg.com/vi/x/hq.jpg"}

	off := testFilter(t, Settings{Providers: []string{"vk"}, ClickToPlay: true, LocalPreviews: true}, known)
	if in := `<p>` + testVideo + `</p>`; off.Apply(ctx, in) != in {
		t.Error("a service off: the link must stay")
	}

	player := testFilter(t, Settings{Providers: []string{"youtube"}}, known)
	out := player.Apply(ctx, `<p>`+testVideo+`</p>`)
	if !strings.Contains(out, `<iframe class="video-embed-player" src="https://www.youtube-nocookie.com/embed/dQw4w9WgXcQ"`) || strings.Contains(out, "autoplay=1") || strings.Contains(out, `class="video-embed-play"`) {
		t.Errorf("no click to play: %s", out)
	}

	remote := testFilter(t, Settings{Providers: []string{"youtube"}, ClickToPlay: true}, known)
	if out := remote.Apply(ctx, `<p>`+testVideo+`</p>`); !strings.Contains(out, `src="https://i.ytimg.com/vi/x/hq.jpg"`) {
		t.Errorf("no local previews: %s", out)
	}

	unknown := testFilter(t, defaultSettings())
	unknown.service.loaded[testShorts] = true // the table asked: nothing there
	out = unknown.Apply(ctx, `<p>`+testShorts+`</p>`)
	if !strings.Contains(out, `<span class="video-embed-title">YouTube</span>`) || strings.Contains(out, "<img") {
		t.Errorf("unknown video: %s", out)
	}
	if _, ok := unknown.service.pending[testShorts]; !ok {
		t.Error("an unknown video must wait for the job")
	}
}

// TestEmbedsRetry - a failed answer is asked again only a day later.
func TestEmbedsRetry(t *testing.T) {
	m, _ := MatchURL(testVideo, map[string]bool{"youtube": true})
	s := NewService(nil, nil, testkit.Logger())
	s.known[testVideo], s.loaded[testVideo] = Embed{URL: testVideo, Failed: true, FetchedAt: time.Now()}, true
	s.Embeds(context.Background(), []Match{m})
	if len(s.pending) != 0 {
		t.Error("failed just now: not again yet")
	}
	s.known[testVideo] = Embed{URL: testVideo, Failed: true, FetchedAt: time.Now().Add(-2 * retryAfter)}
	s.Embeds(context.Background(), []Match{m})
	if len(s.pending) != 1 {
		t.Error("failed a day ago: asked again")
	}
}
