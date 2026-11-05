package interfaces

import (
	"net/http"
	"slices"
	"strings"

	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/pkg/httputil"
)

// embedSearchLimit - how many things the editor's "Insert" window lists;
// embedDescribeLimit - how many tags one request describes.
const (
	embedSearchLimit   = 20
	embedDescribeLimit = 50
)

// EmbedsController - the editor's "Insert" window and the cards over the
// embed tags in a text (service.EmbedPickerKinds, SearchEmbeds,
// DescribeEmbed): JSON for whoever may write in a text format with
// "Embeds" on, the things as they may see them.
type EmbedsController struct {
	*BaseController
	formats service.TextFormatService
}

// NewEmbedsController - the controller over the text formats (who may
// embed).
func NewEmbedsController(deps BaseDeps, formats service.TextFormatService) *EmbedsController {
	base := NewBaseController(deps)
	base.logger = deps.Logger.With("component", "embeds.handler")
	return &EmbedsController{BaseController: base, formats: formats}
}

// mayEmbed - whether r's user may write in a text format with "Embeds"
// on (CanUseTextFormat); a failed check denies.
func (c *EmbedsController) mayEmbed(r *http.Request) bool {
	user, _ := c.GetCurrentUser(r)
	if user == nil {
		return false
	}
	formats, err := c.formats.TextFormats(r.Context())
	if err != nil {
		c.LogFailure(err, "listing the text formats for embedding")
		return false
	}
	for _, f := range formats {
		if slices.Contains(f.FilterNames(), service.EmbedsFilter) && c.authService.CanUseTextFormat(r.Context(), user, f) {
			return true
		}
	}
	return false
}

// Kinds handles GET /api/embeds/kinds: {"kinds": [{name, label}]}, the
// window's choice.
func (c *EmbedsController) Kinds(w http.ResponseWriter, r *http.Request) {
	if !c.mayEmbed(r) {
		httputil.SendJSON(w, http.StatusForbidden, map[string]any{"error": "forbidden"})
		return
	}
	httputil.SendJSON(w, http.StatusOK, map[string]any{"kinds": service.EmbedPickerKinds(r.Context())})
}

// Search handles GET /api/embeds/search?kind=&q=: {"options": [...]}, the
// window's list.
func (c *EmbedsController) Search(w http.ResponseWriter, r *http.Request) {
	if !c.mayEmbed(r) {
		httputil.SendJSON(w, http.StatusForbidden, map[string]any{"error": "forbidden"})
		return
	}
	opts, err := service.SearchEmbeds(r.Context(), r.URL.Query().Get("kind"), r.URL.Query().Get("q"), embedSearchLimit)
	if err != nil {
		c.LogFailure(err, "searching embeds", "kind", r.URL.Query().Get("kind"))
		httputil.SendJSON(w, http.StatusInternalServerError, map[string]any{"error": "search failed"})
		return
	}
	if opts == nil {
		opts = []service.EmbedOption{}
	}
	httputil.SendJSON(w, http.StatusOK, map[string]any{"options": opts})
}

// Describe handles GET /api/embeds/describe?tag=<kind>:<name>&tag=…:
// {"items": {"<kind>:<name>": {...}}}, the window's preview and the
// cards; a tag of nothing (missing, unknown) is left out.
func (c *EmbedsController) Describe(w http.ResponseWriter, r *http.Request) {
	if !c.mayEmbed(r) {
		httputil.SendJSON(w, http.StatusForbidden, map[string]any{"error": "forbidden"})
		return
	}
	tags := r.URL.Query()["tag"]
	if len(tags) > embedDescribeLimit {
		tags = tags[:embedDescribeLimit]
	}
	items := map[string]service.EmbedOption{}
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		opt, ok, err := service.DescribeEmbed(r.Context(), tag)
		if err != nil {
			c.LogFailure(err, "describing an embed", "tag", tag)
			continue
		}
		if ok {
			items[tag] = opt
		}
	}
	httputil.SendJSON(w, http.StatusOK, map[string]any{"items": items})
}
