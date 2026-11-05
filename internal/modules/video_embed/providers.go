package videoembed

import (
	_ "embed"
	"encoding/json"
	"net/url"
	"regexp"
	"strings"
)

// Provider - a video service (providers.json): Name - its machine name
// (the settings' list), Label, Hosts - what the settings page shows;
// Sites - the hosts of its links (a link field's allowed sites); On -
// on unless the settings say otherwise; URLs - the links to its videos;
// Player - the player's address, {id}/{oid} from the link; Autoplay - the
// query parameter starting it at once (after a click); OEmbed - the
// address of its oEmbed answer ({url} - the link), or OpenGraph - the
// video's page's Open Graph tags instead; ThumbnailHosts - the hosts (and
// their subdomains) its previews may come from.
type Provider struct {
	Name           string        `json:"name"`
	Label          string        `json:"label"`
	Hosts          string        `json:"hosts"`
	Sites          []string      `json:"sites"`
	On             bool          `json:"on"`
	URLs           []providerURL `json:"urls"`
	Player         string        `json:"player"`
	Autoplay       string        `json:"autoplay"`
	OEmbed         string        `json:"oembed"`
	OpenGraph      bool          `json:"opengraph"`
	ThumbnailHosts []string      `json:"thumbnail_hosts"`
}

// providerURL - a kind of a provider's links: Pattern with the named
// groups the player needs; Width/Height - the proportions of its videos
// when they aren't the service's (Shorts are vertical though oEmbed says
// otherwise), 0 - the service's.
type providerURL struct {
	Pattern string `json:"pattern"`
	Width   int    `json:"width"`
	Height  int    `json:"height"`
	re      *regexp.Regexp
}

//go:embed providers.json
var providersJSON []byte

// providers - the services, in the settings page's order.
var providers = mustProviders(providersJSON)

// mustProviders parses the embedded providers.json (a constant: broken -
// a panic at start, TestProviders catches it first).
func mustProviders(data []byte) []Provider {
	var list []Provider
	if err := json.Unmarshal(data, &list); err != nil {
		panic("video_embed: providers.json: " + err.Error())
	}
	for i := range list {
		for j := range list[i].URLs {
			list[i].URLs[j].re = regexp.MustCompile(list[i].URLs[j].Pattern)
		}
	}
	return list
}

// Match - a link recognized as a provider's video: URL as written, the
// player's address (PlayerAutoplay - starting at once), the proportions
// its link kind fixes (0 - none).
type Match struct {
	Provider *Provider
	URL      string
	Player   string
	Width    int
	Height   int
}

// PlayerAutoplay - the player's address starting the video at once (a
// click on the preview asked for it).
func (m Match) PlayerAutoplay() string {
	if m.Provider.Autoplay == "" {
		return m.Player
	}
	sep := "?"
	if strings.Contains(m.Player, "?") {
		sep = "&"
	}
	return m.Player + sep + m.Provider.Autoplay
}

// Portrait - whether the link's kind fixes vertical videos.
func (m Match) Portrait() bool { return m.Height > m.Width }

// MatchURL recognizes link as a video of one of the providers on (on -
// by name); false - not a video link of theirs.
func MatchURL(link string, on map[string]bool) (Match, bool) {
	link = strings.TrimSpace(link)
	if link == "" || len(link) > 2048 {
		return Match{}, false
	}
	for i := range providers {
		p := &providers[i]
		if !on[p.Name] {
			continue
		}
		for _, u := range p.URLs {
			m := u.re.FindStringSubmatch(link)
			if m == nil {
				continue
			}
			player := p.Player
			for k, name := range u.re.SubexpNames() {
				if name != "" {
					player = strings.ReplaceAll(player, "{"+name+"}", url.QueryEscape(m[k]))
				}
			}
			return Match{Provider: p, URL: link, Player: player, Width: u.Width, Height: u.Height}, true
		}
	}
	return Match{}, false
}

// oembedURL - the address of the provider's oEmbed answer about link.
func (p *Provider) oembedURL(link string) string {
	return strings.ReplaceAll(p.OEmbed, "{url}", url.QueryEscape(link))
}

// thumbnailAllowed - whether a preview's address is one the provider's
// previews come from: https, a host of ThumbnailHosts or its subdomain.
func (p *Provider) thumbnailAllowed(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "https" {
		return false
	}
	host := strings.ToLower(u.Hostname())
	for _, h := range p.ThumbnailHosts {
		if host == h || strings.HasSuffix(host, "."+h) {
			return true
		}
	}
	return false
}

// defaultProviders - the providers on when the settings say nothing.
func defaultProviders() []string {
	var names []string
	for _, p := range providers {
		if p.On {
			names = append(names, p.Name)
		}
	}
	return names
}
