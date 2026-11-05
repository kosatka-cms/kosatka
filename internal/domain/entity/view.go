package entity

import (
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

// View - the container of ViewDisplayDefs: one "Default" display (never
// rendered, the shared baseline) plus any number of real displays, each
// inheriting or overriding Settings/Filters/Sort/Fields from Default per
// section. Label is admin-facing only. Created only by
// views.DefaultService.CloneDisplay.
type View struct {
	ID          int
	MachineName string
	Label       string
	CreatedBy   int
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// ViewBaseEntity - the base table a display's Filters/Sort/Fields/RunView
// target. Immutable once the display is created, like MachineName.
// "content" (nodes) is the zero value/default; "comment" lists comments
// via ViewQuery.RunCommentView - a configurable counterpart to the
// fixed PluginBlockLastComments block.
type ViewBaseEntity string

// The base entities a display lists: content (nodes), comments.
const (
	ViewBaseEntityContent ViewBaseEntity = "content"
	ViewBaseEntityComment ViewBaseEntity = "comment"
)

// IsComment reports whether the display lists comments (its
// ViewBaseEntity).
func (v ViewDisplayDef) IsComment() bool {
	return v.BaseEntity == ViewBaseEntityComment
}

// ViewDisplayDef - an admin-configured listing display (page, block,
// attachment, term page or profile tab) inside a View. Content type
// scoping is just a filter (ViewFilterBuiltinContentType), so a display
// without one spans every type. A display with nil ViewID is a "lone"
// display; ViewID/IsDefault/Override* matter only once
// views.DefaultService.CloneDisplay has grouped it into a View.
type ViewDisplayDef struct {
	ID int
	// BaseEntity - see ViewBaseEntity.
	BaseEntity ViewBaseEntity
	// ViewID - the owning View; nil for a display that has never been
	// cloned. Set by CloneDisplay.
	ViewID *int
	// IsDefault - true for the View's shared baseline display, which is
	// never rendered itself and is hidden from the admin views list.
	// Only views.DefaultService.CloneDisplay creates one.
	IsDefault bool
	// OverrideSettings/Filters/Sort/Fields - per-section "Override"/"Use
	// default" state. false = views.DefaultService's Effective* methods
	// resolve the section from the View's Default. Meaningless when
	// ViewID is nil or IsDefault is true.
	OverrideSettings bool
	OverrideFilters  bool
	OverrideSort     bool
	OverrideFields   bool
	MachineName      string
	Label            string
	// MetaTitle/MetaDescription - SEO fields for page displays. Empty
	// MetaTitle falls back to Label.
	MetaTitle       string
	MetaDescription string
	// URLPath - the page display's own path (no leading "/"), resolved
	// by a catch-all fallback. Unused for
	// non-page displays.
	URLPath string
	// DisplayType - see ViewDisplayType.
	DisplayType ViewDisplayType
	// AttachContentTypes/AttachWeight/AttachTitle/AttachShowTitle - for
	// attachment displays: comma-separated content types whose node page
	// shows this display between the article and its comments, ordered
	// by AttachWeight; AttachTitle is the public heading (empty = Label).
	// Structural placement: never inherited from Default nor snapshotted.
	AttachContentTypes string
	AttachWeight       int
	AttachTitle        string
	AttachShowTitle    bool
	// TermVocabularies - for term displays: comma-separated vocabulary
	// IDs whose term pages use this display (empty = every vocabulary).
	// Among several matches the lowest AttachWeight wins. Structural -
	// never inherited nor snapshotted.
	TermVocabularies string
	// ProfileTabPath/ProfileTabAccess - for user_tab displays: the tab's
	// path segment under a profile (/users/<alias>/<path>, /me/<path>)
	// and who sees it. The tab label is Label, its order AttachWeight.
	// Structural - never inherited nor snapshotted.
	ProfileTabPath   string
	ProfileTabAccess ViewProfileTabAccess
	// ThemeTemplate - a theme template (views/<ThemeTemplate>.html
	// defining "view_<ThemeTemplate>") that renders the list instead of
	// the generic display style. Empty = DisplayStyle. Structural -
	// never inherited nor snapshotted.
	ThemeTemplate string
	// DisplayStyle - grid (cards in N columns), list (one plain row per
	// item), table (columns from ViewFieldDefs) or carousel. Same values
	// for both base entities.
	DisplayStyle ViewDisplayStyle
	// GridColumns - column count for the grid style.
	GridColumns int
	// OverflowAfter - 0 renders every item in DisplayStyle. >0 renders
	// only the first OverflowAfter items that way and the rest as a
	// compact list below (title plus fields not excluded or marked
	// ViewFieldDef.HideInOverflow). Works with any style, so "N full
	// cards + a plain list of the rest" needs one display, not two.
	OverflowAfter int
	// CarouselSlidesPerView/Autoplay/Loop/ShowNavigation/ShowPagination -
	// carousel style settings, part of the Settings bundle.
	CarouselSlidesPerView  int
	CarouselAutoplay       bool
	CarouselLoop           bool
	CarouselShowNavigation bool
	CarouselShowPagination bool
	// CarouselLinkFieldStorageID - when set, each carousel slide is
	// wrapped in a link to this text field's value; nil = no wrapping
	// link. Restricted to FieldTypeText at the service layer.
	CarouselLinkFieldStorageID *int
	// CarouselEffect - Swiper transition: "slide" (default) or "fade".
	CarouselEffect string
	ShowPagination bool
	// PaginationAjax - with ShowPagination, pager links fetch the next
	// page and swap #view-list-container's content instead of navigating
	// (see initViewPagination in web/static/js/app.js). Default false:
	// plain ?page=N links that work without JS.
	PaginationAjax bool
	// ShowGlossary - renders a letter-index bar above the list. RunView
	// then synthesizes (never persists) a starts_with filter from
	// ?letter=X - see service.glossaryLetterFilter.
	ShowGlossary bool
	// GlossaryFieldStorageID - the text field the glossary groups by;
	// nil = the node title. Restricted to FieldTypeText at the service
	// layer (views.GlossaryFieldType).
	GlossaryFieldStorageID *int
	PageSize               int
	// HeaderText/FooterText - rich text rendered before/after the list.
	// Part of the Settings bundle. Sanitized on save, rendered unescaped
	// (trusted admin HTML).
	HeaderText string
	FooterText string
	// OverflowHeaderText - plain-text heading shown above the compact
	// overflow list (see OverflowAfter), only when that list is
	// non-empty. Empty = no heading.
	OverflowHeaderText string
	// CustomCSS - raw CSS rendered as an inline <style> on the display's
	// page. Not namespaced: #view-list-container is shared by every View
	// page, so admins must scope their own selectors.
	CustomCSS string
	// RowTemplate - row (card) markup for grid/list/carousel displays:
	// HTML with {{field:<machine_name>}} tokens (the field's rendered
	// output, empty when the item has no value) and {{url}} (the item's
	// path). Empty = fields rendered one after another. Settings bundle.
	RowTemplate string
	// GroupByField - machine name of one of the display's fields (field
	// storage or builtin like created_at); rows are grouped by its value,
	// one heading per group in order of first appearance
	// (service.groupViewRowsIn). Empty = no grouping.
	// GroupDateFormat - heading format for a date field: "" (the rendered
	// field) or ViewGroupDateYear/Month/Day. Settings bundle.
	GroupByField    string
	GroupDateFormat string
	// PublishedSnapshot/PublishedAt - nil PublishedAt = never published,
	// the page renders live data. Once set, PublishedSnapshot holds a
	// JSON ViewSnapshot (Settings bundle, Label/meta, Filters, Sorts,
	// Fields as of the last publish) that every visitor, admins included,
	// sees; the admin preview shows live state. URLPath/Status/
	// DisplayType/MachineName are not snapshotted and apply immediately
	// (ViewByURLPath resolves by the live column).
	PublishedSnapshot string
	PublishedAt       *time.Time
	Status            int // 1=enabled (resolves on the public side), 0=disabled
	CreatedBy         int
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// GlossaryFieldStorageIDValue - GlossaryFieldStorageID, 0 when nil (for
// templates).
func (v ViewDisplayDef) GlossaryFieldStorageIDValue() int {
	if v.GlossaryFieldStorageID == nil {
		return 0
	}
	return *v.GlossaryFieldStorageID
}

// CarouselLinkFieldStorageIDValue - CarouselLinkFieldStorageID, 0 when
// nil.
func (v ViewDisplayDef) CarouselLinkFieldStorageIDValue() int {
	if v.CarouselLinkFieldStorageID == nil {
		return 0
	}
	return *v.CarouselLinkFieldStorageID
}

// ViewIDValue - ViewID, 0 for a lone display.
func (v ViewDisplayDef) ViewIDValue() int {
	if v.ViewID == nil {
		return 0
	}
	return *v.ViewID
}

// HasGroup - whether the display belongs to a View; templates use it to
// show the display tabs and override banners.
func (v ViewDisplayDef) HasGroup() bool {
	return v.ViewID != nil
}

// HasPublishedSnapshot - whether the display has been published at
// least once (enables the "discard changes" action).
func (v ViewDisplayDef) HasPublishedSnapshot() bool {
	return v.PublishedAt != nil
}

// CanEditSettings (and CanEditFilters, CanEditSort, CanEditFields) -
// whether the section is the display's own (no group, the Default itself,
// or overridden) rather than inherited. Checked by the admin handlers as a
// defense-in-depth guard and by admin/views/page.html to choose controls
// vs. the "inherited" banner.
func (v ViewDisplayDef) CanEditSettings() bool {
	return v.IsDefault || v.ViewID == nil || v.OverrideSettings
}

// CanEditFilters - whether the filters are the display's own (see
// CanEditSettings).
func (v ViewDisplayDef) CanEditFilters() bool {
	return v.IsDefault || v.ViewID == nil || v.OverrideFilters
}

// CanEditSort - whether the sort is the display's own (see
// CanEditSettings).
func (v ViewDisplayDef) CanEditSort() bool { return v.IsDefault || v.ViewID == nil || v.OverrideSort }

// CanEditFields - whether the fields are the display's own (see
// CanEditSettings).
func (v ViewDisplayDef) CanEditFields() bool {
	return v.IsDefault || v.ViewID == nil || v.OverrideFields
}

// ViewDisplayDefs - a list of View displays.
type ViewDisplayDefs []ViewDisplayDef

// ViewDisplayType - where a display renders: "page" has its own URL
// (URLPath); "block" and the others have no URL and render inside other
// pages.
type ViewDisplayType string

// The display types (ViewDisplayType).
const (
	ViewDisplayTypePage  ViewDisplayType = "page"
	ViewDisplayTypeBlock ViewDisplayType = "block"
	// ViewDisplayTypeAttachment - attached to node detail pages of
	// AttachContentTypes (see ViewDisplayDef.AttachContentTypes).
	ViewDisplayTypeAttachment ViewDisplayType = "attachment"
	// ViewDisplayTypeTerm - lists a taxonomy term's materials on the
	// term's own page (see ViewDisplayDef.TermVocabularies); the term
	// comes from the URL as the has_current_term contextual filter.
	ViewDisplayTypeTerm ViewDisplayType = "term"
	// ViewDisplayTypeUserTab - a tab of the user profile listing that
	// user's materials (see ViewDisplayDef.ProfileTabPath); the user
	// comes from the URL as the created_by contextual filter.
	ViewDisplayTypeUserTab ViewDisplayType = "user_tab"
)

// ViewURLArgument - a URLPath segment taking a value from the request
// path (a "%" argument): the user with that alias, which the
// created_by contextual filter then uses ("User from URL"). At most one
// per path.
const ViewURLArgument = "%"

// MatchURLArgument - whether path matches pattern (a URLPath with one
// "%" segment) and the argument value.
func MatchURLArgument(pattern, path string) (string, bool) {
	ps := strings.Split(pattern, "/")
	xs := strings.Split(path, "/")
	if len(ps) != len(xs) {
		return "", false
	}
	arg, found := "", false
	for i := range ps {
		switch {
		case ps[i] == ViewURLArgument:
			if xs[i] == "" {
				return "", false
			}
			arg, found = xs[i], true
		case ps[i] != xs[i]:
			return "", false
		}
	}
	return arg, found
}

// ProfileBuiltinTabPaths - the user profile's own tab paths (the core's
// routes), which a user_tab display can't take.
var ProfileBuiltinTabPaths = []string{"edit", "files"}

// moduleProfileTabPaths - the modules' profile tab paths
// (ReserveProfileTabPath), which a user_tab display can't take either.
var (
	moduleProfileTabPathsMu sync.RWMutex
	moduleProfileTabPaths   = map[string]bool{}
)

// ReserveProfileTabPath - a module's profile tab has path (its wiring,
// interfaces.AddProfileTab): no user_tab display may take it.
func ReserveProfileTabPath(path string) {
	moduleProfileTabPathsMu.Lock()
	defer moduleProfileTabPathsMu.Unlock()
	moduleProfileTabPaths[path] = true
}

// ResetProfileTabPaths forgets the modules' tab paths (wiring the app
// again).
func ResetProfileTabPaths() {
	moduleProfileTabPathsMu.Lock()
	defer moduleProfileTabPathsMu.Unlock()
	moduleProfileTabPaths = map[string]bool{}
}

// ProfileTabPathReserved - whether path is a built-in tab's or a module's
// (a user_tab display can't take it).
func ProfileTabPathReserved(path string) bool {
	if slices.Contains(ProfileBuiltinTabPaths, path) {
		return true
	}
	moduleProfileTabPathsMu.RLock()
	defer moduleProfileTabPathsMu.RUnlock()
	return moduleProfileTabPaths[path]
}

// ViewProfileTabAccess - who sees a user_tab display's tab.
type ViewProfileTabAccess string

const (
	// ViewProfileTabAccessAll - every visitor.
	ViewProfileTabAccessAll ViewProfileTabAccess = "all"
	// ViewProfileTabAccessOwner - the profile's owner and admins.
	ViewProfileTabAccessOwner ViewProfileTabAccess = "owner"
)

// HasURL - only a page display owns a URL path; block, attachment,
// term and user_tab displays are rendered inside other pages.
func (t ViewDisplayType) HasURL() bool {
	return t == ViewDisplayTypePage
}

// AttachContentTypeList - AttachContentTypes split into machine names.
func (v ViewDisplayDef) AttachContentTypeList() []string {
	var out []string
	for _, t := range strings.Split(v.AttachContentTypes, ",") {
		if t = strings.TrimSpace(t); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// TermVocabularyIDs - TermVocabularies parsed into IDs (invalid
// entries skipped).
func (v ViewDisplayDef) TermVocabularyIDs() []int {
	var out []int
	for _, s := range strings.Split(v.TermVocabularies, ",") {
		if id, err := strconv.Atoi(strings.TrimSpace(s)); err == nil && id > 0 {
			out = append(out, id)
		}
	}
	return out
}

// MatchesVocabulary - whether a term display lists terms of vid
// (an empty TermVocabularies matches every vocabulary).
func (v ViewDisplayDef) MatchesVocabulary(vid int) bool {
	ids := v.TermVocabularyIDs()
	return len(ids) == 0 || slices.Contains(ids, vid)
}

// MatchesContentType - whether an attachment display is attached to
// contentType (same comparison as BlockPlacementDef.MatchesContentType,
// so the shared visibility_content_types checkbox partial works for
// both).
func (v ViewDisplayDef) MatchesContentType(contentType string) bool {
	return slices.Contains(v.AttachContentTypeList(), contentType)
}

// ViewDisplayStyle - see ViewDisplayDef.DisplayStyle's doc comment.
type ViewDisplayStyle string

// The display styles: a grid of cards, a list, a table, a carousel.
const (
	ViewDisplayStyleGrid  ViewDisplayStyle = "grid"
	ViewDisplayStyleList  ViewDisplayStyle = "list"
	ViewDisplayStyleTable ViewDisplayStyle = "table"
	// ViewDisplayStyleCarousel - a Swiper carousel, one slide per item;
	// configured by ViewDisplayDef.Carousel*.
	ViewDisplayStyleCarousel ViewDisplayStyle = "carousel"
)
