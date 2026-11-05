package videoembed

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/modulekit"
)

// formatterNamed - the module's link field formatter of that name.
func formatterNamed(t *testing.T, f *Filter, name string) modulekit.FieldFormatter {
	t.Helper()
	for _, ff := range f.FieldFormatters() {
		if ff.Name == name {
			return ff
		}
	}
	t.Fatalf("no formatter %s", name)
	return modulekit.FieldFormatter{}
}

// TestFieldFormatters - a link field shown as videos: the players, or the
// previews starting the player on a click; a link's own title stands for
// its video's, a link of no service on stays a link; "featured" - the
// first as chosen, the others as small previews after it. The sites to
// offer are the services' on.
func TestFieldFormatters(t *testing.T) {
	f := testFilter(t, defaultSettings(),
		Embed{URL: testVideo, Provider: "youtube", Title: "Trailer"},
		Embed{URL: testShorts, Provider: "youtube", Title: "Short"})
	ctx := context.Background()
	links := []modulekit.Link{{URL: testVideo, Title: "Main trailer"}, {URL: "https://example.com/news"}, {URL: testShorts}}

	out := string(formatterNamed(t, f, formatterPlayer).Render(ctx, links, modulekit.FieldLayoutAll))
	for _, want := range []string{
		`<div class="video-embed-field">`,
		`<iframe class="video-embed-player" src="https://www.youtube-nocookie.com/embed/dQw4w9WgXcQ" title="Main trailer"`,
		`<p class="video-embed-link"><a href="https://example.com/news" rel="nofollow noopener">https://example.com/news</a></p>`,
		`src="https://www.youtube-nocookie.com/embed/aqz-KE-bpKQ" title="Short"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("players: no %s in %s", want, out)
		}
	}
	if strings.Contains(out, `class="video-embed-play"`) || strings.Contains(out, "video-embed-more") {
		t.Errorf("players: previews in %s", out)
	}

	out = string(formatterNamed(t, f, formatterPreview).Render(ctx, links[:1], modulekit.FieldLayoutAll))
	if !strings.Contains(out, `<a class="video-embed-play" href="`+testVideo+`" data-video-player="https://www.youtube-nocookie.com/embed/dQw4w9WgXcQ?autoplay=1" data-video-title="Main trailer"`) {
		t.Errorf("preview: %s", out)
	}

	out = string(formatterNamed(t, f, formatterPlayer).Render(ctx, links, modulekit.FieldLayoutFeatured))
	player, more := strings.Index(out, `<iframe class="video-embed-player"`), strings.Index(out, `<div class="video-embed-more">`)
	if !strings.Contains(out, "video-embed-field--featured") || player < 0 || more < player || !strings.Contains(out[more:], `data-video-player="https://www.youtube-nocookie.com/embed/aqz-KE-bpKQ?autoplay=1"`) {
		t.Errorf("featured: %s", out)
	}

	if out := formatterNamed(t, f, formatterPlayer).Render(ctx, nil, modulekit.FieldLayoutAll); out != "" {
		t.Errorf("no links: %q", out)
	}
	if got := formatterNamed(t, f, formatterPlayer).Hosts(ctx); !slices.Contains(got, "youtube.com") || slices.Contains(got, "vimeo.com") {
		t.Errorf("sites %v: want the services on only", got)
	}
}
