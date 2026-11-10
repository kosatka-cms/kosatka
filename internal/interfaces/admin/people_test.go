package admin

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPeopleListSortURL(t *testing.T) {
	tests := []struct {
		name    string
		rawURL  string
		column  string
		wantURL string
	}{
		{
			name:    "empty sort defaults to created, toggles to asc",
			rawURL:  "/admin/users",
			column:  "created",
			wantURL: "/admin/users?order=asc&sort=created",
		},
		{
			name:    "different column resets to desc",
			rawURL:  "/admin/users?sort=created&order=asc",
			column:  "last_access",
			wantURL: "/admin/users?order=desc&sort=last_access",
		},
		{
			// No sort in query defaults to "created", which matches the clicked column, so
			// order toggles off the default current order=desc -> asc.
			name:    "preserves created/status/role filters and toggles off default order",
			rawURL:  "/admin/users?created=1&status=1&role=2",
			column:  "created",
			wantURL: "/admin/users?created=1&order=asc&role=2&sort=created&status=1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, tt.rawURL, nil)
			got := peopleListSortURL(r, tt.column)
			assertSameQueryURL(t, got, tt.wantURL)
		})
	}
}
