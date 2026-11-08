package service

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"maps"
	"net/url"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/assets"
	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/imageurl"
	"github.com/kosatka-cms/kosatka/internal/domain/themes"
	"github.com/kosatka-cms/kosatka/internal/lib/dateformat"
	"github.com/kosatka-cms/kosatka/internal/lib/photoswipe"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
	"github.com/kosatka-cms/kosatka/pkg/sitetime"
)

// buildFuncMap returns the template helpers shared by every render path, with extra
// overriding/adding entries on top.
func (t *DefaultTemplateService) buildFuncMap(extra template.FuncMap) template.FuncMap {
	return t.buildFuncMapIn(extra, nil, "")
}

// buildFuncMapIn - buildFuncMap with the time funcs in zone (nil - the
// site's zone, read on every call).
func (t *DefaultTemplateService) buildFuncMapIn(extra template.FuncMap, zone *time.Location, lang string) template.FuncMap {
	return t.buildFuncMapFor(extra, zone, lang, themeScope{})
}

// buildFuncMapFor - buildFuncMapIn in a theme: its stylesheets (themeCSS)
// and its translations.
func (t *DefaultTemplateService) buildFuncMapFor(extra template.FuncMap, zone *time.Location, lang string, theme themeScope) template.FuncMap {
	c := funcScope{t: t, zone: zone, lang: lang, theme: theme}
	funcs := template.FuncMap{}
	for _, group := range []template.FuncMap{c.markupFuncs(), c.dateFuncs(), c.fieldFuncs(), c.languageFuncs(), c.pageFuncs(), generalFuncs} {
		maps.Copy(funcs, group)
	}
	maps.Copy(funcs, extra)
	return funcs
}

// funcScope - what the template funcs of one set read: the service, the
// zone (nil - the site's, read on every call), the language ("" - the
// site's default) and the theme.
type funcScope struct {
	t     *DefaultTemplateService
	zone  *time.Location
	lang  string
	theme themeScope
}

// setLang - the language the set translates to: lang, else the site's
// default (read on every call, see getOrParseIn).
func (c funcScope) setLang() string {
	if c.lang != "" {
		return c.lang
	}
	return i18n.Default()
}

// datePattern - the pattern of a catalog format ("" or unknown - the
// default one's; the date and time without a "datetime" format).
func (c funcScope) datePattern(name string) string {
	if name == entity.DateTimeFormat {
		if c.t.dateFormats == nil {
			return defaultDateTimeFormatPattern
		}
		if p, ok := c.t.dateFormats.Lookup(context.Background(), name); ok {
			return p
		}
		return defaultDateTimeFormatPattern
	}
	if c.t.dateFormats == nil {
		return defaultDateFormatPattern
	}
	return c.t.dateFormats.Pattern(context.Background(), name)
}

// inZone - v in the set's zone.
func (c funcScope) inZone(v time.Time) time.Time {
	if c.zone != nil {
		return v.In(c.zone)
	}
	return sitetime.In(v)
}

// fieldDate - a date field's value v as FieldDate: the date format
// name's pattern, the time format's (timeFormat; "" - hours and
// minutes), a range whole (full) or shortened; a value with a time in
// the page's zone.
func (c funcScope) fieldDate(v any, name, timeFormat string, full bool) FieldDate {
	d, _ := entity.AsDateValue(v)
	fd := FieldDate{Time: d.Start, End: d.End, WithTime: d.WithTime, Pattern: defaultDateFormatPattern,
		TimePattern: defaultTimeFormatPattern, Short: !full, Lang: c.lang}
	if c.t.dateFormats != nil {
		fd.Pattern = c.t.dateFormats.Pattern(context.Background(), name)
		if timeFormat != "" {
			if p, ok := c.t.dateFormats.Lookup(context.Background(), timeFormat); ok {
				fd.TimePattern = p
			}
		}
	}
	if d.WithTime {
		fd.Time = c.inZone(d.Start)
		if d.HasEnd() {
			fd.End = c.inZone(d.End)
		}
	}
	return fd
}

