package views

import (
	"context"
	"testing"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

func strPtr(v string) *string { return &v }

// TestCommentFieldValueTrimLength - TrimLength 0 leaves the raw value
// untouched (including HTML tags); >0 strips tags and truncates to that
// many runes with a trailing "…".
func TestCommentFieldValueTrimLength(t *testing.T) {
	c := entity.Comment{
		Subject: strPtr("<b>Subject</b> with <i>markup</i>"),
		Content: "<p>The quick brown fox jumps over the lazy dog</p>",
	}

	t.Run("zero trim length leaves value untouched", func(t *testing.T) {
		got := commentFieldValue(context.Background(), c, entity.ViewFieldBuiltinCommentContent, 0)
		if got != c.Content {
			t.Fatalf("expected raw content unchanged, got %q", got)
		}
	})

	t.Run("positive trim length strips HTML and truncates", func(t *testing.T) {
		got := commentFieldValue(context.Background(), c, entity.ViewFieldBuiltinCommentContent, 10)
		want := "The quick…"
		if got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	})

	t.Run("strips HTML without truncating when under the limit", func(t *testing.T) {
		got := commentFieldValue(context.Background(), c, entity.ViewFieldBuiltinCommentSubject, 100)
		want := "Subject with markup"
		if got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	})

	t.Run("date field ignores trim length", func(t *testing.T) {
		dated := entity.Comment{CreatedAt: time.Date(2026, 3, 4, 5, 6, 0, 0, time.UTC)}
		got := commentFieldValue(context.Background(), dated, entity.ViewFieldBuiltinCommentCreatedAt, 3)
		if len(got) != len("02.01.2006 15:04") {
			t.Fatalf("expected a full formatted date, got %q", got)
		}
	})
}

// TestCommentListData - the list's fields in the display's order, one
// item per comment with their values, and the display's look.
func TestCommentListData(t *testing.T) {
	view := entity.ViewDisplayDef{DisplayStyle: entity.ViewDisplayStyleTable, HeaderText: "<p>Before</p>", GroupByField: "comment_created_at"}
	fields := entity.ViewFieldDefs{
		{ID: 2, BuiltinField: entity.ViewFieldBuiltinCommentContent, SortOrder: 2},
		{ID: 1, BuiltinField: entity.ViewFieldBuiltinCommentSubject, SortOrder: 1},
	}
	comments := entity.Comments{{ID: 7, Subject: strPtr("Hi"), Content: "Text"}}
	data := CommentListData(context.Background(), view, fields, comments)

	head := data["listFieldsHead"].(entity.FieldInstanceDefs)
	if len(head) != 2 || head[0].MachineName != string(entity.ViewFieldBuiltinCommentSubject) {
		t.Fatalf("fields: %+v", head)
	}
	items := data["items"].([]CommentListItem)
	if len(items) != 1 || items[0].Values[head[0].ID] != "Hi" || items[0].Values[head[1].ID] != "Text" {
		t.Fatalf("items: %+v", items)
	}
	if data["displayStyle"] != "table" || data["headerText"] != "<p>Before</p>" || data["groupByField"] != "comment_created_at" {
		t.Fatalf("look: %v %v %v", data["displayStyle"], data["headerText"], data["groupByField"])
	}
}
