package admin

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestContentListSortURL(t *testing.T) {
	tests := []struct {
		name    string
		rawURL  string
		column  string
		wantURL string
	}{
		{
			name:    "default sort, click same column toggles to asc",
			rawURL:  "/admin/content",
			column:  "updated",
			wantURL: "/admin/content?order=asc&sort=updated",
		},
		{
			name:    "click a different column resets to desc",
			rawURL:  "/admin/content?sort=updated&order=asc",
			column:  "views",
			wantURL: "/admin/content?order=desc&sort=views",
		},
		{
			name:    "click current desc column toggles to asc",
			rawURL:  "/admin/content?sort=status&order=desc",
			column:  "status",
			wantURL: "/admin/content?order=asc&sort=status",
		},
		{
			name:    "unknown sort value in query falls back to default (updated)",
			rawURL:  "/admin/content?sort=bogus",
			column:  "updated",
			wantURL: "/admin/content?order=asc&sort=updated",
		},
		{
			name:    "preserves status/moderation/type filters",
			rawURL:  "/admin/content?status=1&moderation=0&type=article",
			column:  "created",
			wantURL: "/admin/content?moderation=0&order=desc&sort=created&status=1&type=article",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, tt.rawURL, nil)
			got := contentListSortURL(r, tt.column)
			assertSameQueryURL(t, got, tt.wantURL)
		})
	}
}

func TestModerationListSortURL(t *testing.T) {
	tests := []struct {
		name    string
		rawURL  string
		column  string
		wantURL string
	}{
		{
			name:    "default sort is created, toggling it goes to asc",
			rawURL:  "/admin/moderation",
			column:  "created",
			wantURL: "/admin/moderation?order=asc&sort=created",
		},
		{
			name:    "different column resets to desc",
			rawURL:  "/admin/moderation?sort=created&order=asc",
			column:  "title",
			wantURL: "/admin/moderation?order=desc&sort=title",
		},
		{
			name:    "preserves status/type filters",
			rawURL:  "/admin/moderation?status=1&type=article",
			column:  "views",
			wantURL: "/admin/moderation?order=desc&sort=views&status=1&type=article",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, tt.rawURL, nil)
			got := moderationListSortURL(r, tt.column)
			assertSameQueryURL(t, got, tt.wantURL)
		})
	}
}

func TestCommentListSortURL(t *testing.T) {
	tests := []struct {
		name    string
		rawURL  string
		column  string
		wantURL string
	}{
		{
			name:    "default sort is created, toggling it goes to asc",
			rawURL:  "/admin/content/comment",
			column:  "created",
			wantURL: "/admin/content/comment?order=asc&sort=created&status=spam",
		},
		{
			name:    "different column resets to desc",
			rawURL:  "/admin/content/comment?sort=created&order=asc",
			column:  "updated",
			wantURL: "/admin/content/comment?order=desc&sort=updated&status=spam",
		},
		{
			// sort=bogus is invalid so it falls back to "created", which then matches the
			// clicked column, so order toggles off the (valid) current order=asc -> desc.
			name:    "invalid sort value falls back to created and toggles off current order",
			rawURL:  "/admin/content/comment?sort=bogus&order=asc",
			column:  "created",
			wantURL: "/admin/content/comment?order=desc&sort=created&status=spam",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, tt.rawURL, nil)
			got := commentListSortURL(r, tt.column, "spam")
			assertSameQueryURL(t, got, tt.wantURL)
		})
	}
}