// fieldDates - fieldDatesFor: a date field's several dates by its
// settings and layout.
func (c funcScope) fieldDates(field entity.FieldInstanceDef, value any, layout any) FieldDates {
	loc := c.zone
	if loc == nil {
		loc = sitetime.Location()
	}
	now := time.Now()
	var all []FieldDate
	var upcoming []int
	for _, d := range entity.DateValues(value) {
		fd := c.fieldDate(d, field.DateFormat, field.TimeFormat(), field.DateRangeFull())
		fd.Past, fd.Ongoing = d.Ended(now, loc), d.Ongoing(now, loc)
		if !fd.Past {
			upcoming = append(upcoming, len(all))
		}
		all = append(all, fd)
	}
	if len(all) == 0 {
		return FieldDates{}
	}
	switch entity.ParseFieldLayout(fmt.Sprint(layout)) {
	case entity.FieldLayoutFirst:
		return FieldDates{Dates: all[:1]}
	case entity.FieldLayoutUpcoming, entity.FieldLayoutNearest:
		if len(upcoming) == 0 {
			return FieldDates{Dates: all[len(all)-1:]}
		}
		// The nearest - the earliest start of those not ended.
		slices.SortStableFunc(upcoming, func(a, b int) int { return all[a].Compare(all[b].Time) })
		if entity.ParseFieldLayout(fmt.Sprint(layout)) == entity.FieldLayoutNearest {
			return FieldDates{Dates: []FieldDate{all[upcoming[0]]}, More: len(upcoming) - 1}
		}
		out := make([]FieldDate, len(upcoming))
		for i, n := range upcoming {
			out[i] = all[n]
		}
		return FieldDates{Dates: out}
	}
	return FieldDates{Dates: all}
}

