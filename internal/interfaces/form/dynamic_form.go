package form

import (
	"context"
	"fmt"
	"maps"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
	"github.com/kosatka-cms/kosatka/pkg/sitetime"
)

// DynamicForm is the form for admin-defined content types. FieldValues holds
// one raw submitted string per field, keyed by FieldInstanceDef.ID; typed
// conversion happens in DynamicContentController, which has the resolved
// field list. Validate lives here to reuse the shared validation primitives.
type DynamicForm struct {
	NodeMetaFields
	NodeID int
	Title  string
	Slug   string
	// PathAuto - "generate the URL alias automatically" (path_auto);
	// PathAutoShown - the form had that checkbox (path_auto_shown: the
	// URL settings are shown only to who may change them), else the node
	// keeps its own setting.
	PathAuto      bool
	PathAutoShown bool
	// Promote/Sticky - "on the front page", "at the top of lists";
	// NodeFlagsShown - the form had them (node_flags_shown) and its user
	// may set them (DynamicContentController checks), else the node keeps
	// its own (a new one: its type's defaults).
	Promote        bool
	Sticky         bool
	NodeFlagsShown bool
	// Status/OnModeration - published, on moderation; taken from the form
	// only when SettingsAllowed.
	Status       int
	OnModeration int
	// SettingsAllowed - the editor may administer content, so the
	// publishing (Status, OnModeration), the author and the date come
	// from the form; set by DynamicContentController, never by the
	// request. Without it a new node gets its type's DefaultPublishing,
	// an edited one keeps its own.
	SettingsAllowed bool
	// WorkflowManaged - a module's workflow publishes the type's items
	// (interfaces.NodeWorkflow): the publishing and the schedule aren't
	// the form's; set by DynamicContentController, never by the request.
	WorkflowManaged bool
	FieldValues     map[int]string
	// FieldLinks - the link fields' links (FieldTypeLink, ParseLinkRows;
	// a hidden field's carried forward), keyed by FieldInstanceDef.ID.
	FieldLinks map[int][]entity.Link
	// FieldDates - the values of the date fields with a time, an end or
	// several dates (DateRich, DateList; DateValueOf, a hidden field's
	// carried forward; one value - one item) and FieldDateErrors what's
	// wrong with their inputs, keyed by FieldInstanceDef.ID; DateInputs -
	// those inputs as sent (the form shown again).
	FieldDates      map[int][]entity.DateValue
	FieldDateErrors map[int]string
	DateInputs      map[int][]DateInput
	// TextFormatValues - validated per-field text format choice, keyed by
	// FieldInstanceDef.ID; set by resolveTextFormat
	// only for fields with AllowedTextFormatIDs().
	TextFormatValues map[int]entity.TextFormatDef
	// CreatedByID/CreatedAt - author and date overrides (the edit form,
	// SettingsAllowed only). Zero value means "not submitted"; the
	// existing node's values are kept.
	CreatedByID int
	CreatedAt   time.Time
	// ScheduleShown - the form had the schedule (SettingsAllowed, a
	// content type's node): PublishOn / UnpublishOn are the node's new
	// ones, zero - none. Without it the node keeps its own.
	ScheduleShown bool
	PublishOn     time.Time
	UnpublishOn   time.Time
	// ViewRoles - who may view the node, of those its type is for
	// (entity.Node.ViewRoles); ViewRolesShown - the form had them
	// (SettingsAllowed, a content type's node), else the node keeps its own.
	ViewRoles      []string
	ViewRolesShown bool
	// CommentMode - the node's comment visibility (see entity.CommentMode).
	// Empty means "not submitted"; on create the content type's
	// DefaultCommentMode applies.
	CommentMode string
	// Submitted - the whole submitted form (nil - not submitted): the
	// modules' sections of a redisplayed form show what was sent
	// (interfaces.NodeSettings).
	Submitted url.Values
	// FieldUploadErrors - image upload validation failures (extensions, size,
	// resolution; see entity.FieldUploadPolicy) set by
	// resolveFileUploads, reported by Validate
	// so they render inline on the form.
	FieldUploadErrors map[int]string
	// FileAlts - the alt texts sent for the images, by file ID: written
	// with the content (not while the form is read: it may come back with
	// errors), shown again on a redisplayed form.
	FileAlts map[int]string
	// SentFiles, FileDeletes - a form sent back with errors (the node
	// controller's sentBack): an image or file field's file sent and not
	// saved yet, which the next submit keeps (<field>_sent, checked as a
	// pick from the media library), by field ID; the fields whose saved
	// file was ticked for deleting (the tick shown again).
	SentFiles   map[int]int
	FileDeletes map[int]bool
	// FieldBlocks - the blocks fields' items as submitted (a hidden
	// field's - the node's), keyed by FieldInstanceDef.ID; BlockErrors -
	// what's wrong with their values, by input name (Validate reports
	// them). Set by resolveBlocks (keepBlocks - a hidden field's).
	FieldBlocks map[int][]BlockInput
	BlockErrors map[string]string
}

// BlockInput - a block of a blocks field as submitted: its key in the
// form (its inputs' names' part), its item (its values resolved) and its
// own form (shown again: its links and dates as sent).
type BlockInput struct {
	Key  string
	Item entity.BlockItem
	Form *DynamicForm
}

