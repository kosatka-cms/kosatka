package admin

import (
	"testing"
)

func TestParseRedirectBulk(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []redirectPair
	}{
		{
			// Documented format (see web/templates/admin/config/redirects.html): "RewriteRule from$ to [R=301,L]" (no caret).
			name:  "htaccess RewriteRule format, target without leading slash",
			input: `RewriteRule old-page$ new-page [R=301,L]`,
			want:  []redirectPair{{From: "old-page", To: "/new-page"}},
		},
		{
			name:  "htaccess RewriteRule format, target already has leading slash",
			input: `RewriteRule old-page$ /new-page [R=301,L]`,
			want:  []redirectPair{{From: "old-page", To: "/new-page"}},
		},
		{
			// Known quirk, not the documented format: a genuine .htaccess line anchored with "^" leaks
			// the caret into From because the regex only excludes whitespace and "$", not "^".
			name:  "htaccess format with undocumented leading caret leaks into From",
			input: `RewriteRule ^old-page$ /new-page [R=301,L]`,
			want:  []redirectPair{{From: "^old-page", To: "/new-page"}},
		},
		{
			name:  "simple arrow format",
			input: "old-path -> new-path",
			want:  []redirectPair{{From: "old-path", To: "/new-path"}},
		},
		{
			name:  "simple arrow format with leading slash already present",
			input: "/old-path -> /new-path",
			want:  []redirectPair{{From: "old-path", To: "/new-path"}},
		},
		{
			name:  "blank lines and comments are skipped",
			input: "\n# a comment\n\nold -> new\n",
			want:  []redirectPair{{From: "old", To: "/new"}},
		},
		{
			name:  "multiple lines mixing both formats",
			input: "a -> b\nRewriteRule c$ d [R=301,L]",
			want: []redirectPair{
				{From: "a", To: "/b"},
				{From: "c", To: "/d"},
			},
		},
		{
			name:  "line matching neither format is ignored",
			input: "this line has neither format",
			want:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseRedirectBulk(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("got %+v, want %+v", got, tt.want)
			}
			for i := range tt.want {
				if got[i] != tt.want[i] {
					t.Errorf("index %d: got %+v, want %+v", i, got[i], tt.want[i])
				}
			}
		})
	}
}