// markupFuncs - HTML out as is, escaped or rewritten, text formats'
// filters, excerpts, JSON.
func (c funcScope) markupFuncs() template.FuncMap {
	t, setLang := c.t, c.setLang
	return template.FuncMap{
		// rawHTML - s output as is, unescaped: only for markup sanitized
		// on save or written by an administrator.
		"rawHTML": func(s string) template.HTML { return template.HTML(s) },
		// safeCSS - a View's CustomCSS for <style>{{.customCSS | safeCSS}}</style>.
		// html/template escapes template.HTML inside <style>; template.CSS is
		// the type for trusted CSS.
		"safeCSS": func(s string) template.CSS { return template.CSS(s) },
		// renderRowTemplate - a View row template (ViewDisplayDef.RowTemplate):
		// {{url}} → the item's path, {{field:<machine_name>}} → that field's
		// rendered output (empty without a value). Plain string replacement,
		// never parsed as a Go template (the template is admin text).
		"renderRowTemplate": renderRowTemplate,
		// rewriteFieldValue - ViewFieldDef.RewriteHTML: replaces the literal
		// "{{value}}" token with the rendered field value and "{{field:<key>}}"
		// with sibling field values from fieldTokens (keyed by machine name,
		// builtins included). Plain strings.Replace on purpose: rewriteHTML is
		// admin-authored and must never be parsed as a Go template (template
		// injection). Inputs are already safe HTML; unknown tokens stay as-is.
		"rewriteFieldValue": func(rewriteHTML string, value template.HTML, fieldTokens map[string]template.HTML) template.HTML {
			if rewriteHTML == "" {
				return value
			}
			out := strings.ReplaceAll(rewriteHTML, "{{value}}", string(value))
			for key, tok := range fieldTokens {
				out = strings.ReplaceAll(out, "{{field:"+key+"}}", string(tok))
			}
			return template.HTML(out)
		},
		// newFieldTokenMap/setFieldToken build rewriteFieldValue's fieldTokens in
		// templates (dynamic/list.html fills one map per item from all its fields,
		// including excluded ones, before rendering). setFieldToken returns ""
		// so a call without "$_ :=" prints nothing.
		"newFieldTokenMap": func() map[string]template.HTML { return map[string]template.HTML{} },
		"setFieldToken": func(m map[string]template.HTML, key string, value template.HTML) string {
			m[key] = value
			return ""
		},
		"stripImages": func(s string) string {
			// Remove all <img> tags.
			re := regexp.MustCompile(`(?s)<img[^>]*>`)
			return re.ReplaceAllString(s, "")
		},
		// json - v as a JSON string, for an attribute (data-ids="…").
		// Inside <script> output the value itself ({{.}}): html/template
		// marshals it to JSON there, while this string would become a JS
		// string literal.
		"json": func(v any) string {
			b, err := json.Marshal(v)
			if err != nil {
				return "[]"
			}
			return string(b)
		},
		// truncate - getPreview output as HTML: s must be HTML sanitized on
		// save (a text format, comments, signatures); plain text goes
		// through truncateText or plainExcerpt (plainPreview is its other
		// name, the one teasers use), which are escaped.
		"truncate":     func(s string, limit int) template.HTML { return template.HTML(getPreview(s, limit)) },
		"truncateText": truncateText,
		"plainPreview": PlainExcerpt,
		"plainExcerpt": PlainExcerpt,
		// plainLead - the first paragraph of HTML as plain text (PlainLead).
		"plainLead": PlainLead,
		"processContent": func(s string) template.HTML {
			if t.contentProcessor != nil {
				return template.HTML(t.contentProcessor.ProcessImages(s))
			}
			return template.HTML(s)
		},
		// processText - processContent and then the modules' text filters
		// on for the value's format (formatID; 0 - none, ApplyTextFilters):
		// {{processText .Value .FormatID}}.
		"processText": func(s string, formatID int) template.HTML {
			if t.contentProcessor != nil {
				s = t.contentProcessor.ProcessImages(s)
			}
			return template.HTML(ApplyTextFilters(i18n.WithLang(context.Background(), setLang()), s, formatID))
		},
		// textFilters - only the format's text filters, for HTML shown
		// as is otherwise (a block's body): {{textFilters .Body .FormatID}}.
		"textFilters": func(s string, formatID int) template.HTML {
			return template.HTML(ApplyTextFilters(i18n.WithLang(context.Background(), setLang()), s, formatID))
		},
		// embed - what "<kind>:<name>" embeds ([[block:footer]] in a text):
		// {{embed "block:footer"}}; filled in on the page, as its visitor
		// sees it (EmbedHTML, ResolveEmbeds).
		"embed": EmbedHTML,
		// textFormatEditor - a format's editor with its filters' buttons
		// (EditorFor), for the format pickers' data-plugins/data-toolbar/
		// data-external-plugins.
		"textFormatEditor": EditorFor,
		"photoSwipeCaption": func(title, description string) string {
			return photoswipe.CaptionHTML(title, description)
		},
	}
}

