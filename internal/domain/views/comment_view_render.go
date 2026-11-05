package views

import (
	"context"
	"slices"
	"strings"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/pkg/htmlsanitize"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
	"github.com/kosatka-cms/kosatka/pkg/sitetime"
)

// CommentListItem - the render unit of a comment-based View, the comment
// counterpart of views.ContentListItem (internal/interfaces). Comments have only
// builtin fields (entity.ViewFieldBuiltinField comment_* constants), so
// Values is built directly by commentFieldValue. Lives in package service
// so both service.CacheService (View blocks) and DynamicContentController (pages)
// can build it.
type CommentListItem struct {
	Comment entity.Comment
	Fields  entity.FieldInstanceDefs
	Values  map[int]string
	// Dates - the date fields' values (UTC); comment_list.html shows them in
	// the request's time zone, in the field's date format when set.
	Dates map[int]time.Time
}

// BuildCommentFieldInstances - the comment counterpart of
// BuildContentFieldInstances. Every entry is a
// builtin field. The synthetic FieldInstanceDef reuses the ViewFieldDef's
// ID: comments have no field storages an ID could collide with.
func BuildCommentFieldInstances(ctx context.Context, viewFields entity.ViewFieldDefs) entity.FieldInstanceDefs {
	var listFields entity.FieldInstanceDefs
	for _, vf := range viewFields {
		if !slices.Contains(commentBuiltins.fields, vf.BuiltinField) {
			continue
		}
		label := vf.Label
		if label == "" {
			label = FieldBuiltinLabel(ctx, vf.BuiltinField)
		}
		fieldType := entity.FieldTypeText
		if vf.BuiltinField == entity.ViewFieldBuiltinCommentCreatedAt {
			fieldType = entity.FieldTypeDate
		}
		listFields = append(listFields, entity.FieldInstanceDef{
			ID:              vf.ID,
			MachineName:     string(vf.BuiltinField),
			Label:           label,
			FieldType:       fieldType,
			ShowLabelInList: vf.ShowLabel,
			TrimLength:      vf.TrimLength,
			DateFormat:      vf.DateFormat,
		})
	}
	return listFields
}

// commentFieldValue resolves one comment_* builtin field's display value
// from the eager-loaded entity.Comment (Node/Author loaded by
// RunCommentView); Subject, Node and Author may all be nil. trimLength
// (entity.ViewFieldDef.TrimLength) is applied via trimCommentText to every
// field except the formatted date.
func commentFieldValue(ctx context.Context, c entity.Comment, builtin entity.ViewFieldBuiltinField, trimLength int) string {
	switch builtin {
	case entity.ViewFieldBuiltinCommentSubject:
		if c.Subject != nil {
			return trimCommentText(*c.Subject, trimLength)
		}
		return ""
	case entity.ViewFieldBuiltinCommentContent:
		return trimCommentText(c.Content, trimLength)
	case entity.ViewFieldBuiltinCommentAuthor:
		if c.Author != nil {
			return trimCommentText(c.Author.Username, trimLength)
		}
		return i18n.T(ctx, "Guest")
	case entity.ViewFieldBuiltinCommentCreatedAt:
		return sitetime.Format(c.CreatedAt, "02.01.2006 15:04")
	case entity.ViewFieldBuiltinCommentNodeTitle:
		if c.Node != nil {
			return trimCommentText(c.Node.Title, trimLength)
		}
		return ""
	default:
		return ""
	}
}

// trimCommentText applies a comment View field's TrimLength: limit<=0
// leaves s untouched (raw value, escaped by html/template); limit>0 strips
// markup via htmlsanitize.StripToText and truncates to limit runes (not
// bytes, so multi-byte text is not cut mid-character).
func trimCommentText(s string, limit int) string {
	if limit <= 0 {
		return s
	}
	plain := htmlsanitize.StripToText(s)
	runes := []rune(plain)
	if len(runes) <= limit {
		return plain
	}
	return strings.TrimRight(string(runes[:limit]), " \t\n\r") + "…"
}

// BuildCommentListItems pairs each comment with the display's synthetic
// fields (listFields, from BuildCommentFieldInstances) and their resolved
// values.
func BuildCommentListItems(ctx context.Context, comments entity.Comments, listFields entity.FieldInstanceDefs) []CommentListItem {
	items := make([]CommentListItem, len(comments))
	for i, cm := range comments {
		values := make(map[int]string, len(listFields))
		dates := map[int]time.Time{}
		for _, lf := range listFields {
			values[lf.ID] = commentFieldValue(ctx, cm, entity.ViewFieldBuiltinField(lf.MachineName), lf.TrimLength)
			if lf.FieldType == entity.FieldTypeDate {
				dates[lf.ID] = cm.CreatedAt
			}
		}
		items[i] = CommentListItem{Comment: cm, Fields: listFields, Values: values, Dates: dates}
	}
	return items
}

// CommentListData - what dynamic/comment_list.html reads of a comment
// View's list and its look (display style, header and footer, custom
// CSS, row template, grouping), the same for its page, its block, a
// profile tab and the admin's preview (a view with the draft settings);
// the caller adds what only it has (pagination, isViewBlock). The list's
// fields are the display's viewFields, in their order.
func CommentListData(ctx context.Context, view entity.ViewDisplayDef, viewFields entity.ViewFieldDefs, comments entity.Comments) map[string]any {
	listFields := BuildCommentFieldInstances(ctx, viewFields.SortBySortOrder())
	return map[string]any{
		"items":           BuildCommentListItems(ctx, comments, listFields),
		"listFieldsHead":  listFields,
		"displayStyle":    string(view.DisplayStyle),
		"headerText":      view.HeaderText,
		"footerText":      view.FooterText,
		"customCSS":       view.CustomCSS,
		"rowTemplate":     view.RowTemplate,
		"groupByField":    view.GroupByField,
		"groupDateFormat": view.GroupDateFormat,
	}
}
