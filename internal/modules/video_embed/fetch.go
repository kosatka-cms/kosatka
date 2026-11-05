package videoembed

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/kosatka-cms/kosatka/internal/modules/video_embed/manifest"
	"github.com/kosatka-cms/kosatka/modulekit"
	"github.com/kosatka-cms/kosatka/pkg/filestore"
	"golang.org/x/net/html"
)

// The limits of what a service's answer may be.
const (
	fetchTimeout   = 10 * time.Second
	metaMaxBytes   = 2 << 20
	thumbMaxBytes  = 5 << 20
	titleMaxLength = 512
)

// errNotOK - a service answered other than 200.
var errNotOK = errors.New("not 200")

// Fetcher asks a video's service about it (oEmbed, else its page's Open
// Graph tags) and copies its preview to the site.
type Fetcher struct {
	client *http.Client
	// files - the store the previews' copies go to (the public files).
	files func() filestore.Store
}

// NewFetcher creates a Fetcher keeping the previews under the public
// files' video_embed/.
func NewFetcher() *Fetcher {
	return &Fetcher{
		client: &http.Client{Timeout: fetchTimeout},
		files:  modulekit.PublicFiles,
	}
}

// Fetch returns what the service says about the video: a failed one when
// it doesn't answer. With local, the preview is copied to the site.
func (f *Fetcher) Fetch(ctx context.Context, m Match, local bool) Embed {
	e := Embed{URL: m.URL, Provider: m.Provider.Name, FetchedAt: time.Now()}
	var err error
	switch {
	case m.Provider.OEmbed != "":
		err = f.oembed(ctx, m, &e)
	case m.Provider.OpenGraph:
		err = f.openGraph(ctx, m, &e)
	}
	if err != nil {
		e.Failed = true
		return e
	}
	if m.Width > 0 && m.Height > 0 {
		e.Width, e.Height = m.Width, m.Height
	}
	if e.ThumbnailURL != "" && !m.Provider.thumbnailAllowed(e.ThumbnailURL) {
		e.ThumbnailURL = ""
	}
	if local && e.ThumbnailURL != "" {
		// No copy - no preview: better than the service's address the
		// settings ask not to load.
		e.ThumbnailFile, _ = f.copyThumbnail(ctx, e.ThumbnailURL, urlHash(m.URL))
	}
	e.Title = cutTitle(e.Title)
	return e
}

// get - the body of a GET of address, at most limit bytes.
func (f *Fetcher) get(ctx context.Context, address string, limit int64) ([]byte, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return nil, "", fmt.Errorf("request %s: %w", address, err)
	}
	req.Header.Set("User-Agent", "Kosatka (video embeds)")
	resp, err := f.client.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("fetching %s: %w", address, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("fetching %s: %d: %w", address, resp.StatusCode, errNotOK)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, limit))
	if err != nil {
		return nil, "", fmt.Errorf("reading %s: %w", address, err)
	}
	return body, resp.Header.Get("Content-Type"), nil
}

// oembed fills e from the provider's oEmbed answer.
func (f *Fetcher) oembed(ctx context.Context, m Match, e *Embed) error {
	body, _, err := f.get(ctx, m.Provider.oembedURL(m.URL), metaMaxBytes)
	if err != nil {
		return err
	}
	var answer struct {
		Title        string          `json:"title"`
		ThumbnailURL string          `json:"thumbnail_url"`
		Width        json.RawMessage `json:"width"`
		Height       json.RawMessage `json:"height"`
	}
	if err := json.Unmarshal(body, &answer); err != nil {
		return fmt.Errorf("oEmbed of %s: %w", m.URL, err)
	}
	e.Title, e.ThumbnailURL = answer.Title, answer.ThumbnailURL
	e.Width, e.Height = jsonInt(answer.Width), jsonInt(answer.Height)
	return nil
}

// jsonInt - an oEmbed size: a number or a string of one (0 - neither).
func jsonInt(raw json.RawMessage) int {
	s := strings.Trim(string(raw), `"`)
	n, err := strconv.Atoi(s)
	if err != nil {
		f, ferr := strconv.ParseFloat(s, 64)
		if ferr != nil {
			return 0
		}
		n = int(f)
	}
	return max(n, 0)
}

// openGraph fills e from the video's page's og: tags.
func (f *Fetcher) openGraph(ctx context.Context, m Match, e *Embed) error {
	body, _, err := f.get(ctx, m.URL, metaMaxBytes)
	if err != nil {
		return err
	}
	tags := openGraphTags(string(body))
	e.Title, e.ThumbnailURL = tags["og:title"], tags["og:image"]
	e.Width, _ = strconv.Atoi(tags["og:video:width"])
	e.Height, _ = strconv.Atoi(tags["og:video:height"])
	if e.Title == "" && e.ThumbnailURL == "" {
		return fmt.Errorf("no Open Graph tags on %s: %w", m.URL, errNotOK)
	}
	return nil
}

// openGraphTags - the page's <meta property="og:…" content="…"> values.
func openGraphTags(page string) map[string]string {
	tags := map[string]string{}
	z := html.NewTokenizer(strings.NewReader(page))
	for {
		switch z.Next() {
		case html.ErrorToken:
			return tags
		case html.StartTagToken, html.SelfClosingTagToken:
			t := z.Token()
			if t.Data == "body" {
				return tags
			}
			if t.Data != "meta" {
				continue
			}
			var prop, content string
			for _, a := range t.Attr {
				switch a.Key {
				case "property", "name":
					prop = a.Val
				case "content":
					content = a.Val
				}
			}
			if strings.HasPrefix(prop, "og:") {
				if _, seen := tags[prop]; !seen {
					tags[prop] = content
				}
			}
		}
	}
}

// copyThumbnail saves the preview under the public files
// (video_embed/<2 of hash>/<hash>.<ext>) and returns its path relative
// to them.
func (f *Fetcher) copyThumbnail(ctx context.Context, address, hash string) (string, error) {
	body, contentType, err := f.get(ctx, address, thumbMaxBytes)
	if err != nil {
		return "", err
	}
	mediaType, _, _ := mime.ParseMediaType(contentType)
	if mediaType == "" {
		mediaType = http.DetectContentType(body)
	}
	ext := map[string]string{"image/jpeg": ".jpg", "image/png": ".png", "image/webp": ".webp"}[mediaType]
	if ext == "" {
		return "", fmt.Errorf("preview %s: %s isn't a picture: %w", address, mediaType, errNotOK)
	}
	rel := path.Join(manifest.Name, hash[:2], hash+ext)
	if _, err := f.files().Put(ctx, rel, bytes.NewReader(body), filestore.PutOptions{ContentType: mediaType}); err != nil {
		return "", fmt.Errorf("saving the preview: %w", err)
	}
	return rel, nil
}

// cutTitle - the title within the column's length, a character not cut.
func cutTitle(s string) string {
	s = strings.TrimSpace(s)
	if utf8.RuneCountInString(s) <= titleMaxLength {
		return s
	}
	return string([]rune(s)[:titleMaxLength])
}
