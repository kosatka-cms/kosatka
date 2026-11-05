package entity

import (
	"context"
	"strings"
	"time"
)

// TextFormatDef - an admin-editable text format: a named set of allowed HTML
// tags plus a TinyMCE toolbar config. The format is chosen per value on each
// save rather than fixed on the field definition, since the same field may be
// edited by both regular users and admins.
//
// Deliberate simplification: the allowlist is a flat tag list (AllowedTags)
// plus one AllowClassAttribute toggle, not per-tag attribute allowlists.
// Access is granted via role permissions (see AuthService.CanUseTextFormat).
type TextFormatDef struct {
	ID          int
	MachineName string
	Label       string
	// Weight - order in the format dropdown (lower first) and fallback priority:
	// when a value has no selected format, or the selected one is no longer
	// allowed for the user, the lowest-weight format allowed for the field wins
	// (see resolveTextFormat). Give the richest format the lowest weight,
	// otherwise rich content falls back to a plain textarea.
	Weight int
	// AllowedTags - space-separated HTML tags SanitizeWithFormat (pkg/htmlsanitize)
	// keeps. Empty means plain text: all markup is stripped. "script"/"style" keep
	// their raw content unescaped (see AllowUnsafeContent). "iframe" allows embeds
	// with https src and player attributes only. The <!--break--> marker is kept
	// in every format except plain text.
	AllowedTags string
	// AllowClassAttribute - allow the class attribute on kept tags (a per-format
	// equivalent of htmlsanitize.SanitizeAdmin's AllowStyling()).
	AllowClassAttribute bool
	// HasRichEditor - whether the format has a visual editor (TinyMCE). Fields
	// using formats always render a plain textarea; JS attaches or detaches
	// TinyMCE based on the currently selected format (node/edit_scripts.html).
	HasRichEditor bool
	// TinyMCEPlugins/TinyMCEToolbar - editor config for this format, applied by JS
	// when the format is selected. Empty means the default toolbar is kept.
	TinyMCEPlugins string
	TinyMCEToolbar string
	// AllowedExtensions/MaxFileSizeBytes - TinyMCE file manager upload policy for
	// this format, independent of site-wide UploadSettings. Empty/0 means unset;
	// see ResolveTextFormatUploadPolicy for the fallback.
	AllowedExtensions string
	MaxFileSizeBytes  int
	// Status - 1 = selectable for new values, 0 = hidden. Existing values keep
	// rendering and editing with it.
	Status int
	// Filters - the modules' text filters on for the format (their names,
	// space-separated; service.RegisterTextFilter): they process a value of
	// the format when it's shown, the stored HTML stays as saved.
	Filters   string
	CreatedBy int
	CreatedAt time.Time
	UpdatedAt time.Time
}

// TextFormatDefs - a list of text formats.
type TextFormatDefs []TextFormatDef

// AllowUnsafeContent reports whether AllowedTags includes "script" or "style"
// (raw-content passthrough in SanitizeWithFormat). AuthService.CanUseTextFormat
// requires full_access for such formats regardless of role permissions: this
// is the security boundary, not optional hardening.
func (f TextFormatDef) AllowUnsafeContent() bool {
	for _, t := range strings.Fields(strings.ToLower(f.AllowedTags)) {
		if t == "script" || t == "style" {
			return true
		}
	}
	return false
}

// FilterNames - Filters as a list.
func (f TextFormatDef) FilterNames() []string {
	return strings.Fields(f.Filters)
}

// AllowedExtensionsList parses AllowedExtensions for
// ResolveTextFormatUploadPolicy.
func (f TextFormatDef) AllowedExtensionsList() []string {
	return strings.Fields(f.AllowedExtensions)
}

// MaxFileSizeMB - MaxFileSizeBytes in MB, for the admin form's
// megabyte-denominated input. 0 when unset.
func (f TextFormatDef) MaxFileSizeMB() float64 {
	return float64(f.MaxFileSizeBytes) / 1024 / 1024
}

// TextFormatRepository - storage port for text formats.
type TextFormatRepository interface {
	TextFormats(ctx context.Context) (TextFormatDefs, error)
	TextFormatByID(ctx context.Context, id int) (*TextFormatDef, error)
	TextFormatByMachineName(ctx context.Context, machineName string) (*TextFormatDef, error)
	CreateTextFormat(ctx context.Context, f TextFormatDef) (*TextFormatDef, error)
	UpdateTextFormat(ctx context.Context, f TextFormatDef) (*TextFormatDef, error)
	DeleteTextFormat(ctx context.Context, id int) error
}