// dateFuncs - dates by the catalog's formats in the page's zone, a
// date field's dates, the current time.
func (c funcScope) dateFuncs() template.FuncMap {
	t, setLang, datePattern, inZone, fieldDate := c.t, c.setLang, c.datePattern, c.inZone, c.fieldDate
	return template.FuncMap{
		// fieldDate - a date field's value (a time.Time, or an
		// entity.DateValue with a time or an end: its times in the page's
		// zone, hours and minutes, a range shortened) with a date format
		// machine name ("" - the default format); field_date_* templates
		// print .Formatted.
		"fieldDate": func(value any, formatName any) FieldDate {
			name, _ := formatName.(string)
			return fieldDate(value, name, "", false)
		},
		// fieldDatesFor - a date field's several dates (one, a plain
		// value, too) by its settings and layout (entity.FieldLayout or
		// its string): all in the editor's order (the ended marked Past,
		// the current Ongoing), the upcoming only (none - the last), the
		// nearest upcoming and how many more, or the first.
		"fieldDatesFor": c.fieldDates,
		// fieldDateFor - fieldDate by a field's own settings too: its time
		// format and whether a range stays whole; formatName "" - the
		// field's date format.
		"fieldDateFor": func(field entity.FieldInstanceDef, value any, formatName any) FieldDate {
			name, _ := formatName.(string)
			if name == "" {
				name = field.DateFormat
			}
			return fieldDate(value, name, field.TimeFormat(), field.DateRangeFull())
		},
		// date - a date (time.Time or *time.Time) in the site's zone by a
		// format of the catalog (/admin/config/date-formats): {{date .CreatedAt
		// ""}} - the default format, {{date .CreatedAt "datetime"}}; "" for
		// no date.
		"date": func(value any, formatName string) string {
			var d time.Time
			switch v := value.(type) {
			case time.Time:
				d = v
			case *time.Time:
				if v != nil {
					d = *v
				}
			}
			if d.IsZero() {
				return ""
			}
			return dateformat.Format(inZone(d), datePattern(formatName), setLang())
		},
		// viewDateFormat - the format of a date in a View row: the
		// Views field's own (Config "date_format"), else the format of
		// the field on the row's content type, else "" (the default).
		"viewDateFormat": func(field entity.FieldInstanceDef, item any) string {
			if f, _ := field.Config["date_format"].(string); f != "" {
				return f
			}
			if t.dateFormats == nil {
				return ""
			}
			typ, ok := viewRowNodeType(item)
			if !ok {
				return ""
			}
			return t.dateFormats.InstanceFormat(context.Background(), typ, field.FieldStorageID)
		},
		// viewDateValue - a View row's date value for display: a builtin
		// timestamp (created_at, Config "timestamp") in the request's
		// zone; a date field (date only) as stored.
		"viewDateValue": func(field entity.FieldInstanceDef, value any) any {
			if ts, _ := field.Config["timestamp"].(bool); ts {
				if v, ok := value.(time.Time); ok {
					return inZone(v)
				}
			}
			return value
		},
		// siteTime - t in the site's time zone (pkg/sitetime): stored
		// times are UTC, so a template formatting one itself
		// ({{(siteTime .CreatedAt).Format "..."}}) shows local time.
		// Takes a time.Time or a *time.Time (nil - the zero time).
		// The zone is the request's (a user's own, see getOrParseIn).
		"siteTime": func(v any) time.Time {
			switch t := v.(type) {
			case time.Time:
				return inZone(t)
			case *time.Time:
				if t != nil {
					return inZone(*t)
				}
			}
			return time.Time{}
		},
		// formatDate - a date and time in the admin's lists: the
		// "datetime" format of the catalog, "-" for no date.
		"formatDate": func(d time.Time) string {
			if d.IsZero() || d.Year() < 1970 {
				return "-"
			}
			return dateformat.Format(inZone(d), datePattern(entity.DateTimeFormat), setLang())
		},
		// now - the current time, for date rules in a theme's field
		// templates (fields/*.html).
		"now": time.Now,
		// currentYear - the year, for a theme's own lines (the copyright
		// itself is the copyright block, copyrightYears).
		"currentYear": func() int { return time.Now().Year() },
		// copyrightYears - the copyright block's years from its first one
		// (CopyrightYears): "2026", "2008-2026".
		"copyrightYears": func(since string) string { return CopyrightYears(since, time.Now()) },
	}
}

