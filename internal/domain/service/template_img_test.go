package service

import (
	"html/template"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/imageurl"
)

// TestImgTag - an <img> of a picture: src, srcset, the size,
// alt, lazy loading; attributes added or replacing those it sets, never
// src, srcset, alt or a handler; values escaped; no picture - nothing.
func TestImgTag(t *testing.T) {
	p := imageurl.Picture{Src: "/a.webp", Srcset: "/a.webp 1x, /b.webp 2x", Width: 720, Height: 405}
	got, err := imgTag(p, `Cat "Tom"`, "class", "photo")
	want := `<img src="/a.webp" srcset="/a.webp 1x, /b.webp 2x" width="720" height="405" alt="Cat &#34;Tom&#34;" loading="lazy" decoding="async" class="photo">`
	if err != nil || string(got) != want {
		t.Errorf("img:\n got %s %v\nwant %s", got, err, want)
	}
	got, _ = imgTag(p, "", "loading", "eager", "width", "360", "fetchpriority", "high")
	want = `<img src="/a.webp" srcset="/a.webp 1x, /b.webp 2x" width="360" height="405" alt="" loading="eager" decoding="async" fetchpriority="high">`
	if string(got) != want {
		t.Errorf("replaced:\n got %s\nwant %s", got, want)
	}
	if got, _ := imgTag(imageurl.Picture{Src: "/c.jpg"}, "C"); got != `<img src="/c.jpg" alt="C" loading="lazy" decoding="async">` {
		t.Errorf("no size: %s", got)
	}
	if got, _ := imgTag(imageurl.Picture{Src: "/c.jpg"}, template.HTML("Tom &amp; Jerry")); got != `<img src="/c.jpg" alt="Tom &amp; Jerry" loading="lazy" decoding="async">` {
		t.Errorf("a translation's alt: %s", got)
	}
	if got, err := imgTag(imageurl.Picture{}, "x"); got != "" || err != nil {
		t.Errorf("no picture: %q %v", got, err)
	}
	for _, attrs := range [][]string{{"onerror", "x"}, {"src", "/x"}, {"alt", "x"}, {"Class", "x"}, {"class"}} {
		if _, err := imgTag(p, "", attrs...); err == nil {
			t.Errorf("%q: no error", attrs)
		}
	}
}
