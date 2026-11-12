package interfaces

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gorilla/sessions"
	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/pkg/csrf"
)

// browserUA - a browser's user agent: its views are counted.
const browserUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/145.0.0.0 Safari/537.36"

// viewCountMissing - counting finds no node (as the service reports it).
type viewCountMissing struct{ service.ViewCountingService }

func (viewCountMissing) CountView(_ context.Context, id int) error {
	return fmt.Errorf("counting a view (node %d): %w", id, fmt.Errorf("%w: node with ID %d", entity.ErrEntityNotFound, id))
}

// TestPostViewMissingNode - a view of a node that doesn't exist (often a
// bot's made-up ID) answers 404 without logging an error.
func TestPostViewMissingNode(t *testing.T) {
	store := sessions.NewCookieStore([]byte("test-secret-key-0123456789abcdef"))
	w := httptest.NewRecorder()
	token := csrf.EnsureToken(store, w, httptest.NewRequest(http.MethodGet, "/", nil))
	var logs bytes.Buffer
	c := NewViewCountController(slog.New(slog.NewTextHandler(&logs, nil)), viewCountMissing{})

	req := httptest.NewRequest(http.MethodPost, "/api/node/view", strings.NewReader(url.Values{"nid": {"999"}, "csrf_token": {token}}.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", browserUA)
	for _, ck := range w.Result().Cookies() {
		req.AddCookie(ck)
	}
	rec := httptest.NewRecorder()
	c.PostView(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("code %d, want 404", rec.Code)
	}
	if strings.Contains(logs.String(), "level=ERROR") {
		t.Errorf("logged an error: %s", logs.String())
	}
}

// viewCountRecorder counts the views it is given; nid 2 is unpublished.
type viewCountRecorder struct {
	service.ViewCountingService
	counted []int
}

func (v *viewCountRecorder) CountView(_ context.Context, id int) error {
	if id == 2 {
		return service.ErrNodeNotPublic
	}
	v.counted = append(v.counted, id)
	return nil
}

// TestPostViewOncePerClient - a client's repeated view of a node isn't
// counted; another node's or another client's is; a crawler's isn't, and
// answers as a counted one; an unpublished node answers like a missing
// one.
func TestPostViewOncePerClient(t *testing.T) {
	rec := &viewCountRecorder{}
	c := NewViewCountController(slog.New(slog.NewTextHandler(io.Discard, nil)), rec)
	ua := browserUA
	post := func(ip, nid string) int {
		req := httptest.NewRequest(http.MethodPost, "/api/node/view", strings.NewReader(url.Values{"nid": {nid}}.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("User-Agent", ua)
		req.RemoteAddr = ip + ":1234"
		w := httptest.NewRecorder()
		c.PostView(w, req)
		return w.Code
	}
	for _, v := range [][2]string{{"192.0.2.1", "1"}, {"192.0.2.1", "1"}, {"192.0.2.1", "3"}, {"192.0.2.2", "1"}} {
		if code := post(v[0], v[1]); code != http.StatusNoContent {
			t.Fatalf("%v: %d", v, code)
		}
	}
	ua = "Mozilla/5.0 (compatible; YandexBot/3.0; +http://yandex.com/bots) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/108.0.0.0"
	if code := post("192.0.2.3", "1"); code != http.StatusNoContent {
		t.Errorf("crawler: %d, want 204", code)
	}
	if fmt.Sprint(rec.counted) != "[1 3 1]" {
		t.Errorf("counted %v", rec.counted)
	}
	ua = browserUA
	if code := post("192.0.2.1", "2"); code != http.StatusNotFound {
		t.Errorf("unpublished: %d, want 404", code)
	}
}

// TestIsCrawler - the crawlers that ran the page's script on a real site,
// and the browsers that look like them.
func TestIsCrawler(t *testing.T) {
	crawlers := []string{
		"Mozilla/5.0 (compatible; YandexRenderResourcesBot/1.0; +http://yandex.com/bots) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/108.0.0.0",
		"Mozilla/5.0 (iPhone; CPU iPhone OS 18_7 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/26.2 Mobile/15E148 Safari/604.1 (compatible; YandexRenderResourcesBot/1.0; +http://yandex.com/bots)",
		"Mozilla/5.0 (compatible; Baiduspider-render/2.0; +http://www.baidu.com/search/spider.html)",
		"Mozilla/5.0 AppleWebKit/537.36 (KHTML, like Gecko; compatible; bingbot/2.0; +http://www.bing.com/bingbot.htm) Chrome/136.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Linux; Android 6.0.1; Nexus 5X Build/MMB29P) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/146.0.0.0 Mobile Safari/537.36 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)",
		"Mozilla/5.0 (X11; Linux x86_64; Storebot-Google/1.0) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/147.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/145.0.0.0 Safari/537.36 (compatible; meta-webindexer/1.1 (+https://developers.facebook.com/docs/sharing/webmasters/crawler))",
		"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) HeadlessChrome/145.0.0.0 Safari/537.36",
		"TwitterBot",
		"",
	}
	for _, ua := range crawlers {
		r := httptest.NewRequest(http.MethodPost, "/api/node/view", nil)
		r.Header.Set("User-Agent", ua)
		if !isCrawler(r) {
			t.Errorf("not a crawler: %q", ua)
		}
	}
	browsers := []string{
		browserUA,
		"Mozilla/5.0 (Linux; Android 10; K) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/155.0.0.0 Mobile Safari/537.36",
		"Mozilla/5.0 (iPhone; CPU iPhone OS 18_7 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/26.2 Mobile/15E148 Safari/604.1",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:140.0) Gecko/20100101 Firefox/140.0",
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/144.0.0.0 YaBrowser/25.8.0.0 Safari/537.36",
		"Mozilla/5.0 (Linux; Android 9; CUBOT_X30) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Mobile Safari/537.36",
		"Mozilla/5.0 (Linux; Android 11; CUBOT P50 Build/RP1A.200720.011) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Mobile Safari/537.36",
	}
	for _, ua := range browsers {
		r := httptest.NewRequest(http.MethodPost, "/api/node/view", nil)
		r.Header.Set("User-Agent", ua)
		if isCrawler(r) {
			t.Errorf("a crawler: %q", ua)
		}
	}
}
