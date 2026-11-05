package entity

import (
	"context"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// PluginBlockID - a built-in block type whose content is rendered by Go and
// templates rather than edited as text (unlike BlockDef.Body). Region, weight
// and visibility work the same as for custom blocks.
type PluginBlockID string

// The core's plugin blocks (corePluginBlocks).
const (
	// PluginBlockLogo - the site logo (SiteInformation.Logo); none uploaded -
	// nothing.
	PluginBlockLogo         PluginBlockID = "logo"
	PluginBlockSearchForm   PluginBlockID = "search_form"
	PluginBlockUserMenu     PluginBlockID = "user_menu"
	PluginBlockLastComments PluginBlockID = "last_comments"
	// PluginBlockTagCloud - the most used terms (TagCloudTerm), the
	// more materials a term is on, the bigger (its settings:
	// TagCloudVocabularyKey, TagCloudCountKey, TagCloudLevelsKey).
	PluginBlockTagCloud PluginBlockID = "tag_cloud"
	// PluginBlockCopyright - the copyright line: "© <year> <holder>", the
	// current year, from a first year when set (CopyrightSinceKey), the
	// holder the site's name unless set (CopyrightHolderKey).
	PluginBlockCopyright PluginBlockID = "copyright"
)

// The copyright block's settings (PluginConfig keys).
const (
	CopyrightHolderKey = "holder"
	CopyrightSinceKey  = "since"
)

// corePluginBlocks - the core's plugin blocks with their admin labels, in
// the picker's order.
var corePluginBlocks = []ModulePluginBlock{
	{ID: PluginBlockLogo, Label: i18n.N("Logo")},
	{ID: PluginBlockSearchForm, Label: i18n.N("Search the site")},
	{ID: PluginBlockUserMenu, Label: i18n.N("User menu")},
	{ID: PluginBlockLastComments, Label: i18n.N("Recent comments")},
	{ID: PluginBlockTagCloud, Label: i18n.N("Tag cloud")},
	{ID: PluginBlockCopyright, Label: i18n.N("Copyright")},
}

// PluginBlocks - plugin blocks offered by the admin "Add system block"
// picker: the core's, then the registered modules'.
func PluginBlocks() []PluginBlockID {
	out := make([]PluginBlockID, 0, len(corePluginBlocks))
	for _, b := range corePluginBlocks {
		out = append(out, b.ID)
	}
	for _, m := range Modules {
		for _, b := range m.PluginBlocks {
			out = append(out, b.ID)
		}
	}
	return out
}

// PluginBlockLabels - the admin label (translation source) of each plugin
// block, the modules' included.
func PluginBlockLabels() map[PluginBlockID]string {
	out := make(map[PluginBlockID]string, len(corePluginBlocks))
	for _, b := range corePluginBlocks {
		out[b.ID] = b.Label
	}
	for _, m := range Modules {
		for _, b := range m.PluginBlocks {
			out[b.ID] = b.Label
		}
	}
	return out
}

// BlockPlacementType - what a BlockPlacementDef references: a BlockDef
// (custom), a PluginBlockID (plugin), a view display or a menu.
type BlockPlacementType string

// The kinds of block placements (BlockPlacementDef's type).
const (
	BlockPlacementCustom BlockPlacementType = "custom"
	BlockPlacementPlugin BlockPlacementType = "plugin"
	// BlockPlacementView - a View's "Block" display placed like any other block.
	BlockPlacementView BlockPlacementType = "view"
	// BlockPlacementMenu - a MenuDef placed as a block; every menu is placeable
	// without an extra "expose as block" step.
	BlockPlacementMenu BlockPlacementType = "menu"
)

// BlockVisibilityScope - which pages a block placement is shown on.
type BlockVisibilityScope string

// The block placements' visibility scopes: every page, the home page, the
// pages of the chosen content types, or a path list.
const (
	BlockVisibilityAll         BlockVisibilityScope = "all"
	BlockVisibilityHomepage    BlockVisibilityScope = "homepage"
	BlockVisibilityContentType BlockVisibilityScope = "content_type"
	// BlockVisibilityPages - a path list (VisibilityPages, one pattern per line);
	// VisibilityPagesMode decides whether it is an allowlist or a blocklist.
	// See service.MatchesVisibilityPages.
	BlockVisibilityPages BlockVisibilityScope = "pages"
)

// BlockVisibilityPagesMode - see BlockVisibilityPages.
type BlockVisibilityPagesMode string

const (
	// BlockVisibilityPagesShow - visible only on pages matching VisibilityPages.
	BlockVisibilityPagesShow BlockVisibilityPagesMode = "show"
	// BlockVisibilityPagesHide - visible everywhere except pages matching VisibilityPages.
	BlockVisibilityPagesHide BlockVisibilityPagesMode = "hide"
)

// BlockDef - a custom block: rich text the administrator edits without a
// deploy. Body is sanitized on save (htmlsanitize.SanitizeAdmin by default,
// or the format chosen in TextFormatID).
type BlockDef struct {
	ID    int
	Title string
	// MachineName - what a text or a template embeds the block by
	// ([[block:<name>]]): unique, [a-z][a-z0-9_]*; a new block without one
	// gets one from its title.
	MachineName string
	// ShowTitle - whether Title renders as an <h2> above the block. Title itself
	// is always required, so this is the only way to hide the heading.
	ShowTitle bool
	Body      string
	// TextFormatID - nil means Body is sanitized with htmlsanitize.SanitizeAdmin.
	// When set, the format's AllowedTags apply (htmlsanitize.SanitizeWithFormat),
	// e.g. to allow a raw <script> for an ad block. Not a FK: a deleted format
	// falls back to SanitizeAdmin on the next save.
	TextFormatID *int
	Status       bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// TextFormatIDValue returns TextFormatID dereferenced (0 = unset).
func (b BlockDef) TextFormatIDValue() int {
	if b.TextFormatID == nil {
		return 0
	}
	return *b.TextFormatID
}

// BlockPlacementDef - where a block is shown: theme region, order within the
// region (Weight) and visibility. Kept separate from BlockDef.
type BlockPlacementDef struct {
	ID     int
	Theme  string
	Region Region
	Weight int
	// PluginConfig - a plugin block's settings (key → value), e.g. the
	// "target" of a block that shows one thing (PluginBlockTargets); nil
	// on an update - kept as stored.
	PluginConfig map[string]string
	// PlacementType - exactly one of BlockID (custom), PluginID (plugin),
	// ViewDisplayID (view) or MenuID (menu) is set.
	PlacementType BlockPlacementType
	BlockID       *int
	PluginID      *PluginBlockID
	// ViewDisplayID - set only for BlockPlacementView; FK to a ViewDisplayDef.
	ViewDisplayID *int
	// MenuID - set only for BlockPlacementMenu; FK to a MenuDef.
	MenuID *int
	// MenuStartLevel - initial visibility level (1-10): 1 renders the menu tree
	// from the top; N>1 renders from the current page's active trail N-1 levels
	// down, and is empty when the page is not under that branch (see
	// service.ResolveMenuBlockRoot). Used only by menu placements.
	MenuStartLevel int
	// MenuLevelsToShow - number of levels to display (0-9); 0 means unlimited.
	MenuLevelsToShow int
	// Title/ShowTitle - the placement's own title, used by plugin placements
	// (custom blocks use BlockDef.Title; views and menus use their own labels).
	Title                 string
	ShowTitle             bool
	VisibilityScope       BlockVisibilityScope
	VisibilityContentType *string
	// VisibilityPages/VisibilityPagesMode - used only when
	// VisibilityScope==BlockVisibilityPages.
	VisibilityPages     string
	VisibilityPagesMode BlockVisibilityPagesMode
	// VisibilityRoles - role names the placement is shown to (comma-
	// separated, RoleAnonymous for guests), combined with the scope;
	// empty - everyone.
	VisibilityRoles string
	// Lazy - the page gets a placeholder (the theme's
	// block_placeholder_<region>) and the content is loaded when it
	// scrolls near the viewport (GET /api/block-placement/{id}).
	Lazy      bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

// ClampMenuLevelSettings clamps menu placement settings to their supported
// ranges (startLevel 1-10, levelsToShow 0-9, 0 = unlimited).
func ClampMenuLevelSettings(startLevel, levelsToShow int) (int, int) {
	if startLevel < 1 {
		startLevel = 1
	}
	if startLevel > 10 {
		startLevel = 10
	}
	if levelsToShow < 0 {
		levelsToShow = 0
	}
	if levelsToShow > 9 {
		levelsToShow = 9
	}
	return startLevel, levelsToShow
}

// VisibilityContentTypeValue returns VisibilityContentType dereferenced
// ("" = unset); templates can't compare a *string with a string.
func (p BlockPlacementDef) VisibilityContentTypeValue() string {
	if p.VisibilityContentType == nil {
		return ""
	}
	return *p.VisibilityContentType
}

// VisibilityContentTypes splits VisibilityContentType (a comma-separated
// list of content type machine names).
func (p BlockPlacementDef) VisibilityContentTypes() []string {
	var out []string
	for _, t := range strings.Split(p.VisibilityContentTypeValue(), ",") {
		if t = strings.TrimSpace(t); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// MatchesContentType reports whether contentType is in
// VisibilityContentTypes; used for BlockVisibilityContentType.
func (p BlockPlacementDef) MatchesContentType(contentType string) bool {
	for _, t := range p.VisibilityContentTypes() {
		if t == contentType {
			return true
		}
	}
	return false
}

// PluginIDValue returns PluginID dereferenced ("" = unset).
func (p BlockPlacementDef) PluginIDValue() PluginBlockID {
	if p.PluginID == nil {
		return ""
	}
	return *p.PluginID
}

// IsPlugin reports whether this is a plugin placement.
func (p BlockPlacementDef) IsPlugin() bool {
	return p.PlacementType == BlockPlacementPlugin
}

// ViewDisplayIDValue returns ViewDisplayID dereferenced (0 = unset).
func (p BlockPlacementDef) ViewDisplayIDValue() int {
	if p.ViewDisplayID == nil {
		return 0
	}
	return *p.ViewDisplayID
}

// IsView reports whether this is a View placement.
func (p BlockPlacementDef) IsView() bool {
	return p.PlacementType == BlockPlacementView
}

// MenuIDValue returns MenuID dereferenced (0 = unset).
func (p BlockPlacementDef) MenuIDValue() int {
	if p.MenuID == nil {
		return 0
	}
	return *p.MenuID
}

// IsMenu reports whether this is a menu placement.
func (p BlockPlacementDef) IsMenu() bool {
	return p.PlacementType == BlockPlacementMenu
}

// BlockWithPlacement - a BlockDef with its placement, as used by the admin
// list and region rendering. Block is zero for non-custom placements.
type BlockWithPlacement struct {
	Block     BlockDef
	Placement BlockPlacementDef
	// ViewDisplay - the referenced display, eager-loaded; set only when Placement.IsView().
	ViewDisplay *ViewDisplayDef
	// Menu - the referenced menu, eager-loaded; set only when Placement.IsMenu().
	Menu *MenuDef
	// PluginData - a plugin block's data for its template, loaded by the
	// cache refresh from its settings (service.SetPluginBlockData); nil
	// for a block without one.
	PluginData any
}

// BlockRepository - storage port for blocks and their placements.
type BlockRepository interface {
	// Blocks - every placement of one theme (with its block, View or menu).
	Blocks(ctx context.Context, theme string) ([]BlockWithPlacement, error)
	BlockByID(ctx context.Context, id int) (*BlockDef, error)
	// BlockByMachineName - the block named name (BlockDef.MachineName);
	// MachineNameTaken - whether a block other than excludeID has name.
	BlockByMachineName(ctx context.Context, name string) (*BlockDef, error)
	MachineNameTaken(ctx context.Context, name string, excludeID int) (bool, error)
	// SearchBlocks - the blocks whose title or machine name has query
	// ("" - any), by title, at most limit.
	SearchBlocks(ctx context.Context, query string, limit int) ([]BlockDef, error)
	// CreateBlock creates the block (named from its title when it has no
	// MachineName) and its placement.
	CreateBlock(ctx context.Context, b BlockDef, p BlockPlacementDef) (*BlockDef, error)
	UpdateBlock(ctx context.Context, b BlockDef) (*BlockDef, error)
	// DeleteBlock deletes a block together with its placements (cascaded in
	// application code, not by FK).
	DeleteBlock(ctx context.Context, id int) error

	// CreatePluginPlacement creates a plugin placement (no BlockDef row).
	CreatePluginPlacement(ctx context.Context, p BlockPlacementDef) (*BlockPlacementDef, error)
	// CreateViewPlacement creates a View placement (no BlockDef row).
	CreateViewPlacement(ctx context.Context, p BlockPlacementDef) (*BlockPlacementDef, error)
	// CreateMenuPlacement creates a menu placement (no BlockDef row).
	CreateMenuPlacement(ctx context.Context, p BlockPlacementDef) (*BlockPlacementDef, error)
	// DeletePlacement deletes any placement by its own ID without touching a
	// BlockDef; needed for placements that have no owning block.
	DeletePlacement(ctx context.Context, placementID int) error

	PlacementByBlockID(ctx context.Context, blockID int, theme string) (*BlockPlacementDef, error)
	ThemeHasPlacements(ctx context.Context, theme string) (bool, error)
	CopyPlacement(ctx context.Context, p BlockPlacementDef) (*BlockPlacementDef, error)
	// PlacementByID returns any placement by its own ID (PlacementByBlockID
	// only works for custom placements).
	PlacementByID(ctx context.Context, id int) (*BlockPlacementDef, error)
	UpdatePlacement(ctx context.Context, p BlockPlacementDef) (*BlockPlacementDef, error)
	// PublishedPlacementsByRegion returns published placements of one region,
	// ordered by Weight. Raw data for the cache: per-page visibility filtering
	// happens later, per request. A custom placement is published when
	// BlockDef.Status is set; other placements are published while they exist.
	PublishedPlacementsByRegion(ctx context.Context, theme string, region Region) ([]BlockWithPlacement, error)
	// ReorderPlacements bulk-updates Weight/Region after drag-and-drop.
	ReorderPlacements(ctx context.Context, updates []BlockPlacementOrderUpdate) error
}

// BlockPlacementOrderUpdate - one row of a bulk reorder
// (see BlockRepository.ReorderPlacements).
type BlockPlacementOrderUpdate struct {
	PlacementID int
	Region      Region
	Weight      int
}

// Roles - VisibilityRoles as a list.
func (p BlockPlacementDef) Roles() []string {
	var out []string
	for _, r := range strings.Split(p.VisibilityRoles, ",") {
		if r = strings.TrimSpace(r); r != "" {
			out = append(out, r)
		}
	}
	return out
}

// VisibleToRoles reports whether a visitor with these roles may see the
// placement: no roles condition, or one of the roles matches.
func (p BlockPlacementDef) VisibleToRoles(roles []string) bool {
	allowed := p.Roles()
	if len(allowed) == 0 {
		return true
	}
	for _, a := range allowed {
		for _, r := range roles {
			if a == r {
				return true
			}
		}
	}
	return false
}

// HasRole reports whether name is in the roles condition.
func (p BlockPlacementDef) HasRole(name string) bool {
	for _, r := range p.Roles() {
		if r == name {
			return true
		}
	}
	return false
}

// PluginSetting - a plugin block's setting ("" - unset).
func (p BlockPlacementDef) PluginSetting(key string) string {
	return p.PluginConfig[key]
}

// PluginBlockTargetKey - the PluginConfig key of the thing a block shows
// (PluginBlockTargets).
const PluginBlockTargetKey = "target"

// PluginBlockTarget - one thing a block can show (its PluginConfig
// "target"), as the placement page offers it.
type PluginBlockTarget struct {
	Value string
	Label string
}

// PluginBlockSettingType - how a plugin block's setting is entered.
type PluginBlockSettingType string

const (
	// PluginBlockSettingNumber - a whole number from Min to Max.
	PluginBlockSettingNumber PluginBlockSettingType = "number"
	// PluginBlockSettingSelect - one of Options.
	PluginBlockSettingSelect PluginBlockSettingType = "select"
	// PluginBlockSettingText - a line of text of at most Max characters
	// (empty allowed), no line breaks or control characters.
	PluginBlockSettingText PluginBlockSettingType = "text"
)

// PluginBlockSetting - one setting of a plugin block on its placement
// page, stored in its PluginConfig under Key (service.SetPluginBlockSettings).
// Label - the English label, translated by the page; Default - the value
// while unset.
type PluginBlockSetting struct {
	Key     string
	Label   string
	Type    PluginBlockSettingType
	Default string
	Min     int
	Max     int
	Options []PluginBlockTarget
}

// Check reports whether value is one the setting accepts.
func (s PluginBlockSetting) Check(value string) bool {
	switch s.Type {
	case PluginBlockSettingNumber:
		n, err := strconv.Atoi(value)
		return err == nil && n >= s.Min && n <= s.Max
	case PluginBlockSettingSelect:
		for _, o := range s.Options {
			if o.Value == value {
				return true
			}
		}
	case PluginBlockSettingText:
		if utf8.RuneCountInString(value) > s.Max {
			return false
		}
		for _, r := range value {
			if unicode.IsControl(r) {
				return false
			}
		}
		return true
	}
	return false
}

// PluginSettingOr - the placement's value of setting s, its Default while
// unset or no longer valid (an option gone, a number out of range).
func (p BlockPlacementDef) PluginSettingOr(s PluginBlockSetting) string {
	if v, ok := p.PluginConfig[s.Key]; ok && s.Check(v) {
		return v
	}
	return s.Default
}
