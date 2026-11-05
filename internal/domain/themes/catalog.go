package themes

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
)

// The themes' catalog: an index (index.json) of the themes kept in their own
// repositories, built by app theme index (internal/themeinstall) from a
// list of them - each one's manifest at its newest release, its stars
// and last update from its host. The Appearance page lists it
// (CatalogService) with the command that installs a theme: a theme
// is installed by app theme add, never from the admin.

// Catalog - an index: when it was built and its themes.
type Catalog struct {
	Generated time.Time      `json:"generated"`
	Themes    []CatalogTheme `json:"themes"`
}

// CatalogTheme - a theme of the catalog: Name - the directory app theme
// add makes (themeinstall.NameOf); Source - its repository; Version - its
// newest release's tag, Released - that release's date; the manifest's
// description (Title - its name) at that release; Screenshot - the
// picture's URL at that release ("" - none or not on a known host);
// Stars, Updated - from its host (0 and zero - unknown).
type CatalogTheme struct {
	Name        string    `json:"name"`
	Source      string    `json:"source"`
	Version     string    `json:"version"`
	Commit      string    `json:"commit,omitempty"`
	Released    time.Time `json:"released,omitzero"`
	Title       string    `json:"title"`
	Description string    `json:"description,omitempty"`
	Author      string    `json:"author,omitempty"`
	Homepage    string    `json:"homepage,omitempty"`
	License     string    `json:"license,omitempty"`
	Tags        []string  `json:"tags,omitempty"`
	MinVersion  string    `json:"min_version,omitempty"`
	Base        string    `json:"base,omitempty"`
	Screenshot  string    `json:"screenshot,omitempty"`
	Stars       int       `json:"stars,omitempty"`
	Updated     time.Time `json:"updated,omitzero"`
}

// Catalog limits: the index's size, a theme's texts.
const (
	maxCatalogBytes    = 4 << 20
	maxCatalogText     = 500
	catalogCacheTTL    = time.Hour
	catalogHTTPTimeout = 15 * time.Second
)

var (
	// catalogSourceRe - a repository address app theme add takes (as
	// themeinstall's).
	catalogSourceRe = regexp.MustCompile(`^(https://|ssh://|[A-Za-z0-9._-]+@[A-Za-z0-9.-]+:)[^\s]+$`)
	catalogTagRe    = regexp.MustCompile(`^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)
	catalogNameRe   = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)
)

// clean - the index's themes a page can show: a valid name, source and
// release; an https homepage and screenshot or none; texts cut short;
// the tags lowercase words. Others are left out (a broken entry doesn't
// break the page).
func (c Catalog) clean() Catalog {
	out := Catalog{Generated: c.Generated}
	for _, t := range c.Themes {
		if !catalogNameRe.MatchString(t.Name) || !catalogSourceRe.MatchString(t.Source) || !catalogTagRe.MatchString(t.Version) {
			continue
		}
		t.Title = cutText(t.Title)
		if t.Title == "" {
			t.Title = t.Name
		}
		t.Description, t.Author, t.License = cutText(t.Description), cutText(t.Author), cutText(t.License)
		if !httpsURL(t.Homepage) {
			t.Homepage = ""
		}
		if !httpsURL(t.Screenshot) {
			t.Screenshot = ""
		}
		if t.MinVersion != "" && !versionRe.MatchString(t.MinVersion) {
			t.MinVersion = ""
		}
		var tags []string
		for _, tag := range t.Tags {
			if tagRe.MatchString(tag) && len(tags) < maxTags {
				tags = append(tags, tag)
			}
		}
		t.Tags = tags
		out.Themes = append(out.Themes, t)
	}
	return out
}

func cutText(s string) string {
	s = strings.TrimSpace(s)
	if r := []rune(s); len(r) > maxCatalogText {
		return string(r[:maxCatalogText]) + "…"
	}
	return s
}

func httpsURL(s string) bool {
	u, err := url.Parse(s)
	return err == nil && u.Scheme == "https" && u.Host != ""
}

// Match - whether the theme has every word of query (case aside) in its
// name, title, description, author or tags.
func (t CatalogTheme) Match(query string) bool {
	hay := strings.ToLower(strings.Join(append([]string{t.Name, t.Title, t.Description, t.Author}, t.Tags...), " "))
	for _, w := range strings.Fields(strings.ToLower(query)) {
		if !strings.Contains(hay, w) {
			return false
		}
	}
	return true
}

// CatalogService reads the catalog at URL ("" - none), keeping it an
// hour.
type CatalogService struct {
	URL    string
	Client *http.Client

	mu      sync.Mutex
	cached  Catalog
	fetched time.Time
}

// ErrNoCatalog - no catalog is configured (themes_catalog_url).
var ErrNoCatalog = errors.New("no themes catalog configured")

// Catalog - the catalog (its cached copy for an hour).
func (s *CatalogService) Catalog(ctx context.Context) (Catalog, error) {
	if s == nil || s.URL == "" {
		return Catalog{}, ErrNoCatalog
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.fetched.IsZero() && time.Since(s.fetched) < catalogCacheTTL {
		return s.cached, nil
	}
	c, err := FetchCatalog(ctx, s.Client, s.URL)
	if err != nil {
		return Catalog{}, err
	}
	s.cached, s.fetched = c, time.Now()
	return c, nil
}

// FetchCatalog reads the index at address (https, or a local file
// path for a site's own) - its valid themes (Catalog.clean).
func FetchCatalog(ctx context.Context, client *http.Client, address string) (Catalog, error) {
	var body io.ReadCloser
	if strings.HasPrefix(address, "https://") || strings.HasPrefix(address, "http://") {
		if client == nil {
			client = &http.Client{Timeout: catalogHTTPTimeout}
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
		if err != nil {
			return Catalog{}, err
		}
		resp, err := client.Do(req)
		if err != nil {
			return Catalog{}, fmt.Errorf("fetching the themes catalog: %w", err)
		}
		if resp.StatusCode != http.StatusOK {
			_ = resp.Body.Close()
			return Catalog{}, fmt.Errorf("fetching the themes catalog: %s", resp.Status)
		}
		body = resp.Body
	} else {
		f, err := os.Open(address)
		if err != nil {
			return Catalog{}, fmt.Errorf("reading the themes catalog: %w", err)
		}
		body = f
	}
	defer func() { _ = body.Close() }()
	b, err := io.ReadAll(io.LimitReader(body, maxCatalogBytes+1))
	if err != nil {
		return Catalog{}, fmt.Errorf("reading the themes catalog: %w", err)
	}
	if len(b) > maxCatalogBytes {
		return Catalog{}, fmt.Errorf("the themes catalog is over %d bytes", maxCatalogBytes)
	}
	var c Catalog
	if err := json.Unmarshal(b, &c); err != nil {
		return Catalog{}, fmt.Errorf("parsing the themes catalog: %w", err)
	}
	return c.clean(), nil
}