// fieldFuncs - a field's value shown (formatters, numbers, e-mails,
// phones, links, images, files) and the addresses of nodes, users and
// files.
func (c funcScope) fieldFuncs() template.FuncMap {
	setLang := c.setLang
	return template.FuncMap{
		// formatField - a field's value shown by a module's formatter
		// (FormatField) in layout, "" - none for it (no such formatter,
		// its module off): the template shows the value itself (fields/
		// link.html). layout - entity.FieldLayout or its string (a View
		// field's config).
		"formatField": func(t entity.ContentTypeFieldType, name any, value any, layout any) template.HTML {
			n, _ := name.(string)
			out, _ := FormatField(i18n.WithLang(context.Background(), setLang()), t, n, value, entity.ParseFieldLayout(fmt.Sprint(layout)))
			return out
		},
		// fieldNumber, fieldEmail, fieldPhone - a number, e-mail or phone
		// field's value for the field_number_*, field_email_*, field_phone_*
		// templates (FieldNumber, FieldEmail, FieldPhone).
		"fieldNumber": func(field entity.FieldInstanceDef, value any) FieldNumber {
			return newFieldNumber(field, value, setLang())
		},
		"fieldEmail": func(field entity.FieldInstanceDef, value any) FieldEmail {
			return newFieldEmail(field, value)
		},
		"fieldPhone": func(value any) FieldPhone { return newFieldPhone(value) },
		// fieldTypeClass - a field type in a View item's wrapper class
		// (item-field-<it>): the type, but "links" for a link field -
		// item-field-link is the class of a field's value linked to the
		// content, which themes style.
		"fieldTypeClass": func(t entity.ContentTypeFieldType) string {
			if t == entity.FieldTypeLink {
				return "links"
			}
			return string(t)
		},
		// fieldLinks - a link field's links to show in layout: all, or
		// only the first (entity.FieldLayoutFirst).
		"fieldLinks": func(value any, layout any) []entity.Link {
			return entity.Links(FirstValue(value, entity.ParseFieldLayout(fmt.Sprint(layout))))
		},
		// humanFileSize formats a byte count as "245 KB"/"3.1 MB" (file field
		// download links).
		"humanFileSize": func(bytes int) string {
			return FormatHumanFileSize(bytes)
		},
		// styledImage - an image file's derivative URI for an image
		// style (e.g. "tags_list"), see imageurl.Styled.
		"styledImage": imageurl.Styled,
		// styledPicture - an image in a style ("" - itself), for img:
		// {{img (styledPicture .File "card") .File.Alt}}.
		"styledPicture": styledPicture,
		// lightboxImage - what the lightbox opens of an image file (its
		// large copy, imageurl.LightboxPicture): the link's href and
		// data-pswp-width/height.
		"lightboxImage": lightboxPicture,
		// img - an <img> of a picture (imageurl.Picture: styledPicture, an
		// item's Picture) with its srcset, sizes and size; see imgTag.
		"img": imgTag,
		// publicFileURL - a file's address (imageurl.FileURL; a private
		// file's imageurl.PrivateFileURL).
		"publicFileURL": func(uri string) string {
			if uri == "" {
				return ""
			}
			if key, ok := strings.CutPrefix(uri, entity.PrivateScheme); ok {
				return imageurl.PrivateFileURL(key)
			}
			return imageurl.FileURL(strings.TrimPrefix(uri, "public://"))
		},
		// resolvePublicURI - see imageurl.PublicURI: unlike publicFileURL, it
		// passes non-public:// paths (e.g. a bundled static asset) unchanged.
		"resolvePublicURI": func(uri string) string {
			return imageurl.PublicURI(uri)
		},
		// nodeEditPath / nodeDeletePath - a node's edit and delete pages
		// (a module type's own, see entity.NodeEditPath).
		"nodeEditPath": func(nodeType any, id int) string {
			return entity.NodeEditPath(fmt.Sprint(nodeType), id)
		},
		"nodeDeletePath": func(nodeType any, id int) string {
			return entity.NodeDeletePath(fmt.Sprint(nodeType), id)
		},
		"userURL":        entity.UserURL,
		"userPathPrefix": func() string { return entity.UserPathPrefix },
		"userRegistrationIP": func(md map[string]any) string {
			return entity.RegistrationIPFromMetadata(md)
		},
		// avatarPlaceholder - the picture of a user without an avatar.
		"avatarPlaceholder": func() string { return entity.AvatarPlaceholderURL },
		// minPasswordLength - entity.MinPasswordLength, for the password
		// inputs' minlength and hints.
		"minPasswordLength": func() int { return entity.MinPasswordLength },
		// registrationOpen - visitors may sign up: the "Register" links
		// show (entity.RegistrationOpen).
		"registrationOpen": entity.RegistrationOpen,
	}
}

