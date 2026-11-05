package videoembed

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/kosatka-cms/kosatka/internal/modules/video_embed/ent/generated"
	"github.com/kosatka-cms/kosatka/internal/modules/video_embed/ent/generated/videoembed"
	"github.com/kosatka-cms/kosatka/modulekit"
)

// Embed - what a video's service says about it (oEmbed or Open Graph):
// Title, the preview (ThumbnailURL - the service's, ThumbnailFile - the
// site's copy, relative to the public files, "" - none), the player's
// proportions (0 - unknown); Failed - the service didn't answer (asked
// again after retryAfter).
type Embed struct {
	URL           string
	Provider      string
	Title         string
	ThumbnailURL  string
	ThumbnailFile string
	Width         int
	Height        int
	Failed        bool
	FetchedAt     time.Time
}

// retryAfter - when a failed video is asked about again.
const retryAfter = 24 * time.Hour

// Settings - the module's settings: Providers - the services on, by name;
// ClickToPlay - a preview first, the player on a click; LocalPreviews -
// the previews are the site's copies.
type Settings struct {
	Providers     []string
	ClickToPlay   bool
	LocalPreviews bool
}

// On - the providers on, by name.
func (s Settings) On() map[string]bool {
	on := make(map[string]bool, len(s.Providers))
	for _, n := range s.Providers {
		on[n] = true
	}
	return on
}

// defaultSettings - the settings until the page is saved.
func defaultSettings() Settings {
	return Settings{Providers: defaultProviders(), ClickToPlay: true, LocalPreviews: true}
}

// Repo - the module's tables.
type Repo struct {
	db *sql.DB
}

// NewRepo creates a Repo on db, in the repositories' dialect.
func NewRepo(db *sql.DB) *Repo {
	return &Repo{db: db}
}

// client - the module's ent client: the context's transaction (InTx - a
// hook running in the core's change, say), else the database.
func (r *Repo) client(ctx context.Context) *generated.Client {
	return generated.NewClient(generated.Driver(modulekit.Driver(ctx, r.db)))
}

// urlHash - a link's key in video_embeds.
func urlHash(link string) string {
	sum := sha256.Sum256([]byte(link))
	return hex.EncodeToString(sum[:])
}

// Settings returns the module's settings (the defaults until saved).
func (r *Repo) Settings(ctx context.Context) (Settings, error) {
	row, err := r.client(ctx).VideoEmbedSettings.Get(ctx, 1)
	if generated.IsNotFound(err) {
		return defaultSettings(), nil
	}
	if err != nil {
		return Settings{}, fmt.Errorf("loading the video embed settings: %w", err)
	}
	return Settings{Providers: strings.Fields(row.Providers), ClickToPlay: row.ClickToPlay, LocalPreviews: row.LocalPreviews}, nil
}

// SaveSettings saves the module's settings.
func (r *Repo) SaveSettings(ctx context.Context, s Settings) error {
	providers := strings.Join(s.Providers, " ")
	err := r.client(ctx).VideoEmbedSettings.UpdateOneID(1).SetProviders(providers).
		SetClickToPlay(s.ClickToPlay).SetLocalPreviews(s.LocalPreviews).Exec(ctx)
	if generated.IsNotFound(err) {
		err = r.client(ctx).VideoEmbedSettings.Create().SetID(1).SetProviders(providers).
			SetClickToPlay(s.ClickToPlay).SetLocalPreviews(s.LocalPreviews).Exec(ctx)
	}
	if err != nil {
		return fmt.Errorf("saving the video embed settings: %w", err)
	}
	return nil
}

// Embeds returns what's known about the links, by link (a link not
// asked about yet - absent).
func (r *Repo) Embeds(ctx context.Context, links []string) (map[string]Embed, error) {
	if len(links) == 0 {
		return map[string]Embed{}, nil
	}
	hashes := make([]string, len(links))
	for i, l := range links {
		hashes[i] = urlHash(l)
	}
	rows, err := r.client(ctx).VideoEmbed.Query().Where(videoembed.URLHashIn(hashes...)).All(ctx)
	if err != nil {
		return nil, fmt.Errorf("loading video embeds: %w", err)
	}
	out := make(map[string]Embed, len(rows))
	for _, row := range rows {
		out[row.URL] = Embed{URL: row.URL, Provider: row.Provider, Title: row.Title, ThumbnailURL: row.ThumbnailURL,
			ThumbnailFile: row.ThumbnailFile, Width: row.Width, Height: row.Height, Failed: row.Status == "failed", FetchedAt: row.FetchedAt}
	}
	return out, nil
}

// SaveEmbed stores what the service said about the link (replacing what
// was known).
func (r *Repo) SaveEmbed(ctx context.Context, e Embed) error {
	status := "ok"
	if e.Failed {
		status = "failed"
	}
	hash := urlHash(e.URL)
	n, err := r.client(ctx).VideoEmbed.Update().Where(videoembed.URLHash(hash)).
		SetProvider(e.Provider).SetTitle(e.Title).SetThumbnailURL(e.ThumbnailURL).SetThumbnailFile(e.ThumbnailFile).
		SetWidth(e.Width).SetHeight(e.Height).SetStatus(status).SetFetchedAt(e.FetchedAt).Save(ctx)
	if err == nil && n == 0 {
		err = r.client(ctx).VideoEmbed.Create().SetURLHash(hash).SetURL(e.URL).
			SetProvider(e.Provider).SetTitle(e.Title).SetThumbnailURL(e.ThumbnailURL).SetThumbnailFile(e.ThumbnailFile).
			SetWidth(e.Width).SetHeight(e.Height).SetStatus(status).SetFetchedAt(e.FetchedAt).Exec(ctx)
	}
	if err != nil {
		return fmt.Errorf("saving the video embed (url %v): %w", e.URL, err)
	}
	return nil
}
