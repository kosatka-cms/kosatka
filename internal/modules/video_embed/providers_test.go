package videoembed

import "testing"

// TestProviders - every service of providers.json has links, a player,
// and a way to learn about its videos; its link kinds' proportions are
// both set or neither.
func TestProviders(t *testing.T) {
	if len(providers) == 0 {
		t.Fatal("no providers")
	}
	for _, p := range providers {
		if p.Name == "" || p.Label == "" || p.Player == "" || len(p.URLs) == 0 || len(p.ThumbnailHosts) == 0 || len(p.Sites) == 0 {
			t.Errorf("%s: incomplete: %+v", p.Name, p)
		}
		if p.OEmbed == "" && !p.OpenGraph {
			t.Errorf("%s: neither oEmbed nor Open Graph", p.Name)
		}
		for _, u := range p.URLs {
			if (u.Width > 0) != (u.Height > 0) {
				t.Errorf("%s: %s: proportions half set", p.Name, u.Pattern)
			}
		}
	}
}

// TestMatchURL - the links of the services on are recognized, with the
// player's address (the autoplay one after a click) and the proportions
// of their kind; a service off, another site, a page of the service
// that isn't a video, and an empty link are not.
func TestMatchURL(t *testing.T) {
	all := map[string]bool{"youtube": true, "vk": true, "rutube": true, "vimeo": true}
	for _, tc := range []struct {
		link, provider, player, autoplay string
		portrait                         bool
	}{
		{"https://www.youtube.com/watch?v=dQw4w9WgXcQ", "youtube", "https://www.youtube-nocookie.com/embed/dQw4w9WgXcQ", "https://www.youtube-nocookie.com/embed/dQw4w9WgXcQ?autoplay=1", false},
		{"https://www.youtube.com/watch?feature=share&v=dQw4w9WgXcQ&t=10", "youtube", "https://www.youtube-nocookie.com/embed/dQw4w9WgXcQ", "", false},
		{"https://youtu.be/dQw4w9WgXcQ?si=x", "youtube", "https://www.youtube-nocookie.com/embed/dQw4w9WgXcQ", "", false},
		{"https://m.youtube.com/shorts/aqz-KE-bpKQ", "youtube", "https://www.youtube-nocookie.com/embed/aqz-KE-bpKQ", "", true},
		{"https://vkvideo.ru/video-22822305_456242110", "vk", "https://vkvideo.ru/video_ext.php?oid=-22822305&id=456242110&hd=2", "https://vkvideo.ru/video_ext.php?oid=-22822305&id=456242110&hd=2&autoplay=1", false},
		{"https://vk.com/clip-1_2", "vk", "https://vkvideo.ru/video_ext.php?oid=-1&id=2&hd=2", "", true},
		{"https://rutube.ru/video/c6cc4d620b1d4338901770a44b3e82f4/", "rutube", "https://rutube.ru/play/embed/c6cc4d620b1d4338901770a44b3e82f4", "https://rutube.ru/play/embed/c6cc4d620b1d4338901770a44b3e82f4?autoplay=true", false},
		{"https://vimeo.com/1084537", "vimeo", "https://player.vimeo.com/video/1084537?dnt=1", "https://player.vimeo.com/video/1084537?dnt=1&autoplay=1", false},
	} {
		m, ok := MatchURL(tc.link, all)
		if !ok || m.Provider.Name != tc.provider || m.Player != tc.player || m.Portrait() != tc.portrait || m.URL != tc.link {
			t.Errorf("%s: %+v %v", tc.link, m, ok)
			continue
		}
		if tc.autoplay != "" && m.PlayerAutoplay() != tc.autoplay {
			t.Errorf("%s: autoplay %s", tc.link, m.PlayerAutoplay())
		}
	}
	for _, link := range []string{"", "https://example.com/watch?v=dQw4w9WgXcQ", "https://www.youtube.com/@channel", "https://vk.com/id1", "javascript:alert(1)"} {
		if m, ok := MatchURL(link, all); ok {
			t.Errorf("%q matched: %+v", link, m)
		}
	}
	if _, ok := MatchURL("https://vimeo.com/1084537", map[string]bool{"youtube": true}); ok {
		t.Error("a service off matched")
	}
}

// TestThumbnailAllowed - a preview comes over https from one of the
// service's hosts or their subdomains only.
func TestThumbnailAllowed(t *testing.T) {
	p := Provider{ThumbnailHosts: []string{"ytimg.com"}}
	for raw, want := range map[string]bool{
		"https://i.ytimg.com/vi/x/hqdefault.jpg": true,
		"https://ytimg.com/x.jpg":                true,
		"http://i.ytimg.com/x.jpg":               false,
		"https://evilytimg.com/x.jpg":            false,
		"https://ytimg.com.evil.example/x.jpg":   false,
		"https://127.0.0.1/x.jpg":                false,
	} {
		if got := p.thumbnailAllowed(raw); got != want {
			t.Errorf("%s: %v, want %v", raw, got, want)
		}
	}
}