// languageFuncs - the page's language and its translations (the
// set's theme's).
func (c funcScope) languageFuncs() template.FuncMap {
	theme, setLang := c.theme, c.setLang
	return template.FuncMap{
		// t - an interface string translated to the page's language
		// (pkg/i18n): {{t "Save"}},
		// {{t "Hello @name" "@name" .user.Username}}.
		"t": func(source string, args ...any) template.HTML {
			return template.HTML(theme.translator().T(setLang(), "", source, args...))
		},
		// tc - t with a context, for strings that read differently in
		// different places: {{tc "Long month name" "May"}}.
		"tc": func(msgctx, source string, args ...any) template.HTML {
			return template.HTML(theme.translator().T(setLang(), msgctx, source, args...))
		},
		// plural - the form for a count:
		// {{plural .Views "@count view" "@count views"}}.
		"plural": func(count any, singular, plural string, args ...any) template.HTML {
			return template.HTML(theme.translator().Plural(setLang(), "", toInt(count), singular, plural, args...))
		},
		// lang - the page's language code (<html lang>).
		"lang": setLang,
		// localURL - an address of the site for the page's language: under
		// its prefix on a site of several (/en/news), as is otherwise or
		// for another site's address (entity.LocalizePath).
		"localURL": func(path string) string { return entity.LocalizePath(setLang(), path) },
		// localSelfURL - a page's canonical or og:url built from current
		// (.currentPath: the request's path without its language prefix)
		// under the page's language's prefix (entity.LocalizeSelfURL).
		"localSelfURL": func(u, current string) string { return entity.LocalizeSelfURL(setLang(), u, current) },
		// otherLanguage - the name of code's language when content in it
		// is shown on a page in another one of a site of several (a list
		// showing an original where there's no translation), else "":
		// {{with otherLanguage .Node.Langcode}}<span>{{.}}</span>{{end}}.
		"otherLanguage": func(code string) string {
			_, others := entity.SiteLanguages()
			if len(others) == 0 || code == "" || code == setLang() {
				return ""
			}
			return i18n.NativeName(code)
		},
		// tjs - t as plain text, for inline scripts and on* attributes:
		// html/template escapes it for the JS context
		// (confirm({{tjs "Delete @name?" "@name" .Name}})).
		"tjs": func(source string, args ...any) string {
			return theme.translator().Text(setLang(), "", source, args...)
		},
		// jsTranslationsURL - the page language's JavaScript translations
		// and Kosatka.t runtime (/i18n/<lang>.js), versioned by content.
		"jsTranslationsURL": func() string {
			l := setLang()
			return "/i18n/" + l + ".js?v=" + i18n.JSVersion(l)
		},
	}
}