// roleNameRe - a role's machine name, as a view role is checked.
var roleNameRe = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// ParseDynamicForm reads the generic node-level fields (title/slug/status/
// tags/meta) plus one "field_<id>" value per id in fieldIDs.
func (f *DynamicForm) ParseDynamicForm(r *http.Request, fieldIDs []int) (*DynamicForm, error) {
	f.Title = r.FormValue("title")
	f.Slug = r.FormValue("slug")
	f.PathAutoShown = r.FormValue("path_auto_shown") == "1"
	f.PathAuto = r.FormValue("path_auto") == "1"
	f.NodeFlagsShown = r.FormValue("node_flags_shown") == "1"
	f.Promote = r.FormValue("promote") == "1"
	f.Sticky = r.FormValue("sticky") == "1"

	f.Status = 0
	f.OnModeration = 0
	if statusStr := r.FormValue("status"); statusStr != "" {
		if status, err := strconv.Atoi(statusStr); err == nil {
			f.Status = status
		}
	}
	if modStr := r.FormValue("on_moderation"); modStr != "" {
		if mod, err := strconv.Atoi(modStr); err == nil {
			f.OnModeration = mod
		}
	}
	f.CommentMode = r.FormValue("comment_mode")
	f.Submitted = r.Form

	// Admin-only overrides (edit form only).
	if createdByStr := r.FormValue("created_by"); createdByStr != "" {
		if id, err := strconv.Atoi(createdByStr); err == nil && id > 0 {
			f.CreatedByID = id
		}
	}
	if createdAtStr := r.FormValue("created_at"); createdAtStr != "" {
		// datetime-local input, interpreted in the request's time zone
		// (the user's or the site's, see pkg/sitetime).
		if parsed, err := time.ParseInLocation("2006-01-02T15:04", createdAtStr, sitetime.FromContext(r.Context())); err == nil {
			f.CreatedAt = parsed
		}
	}

	f.ScheduleShown = r.FormValue("schedule_shown") == "1"
	if f.ScheduleShown {
		f.PublishOn = parseFormTime(r, "publish_on")
		f.UnpublishOn = parseFormTime(r, "unpublish_on")
	}
	f.ViewRolesShown = r.FormValue("view_roles_shown") == "1"
	f.ViewRoles = nil
	if f.ViewRolesShown {
		for _, role := range r.Form["view_roles"] {
			if roleNameRe.MatchString(role) && !slices.Contains(f.ViewRoles, role) {
				f.ViewRoles = append(f.ViewRoles, role)
			}
		}
	}

	f.FieldValues = make(map[int]string, len(fieldIDs))
	for _, id := range fieldIDs {
		f.FieldValues[id] = r.FormValue(fmt.Sprintf("field_%d", id))
	}

	f.ParseNodeMeta(r)
	return f, nil
}

// parseFormTime - the datetime-local input name in the request's time
// zone (the user's or the site's, see pkg/sitetime); zero when empty or
// malformed.
func parseFormTime(r *http.Request, name string) time.Time {
	t, err := time.ParseInLocation("2006-01-02T15:04", r.FormValue(name), sitetime.FromContext(r.Context()))
	if err != nil {
		return time.Time{}
	}
	return t
}

// Validate checks the generic node fields (title required, slug rules) plus
// each field per its FieldType/Required/MaxLen. number/taxonomy/reference/image
// values get a structural parse check since they go into typed value columns.
func (f *DynamicForm) Validate(ctx context.Context, fields entity.FieldInstanceDefs) map[string]string {
	errors := make(map[string]string)

	if msg, failed := validateRequiredTrimmed(f.Title, i18n.T(ctx, "The title field is required")); failed {
		errors["title"] = msg
	}
	if msg, failed := validateSlug(ctx, f.Slug); failed {
		errors["slug"] = msg
	}
	if !f.PublishOn.IsZero() && !f.UnpublishOn.IsZero() && !f.UnpublishOn.After(f.PublishOn) {
		errors["unpublish_on"] = i18n.T(ctx, "The unpublishing must come after the publishing.")
	}
	maps.Copy(errors, f.ValidateFields(ctx, fields))
	maps.Copy(errors, f.BlockErrors)
	return errors
}

// ValidateFields checks the values of fields (Validate without the
// node's own ones: a block's).
func (f *DynamicForm) ValidateFields(ctx context.Context, fields entity.FieldInstanceDefs) map[string]string {
	errors := make(map[string]string)
	for _, field := range fields {
		raw := f.FieldValues[field.ID]
		key := fmt.Sprintf("field_%d", field.ID)
		check := fieldChecks[field.FieldType]
		if check.before != nil {
			if msg := check.before(ctx, f, field, raw); msg != "" {
				errors[key] = msg
				continue
			}
		}
		if check.own {
			continue
		}
		// A HiddenInForm field has no visible input, so a Required error could
		// not be shown or fixed; on update its existing value is carried forward.
		if field.Required && !field.HiddenInForm {
			if msg, failed := validateRequiredTrimmed(raw, i18n.T(ctx, "@field is required", "@field", field.Label)); failed {
				errors[key] = msg
				continue
			}
		}
		if check.after != nil {
			if msg := check.after(ctx, f, field, raw); msg != "" {
				errors[key] = msg
			}
		}
	}
	return errors
}

// fieldAllowedValuesContain reports whether value is one of the field's
// configured AllowedValues().
func fieldAllowedValuesContain(field entity.FieldInstanceDef, value string) bool {
	for _, av := range field.AllowedValues() {
		if av.Value == value {
			return true
		}
	}
	return false
}
