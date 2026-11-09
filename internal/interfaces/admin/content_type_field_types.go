package admin

import (
	"context"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/files"
	"github.com/kosatka-cms/kosatka/internal/interfaces"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// adminFieldType - a field type's settings on the field's page (their
// inputs - field_edit.html's field_settings_<type>): storage - the shared
// ones from the form into config (FieldStorageDef.Config; "multiple" is
// set by then, the create form's); instance - the per-type ones
// (FieldInstanceDef's WidgetConfig); check - what's wrong with them on
// saving the page (an error of the page and its detail; "" - none; it
// may complete config - the blocks' sets kept to the field sets); data -
// what the settings show besides the field (roles, field sets…). Every
// field type is here (TestFieldTypesHaveSettings).
type adminFieldType struct {
	storage  func(r *http.Request, config map[string]any)
	instance func(r *http.Request, config map[string]any)
	check    func(ctx context.Context, c *Controller, r *http.Request, field entity.FieldInstanceDef, config, widgetConfig map[string]any) (errKey, detail string, err error)
	data     func(ctx context.Context, c *Controller, data map[string]any)
}

var adminFieldTypes = map[entity.ContentTypeFieldType]adminFieldType{
	entity.FieldTypeText: {
		storage: func(r *http.Request, config map[string]any) {
			if maxLen, err := strconv.Atoi(r.FormValue("max_len")); err == nil && maxLen > 0 {
				config["max_len"] = maxLen
			}
		},
		instance: func(r *http.Request, config map[string]any) {
			// Unknown values are dropped; Widget() then falls back to WidgetInput.
			if widget := r.FormValue("widget"); isValidWidget(widget) {
				config["widget"] = widget
			}
			// allowed_text_formats - see entity.FieldInstanceDef.AllowedTextFormatIDs.
			if ids := formIDs(r, "allowed_text_formats"); len(ids) > 0 {
				config["allowed_text_formats"] = ids
			}
		},
	},
	entity.FieldTypeNumber: {storage: func(r *http.Request, config map[string]any) {
		// number_kind, decimals, min, max, prefix, suffix -
		// entity.FieldInstanceDef.NumberKind and the rest.
		switch k := entity.NumberKind(r.FormValue("number_kind")); k {
		case entity.NumberInteger:
			config["number_kind"] = string(k)
		case entity.NumberDecimal:
			config["number_kind"] = string(k)
			if d, err := strconv.Atoi(strings.TrimSpace(r.FormValue("decimals"))); err == nil && d >= 0 && d <= 6 {
				config["decimals"] = d
			}
		}
		for _, key := range []string{"min", "max"} {
			if v, err := strconv.ParseFloat(strings.TrimSpace(r.FormValue(key)), 64); err == nil {
				config[key] = v
			}
		}
		for _, key := range []string{"prefix", "suffix"} {
			if v := r.FormValue(key); strings.TrimSpace(v) != "" && utf8.RuneCountInString(v) <= 16 {
				config[key] = v
			}
		}
	}},
	entity.FieldTypeEmail: {storage: func(r *http.Request, config map[string]any) {
		// email_plain - the switch is "hide" and only the field's page has
		// it (email_hidden_shown): a new field's addresses are hidden.
		if r.FormValue("email_hidden_shown") == "1" && r.FormValue("email_hidden") != "1" {
			config["email_plain"] = true
		}
	}},
	entity.FieldTypePhone: {},
	entity.FieldTypeBool:  {},
	entity.FieldTypeDate: {
		storage: func(r *http.Request, config map[string]any) {
			// date_time, date_end - entity.FieldInstanceDef.DateTime, DateEnd.
			if p := entity.ParseDatePart(r.FormValue("date_time")); p != entity.DatePartNone {
				config["date_time"] = string(p)
			}
			if p := entity.ParseDatePart(r.FormValue("date_end")); p != entity.DatePartNone {
				config["date_end"] = string(p)
			}
		},
		instance: func(r *http.Request, config map[string]any) {
			// time_format, date_range_full - entity.FieldInstanceDef.TimeFormat,
			// DateRangeFull (the switch is "shorten"; only the field's page has
			// it, date_range_shown: a new field's periods are shortened).
			if tf := strings.TrimSpace(r.FormValue("time_format")); tf != "" {
				config["time_format"] = tf
			}
			if r.FormValue("date_range_shown") == "1" && r.FormValue("date_range_short") != "1" {
				config["date_range_full"] = true
			}
			maxValuesFromForm(r, config)
		},
	},
	entity.FieldTypeList: {
		storage: func(r *http.Request, config map[string]any) {
			// allowed_values - stored whenever submitted, regardless of widget.
			if values := parseAllowedValuesForm(r.FormValue("allowed_values")); len(values) > 0 {
				config["allowed_values"] = values
			}
		},
		instance: func(r *http.Request, config map[string]any) {
			// list_widget, not "widget": a distinct form name so one parser never
			// confuses the list and text widget values.
			if widget := r.FormValue("list_widget"); isValidListWidget(widget) {
				config["widget"] = widget
			}
		},
		check: func(_ context.Context, _ *Controller, r *http.Request, field entity.FieldInstanceDef, config, _ map[string]any) (string, string, error) {
			if config["allowed_values"] == nil {
				return "allowed-values-required", "", nil
			}
			// Radio only for a single value, checkboxes only for several.
			widget, multiple := r.FormValue("list_widget"), field.Multiple()
			if (widget == string(entity.WidgetRadio) && multiple) || (widget == string(entity.WidgetCheckboxes) && !multiple) {
				return "invalid-list-widget", "", nil
			}
			return "", "", nil
		},
	},
	entity.FieldTypeTaxonomy: {
		storage: func(r *http.Request, config map[string]any) {
			if vocID, err := strconv.Atoi(r.FormValue("vocabulary_id")); err == nil && vocID > 0 {
				config["vocabulary_id"] = vocID
			}
		},
		instance: func(r *http.Request, config map[string]any) {
			// taxonomy_widget - distinct form name, like list_widget.
			if widget := r.FormValue("taxonomy_widget"); isValidTaxonomyWidget(widget) {
				config["widget"] = widget
			}
		},
		// Checked here, not at creation: the field's page is where the
		// vocabulary and the widget are set.
		check: func(_ context.Context, _ *Controller, r *http.Request, field entity.FieldInstanceDef, config, _ map[string]any) (string, string, error) {
			if config["vocabulary_id"] == nil {
				return "vocabulary-required", "", nil
			}
			// Radio only for a single value; checkboxes and autocomplete only
			// for several (see entity.WidgetAutocomplete).
			widget, multiple := r.FormValue("taxonomy_widget"), field.Multiple()
			if (widget == string(entity.WidgetRadio) && multiple) ||
				((widget == string(entity.WidgetCheckboxes) || widget == string(entity.WidgetAutocomplete)) && !multiple) {
				return "invalid-taxonomy-widget", "", nil
			}
			return "", "", nil
		},
	},
	entity.FieldTypeReference: {storage: func(r *http.Request, config map[string]any) {
		// target_content_type - machine names of allowed target types,
		// comma-separated (empty = any type); used by the edit form's
		// search (see entity.FieldStorageDef.TargetContentType).
		if target := joinFormList(r, "target_content_type"); target != "" {
			config["target_content_type"] = target
		}
		// symmetric - a mutual reference (entity.FieldInstanceDef.Symmetric),
		// only together with several values.
		if r.FormValue("symmetric") == "1" && config["multiple"] == true {
			config["symmetric"] = true
		}
	}},
	// An image's - how many too (several images), and whether its files
	// are private (entity.FieldInstanceDef.PrivateFiles).
	entity.FieldTypeImage: {instance: func(r *http.Request, config map[string]any) {
		uploadSettingsFromForm(r, config)
		privateFilesFromForm(r, config)
		maxValuesFromForm(r, config)
	}, check: checkFileDirectory},
	// A file's - how many too (several files), and whether its files are
	// private.
	entity.FieldTypeFile: {instance: func(r *http.Request, config map[string]any) {
		uploadSettingsFromForm(r, config)
		privateFilesFromForm(r, config)
		maxValuesFromForm(r, config)
	}, check: checkFileDirectory},
	entity.FieldTypeLink: {
		storage: func(r *http.Request, config map[string]any) {
			// link_title, allowed_hosts - entity.FieldInstanceDef.LinkTitle,
			// AllowedHosts.
			if m := entity.LinkTitleMode(r.FormValue("link_title")); m != entity.LinkTitleNone && slices.Contains(entity.LinkTitleModes, m) {
				config["link_title"] = string(m)
			}
			if hosts := entity.ParseHosts(r.FormValue("allowed_hosts")); len(hosts) > 0 {
				hostsAny := make([]any, len(hosts))
				for i, h := range hosts {
					hostsAny[i] = h
				}
				config["allowed_hosts"] = hostsAny
			}
		},
		instance: maxValuesFromForm,
		data: func(ctx context.Context, _ *Controller, data map[string]any) {
			data["linkHostSuggestions"] = linkHostSuggestions(ctx)
		},
	},
	entity.FieldTypeUser: {
		storage: func(r *http.Request, config map[string]any) {
			// user_roles - entity.FieldInstanceDef.UserRoles.
			if roles := formIDs(r, "user_roles"); len(roles) > 0 {
				config["user_roles"] = roles
			}
		},
		instance: maxValuesFromForm,
		data: func(ctx context.Context, c *Controller, data map[string]any) {
			if roles, err := c.roleRepo.List(ctx); err != nil {
				c.LogFailure(err, "error listing roles for a user field's page")
			} else {
				data["roles"] = *roles
			}
		},
	},
	entity.FieldTypeBlocks: {
		storage: func(r *http.Request, config map[string]any) {
			// block_types - entity.FieldInstanceDef.BlockTypes; check keeps
			// only the field sets.
			var types []any
			for _, s := range r.Form["block_types"] {
				if s = strings.TrimSpace(s); s != "" && !slices.Contains(types, any(s)) {
					types = append(types, s)
				}
			}
			if len(types) > 0 {
				config["block_types"] = types
			}
		},
		instance: maxValuesFromForm,
		// Only field sets; at least one.
		check: func(ctx context.Context, c *Controller, _ *http.Request, _ entity.FieldInstanceDef, config, _ map[string]any) (string, string, error) {
			sets, err := c.contentTypeService.FieldSets(ctx)
			if err != nil {
				return "", "", err
			}
			var types []any
			for _, name := range (entity.FieldStorageDef{Config: config}).BlockTypes() {
				if slices.ContainsFunc(sets, func(s entity.ContentTypeDef) bool { return s.MachineName == name }) {
					types = append(types, name)
				}
			}
			if len(types) == 0 {
				return "block-types-required", "", nil
			}
			config["block_types"] = types
			return "", "", nil
		},
		data: func(ctx context.Context, c *Controller, data map[string]any) {
			if sets, err := c.contentTypeService.FieldSets(ctx); err != nil {
				c.LogFailure(err, "error listing the field sets for a blocks field's page")
			} else {
				data["fieldSets"] = sets
			}
		},
	},
	// A group's settings (its scope) are the page's own: usable_in_form,
	// usable_in_detail.
	entity.FieldTypeGroup: {},
}

// formIDs - the positive integers of the form's inputs name (several
// same-named), as config stores them.
func formIDs(r *http.Request, name string) []any {
	var ids []any
	for _, s := range r.Form[name] {
		if id, err := strconv.Atoi(strings.TrimSpace(s)); err == nil && id > 0 {
			ids = append(ids, id)
		}
	}
	return ids
}

// maxValuesFromForm - max_values (entity.FieldInstanceDef.MaxValues: of
// several dates, links, users, images, files, blocks).
func maxValuesFromForm(r *http.Request, config map[string]any) {
	if n, err := strconv.Atoi(strings.TrimSpace(r.FormValue("max_values"))); err == nil && n > 0 {
		config["max_values"] = n
	}
}

// uploadSettingsFromForm - an image's or a file's upload settings
// (entity.FieldUploadPolicy) over the site-wide policy (see
// entity.ResolveFieldUploadPolicy); empty keys are left out so the
// accessors fall back to the site-wide default.
func uploadSettingsFromForm(r *http.Request, config map[string]any) {
	if exts := interfaces.ParseExtensionsForm(r.FormValue("allowed_extensions")); len(exts) > 0 {
		extsAny := make([]any, len(exts))
		for i, e := range exts {
			extsAny[i] = e
		}
		config["allowed_extensions"] = extsAny
	}
	if mb, err := strconv.ParseFloat(strings.TrimSpace(r.FormValue("max_filesize_mb")), 64); err == nil && mb > 0 {
		config["max_filesize_bytes"] = int(mb * 1024 * 1024)
	}
	if w, h := parseResolutionForm(r.FormValue("max_resolution")); w > 0 && h > 0 {
		config["max_resolution_width"] = w
		config["max_resolution_height"] = h
	}
	if w, h := parseResolutionForm(r.FormValue("min_resolution")); w > 0 && h > 0 {
		config["min_resolution_width"] = w
		config["min_resolution_height"] = h
	}
	// file_directory - entity.FieldInstanceDef.FileDirectory; checked by
	// checkFileDirectory.
	if dir := strings.TrimSpace(r.FormValue("file_directory")); dir != "" {
		config["file_directory"] = dir
	}
}

// privateFilesFromForm - an image's or a file's "Private files" switch
// (entity.FieldInstanceDef.PrivateFiles) and where its files come from.
func privateFilesFromForm(r *http.Request, config map[string]any) {
	if r.FormValue("private_files") == "1" {
		config["private_files"] = true
	}
	// media_source - entity.FieldInstanceDef.MediaSource ("" - both).
	if s := entity.MediaSource(r.FormValue("media_source")); s == entity.MediaSourceUpload || s == entity.MediaSourceLibrary {
		config["media_source"] = string(s)
	}
}

// checkFileDirectory - an image's or a file's upload directory is one
// files.ValidateFileDirectory takes.
func checkFileDirectory(ctx context.Context, _ *Controller, _ *http.Request, _ entity.FieldInstanceDef, _, widgetConfig map[string]any) (string, string, error) {
	if dir, _ := widgetConfig["file_directory"].(string); dir != "" {
		if err := files.ValidateFileDirectory(dir); err != nil {
			return "file-directory", i18n.ErrorText(ctx, err), nil
		}
	}
	return "", "", nil
}