// pageFuncs - regions and blocks, modules, the theme's and the site's
// assets and settings.
func (c funcScope) pageFuncs() template.FuncMap {
	t, theme := c.t, c.theme
	return template.FuncMap{
		// region converts a string to entity.Region so templates can index
		// map[entity.Region][]entity.BlockWithPlacement: the "index" builtin
		// doesn't convert a string constant to a defined string type.
		"region": func(s string) entity.Region {
			return entity.Region(s)
		},
		// regionHasBlocks - whether a region has a block visible on this
		// page (RegionHasVisibleBlocks); wraps a region's own wrapper
		// markup so an empty region renders nothing.
		"regionHasBlocks": func(region any, ctx any) bool {
			data, _ := ctx.(map[string]any)
			return RegionHasVisibleBlocks(entity.Region(fmt.Sprint(region)), data)
		},
		// hasVisibleCarouselBlock - the page shows a carousel View block
		// (HasVisibleCarouselBlock), so it needs the Swiper stylesheet.
		"hasVisibleCarouselBlock": func(ctx any) bool {
			data, _ := ctx.(map[string]any)
			return HasVisibleCarouselBlock(data)
		},
		// moduleEnabled - whether a built-in module is on (ModuleEnabled).
		"moduleEnabled": ModuleEnabled,
		// featureEnabled - whether a feature switch is on (FeatureEnabled).
		"featureEnabled": FeatureEnabled,
		// placementVisible - PlacementVisible for region_blocks.html.
		"placementVisible": func(p entity.BlockPlacementDef, ctx any) bool {
			data, _ := ctx.(map[string]any)
			return PlacementVisible(p, data) && entity.VisibleInLanguage(entity.VisibilityBlockPlacement, strconv.Itoa(p.ID), c.setLang())
		},
		// termName - a term's name in the page's language (tags, the tag
		// cloud).
		"termName": func(t entity.TaxonomyTerm) string {
			return entity.TranslatedText(c.setLang(), entity.TextTermName, strconv.Itoa(t.ID), t.Name)
		},
		// viewTitle - a View display's title in the page's language (a
		// View block's default title).
		"viewTitle": func(v *entity.ViewDisplayDef) string {
			if v == nil {
				return ""
			}
			return entity.TranslatedText(c.setLang(), entity.TextViewTitle, strconv.Itoa(v.ID), v.Label)
		},
		// blockTitle - a custom block's title in the page's language;
		// placementTitle - a placement's (fallback - the View's or the
		// menu's label when it has none of its own).
		"blockTitle": func(b entity.BlockDef) string {
			return entity.TranslatedText(c.setLang(), entity.TextBlockTitle, strconv.Itoa(b.ID), b.Title)
		},
		"placementTitle": func(p entity.BlockPlacementDef, fallback string) string {
			title := p.Title
			if title == "" {
				title = fallback
			}
			return entity.TranslatedText(c.setLang(), entity.TextPlacementTitle, strconv.Itoa(p.ID), title)
		},
		// resolveMenuBlockRoot - template wrapper around ResolveMenuBlockRoot
		// ("Initial visibility level"), used by region_blocks.html's menu
		// placement branch.
		"resolveMenuBlockRoot": func(tree []MenuTreeItem, startLevel int) []MenuTreeItem {
			return ResolveMenuBlockRoot(tree, startLevel)
		},
		// integrations - counters and verification code settings (verification
		// metas, analytics, raw head/body code) for the layout.
		"integrations": func() entity.IntegrationSettings {
			if t.integrations == nil {
				return entity.IntegrationSettings{}
			}
			return t.integrations.Current(context.Background())
		},
		"themeCSS": func(name string) string { return assets.ThemeCSS(theme.name(), name) },
		"themeJS":  func(name string) string { return assets.ThemeJS(theme.name(), name) },
		// themeStylesheets - the URLs of the stylesheets the theme's chain
		// declares in theme.yaml (a subtheme's own; site_styles, editors).
		"themeStylesheets": func() []string { return themes.Stylesheets(theme.name()) },
		// siteIcon - the pages' icon in the set's theme (SiteIconFor):
		// the "site_icon" define's.
		"siteIcon": func() SiteIcon {
			return SiteIconFor(theme.name())
		},
		// themeSetting - a setting of the set's theme (theme.yaml
		// "settings"): a bool's as bool, the others as string; nil - none.
		"themeSetting": func(id string) any {
			if t.themeSettings == nil {
				return nil
			}
			return t.themeSettings.Value(context.Background(), theme.name(), id)
		},
		// themeSettingsStyle - the <style> of the theme's colours set on
		// the Appearance page (partials/html.html, after the theme's
		// stylesheets).
		"themeSettingsStyle": func() template.HTML {
			if t.themeSettings == nil {
				return ""
			}
			return t.themeSettings.Style(context.Background(), theme.name())
		},
		"themeCSSAsync":  func(name string) template.HTML { return assets.ThemeCSSAsync(theme.name(), name) },
		"staticCSS":      assets.StaticCSS,
		"staticJS":       assets.StaticJS,
		"staticCSSAsync": assets.StaticCSSAsync,
		"moduleStyles":   moduleStyles,
		"moduleScripts":  moduleScripts,
		"attach":         attachModule,
		"attachBlock":    attachPluginBlock,
		// moduleJS/moduleCSS - a module's own script or stylesheet for one
		// page (its static/<file>.min.js|css), e.g. {{moduleJS "<module>"
		// "<file>"}}.
		"moduleJS":  func(module, file string) string { return assets.ModuleURL(module, file, "js") },
		"moduleCSS": func(module, file string) string { return assets.ModuleURL(module, file, "css") },
	}
}

// generalFuncs - numbers, slices, maps, strings and escaping, the same
// for every set.
var generalFuncs = template.FuncMap{
	// botTrap - the bot trap's fields for a form that guards against bots
	// (CheckBotTrap on its submission).
	"botTrap": botTrapFields,
	// commentCaptcha - the comment form's CAPTCHA site key for the user
	// (CommentCaptchaKey; "" - none).
	"commentCaptcha": func(user *entity.User) string { return CommentCaptchaKey(context.Background(), user) },
	"add":            func(a, b int) int { return a + b },
	"sub":            func(a, b int) int { return a - b },
	"mul":            func(a, b int) int { return a * b },
	"displayHeight": func(displayW, origW, origH int) int {
		if origW <= 0 || origH <= 0 {
			return 0
		}
		return displayW * origH / origW
	},
	"isEven": func(n int) bool { return n%2 == 0 },
	// sliceHead/sliceTail - the overflow split of dynamic/list.html
	// (entity.ViewDisplayDef.OverflowAfter). The builtin "slice" can't index
	// with an interface{}-boxed int read from a data map; these take any+int
	// through the normal argument conversion. Non-slices and out-of-range n
	// return the input or an empty slice instead of panicking.
	"sliceHead": func(items any, n int) any {
		v := reflect.ValueOf(items)
		if v.Kind() != reflect.Slice || n <= 0 || n >= v.Len() {
			return items
		}
		return v.Slice(0, n).Interface()
	},
	"sliceTail": func(items any, n int) any {
		v := reflect.ValueOf(items)
		if v.Kind() != reflect.Slice || n <= 0 || n >= v.Len() {
			return nil
		}
		return v.Slice(n, v.Len()).Interface()
	},
	// hasToken - whole-word membership in a space-separated string (TinyMCE
	// "plugins"/"toolbar" config, see entity.TextFormatDef), used to
	// preselect toolbar-builder checkboxes.
	"hasToken": func(s, token string) bool {
		for _, t := range strings.Fields(s) {
			if t == token {
				return true
			}
		}
		return false
	},
	"intContains": func(list []int, id int) bool {
		for _, v := range list {
			if v == id {
				return true
			}
		}
		return false
	},
	"stringContains": slices.Contains[[]string],
	"dict": func(values ...any) map[string]any {
		if len(values)%2 != 0 {
			return nil
		}
		dict := make(map[string]any, len(values)/2)
		for i := 0; i < len(values); i += 2 {
			key, ok := values[i].(string)
			if !ok {
				return nil
			}
			dict[key] = values[i+1]
		}
		return dict
	},
	"pathEscape": func(s string) string {
		return url.PathEscape(s)
	},
	"queryEscape": func(s string) string {
		return url.QueryEscape(s)
	},
	"toString": func(v any) string {
		return fmt.Sprint(v)
	},
}

// toInt - a template number (any integer kind) as int.
func toInt(v any) int {
	switch n := v.(type) {
	case int:
		return n
	case int8:
		return int(n)
	case int16:
		return int(n)
	case int32:
		return int(n)
	case int64:
		return int(n)
	case uint:
		return int(n)
	case uint32:
		return int(n)
	case uint64:
		return int(n)
	case float64:
		return int(n)
	}
	return 0
}
