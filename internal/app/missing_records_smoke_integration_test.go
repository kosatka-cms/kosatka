//go:build dbintegration

package app

import (
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/testdb"
)

// TestMissingRecords - every GET page with a path parameter, signed in as
// the administrator, for a record that doesn't exist: the 404 page (or a
// redirect), never a server error - a missing record is the visitor's
// mistake (RenderLoadFailure), and the log has no errors. So are private
// messages to no one.
func TestMissingRecords(t *testing.T) {
	testdb.Each(t, func(t *testing.T, db testdb.DB) {
		s := serveSite(t, db)
		s.signIn()
		files, err := filepath.Glob(filepath.Join(repoRoot(t), "internal", "modules", "*", "routes.go"))
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, filepath.Join(repoRoot(t), "internal", "http", "router.go"))
		param := regexp.MustCompile(`\{[a-zA-Z_]+(\.\.\.)?\}`)
		seen := map[string]bool{}
		for _, f := range files {
			src, err := os.ReadFile(f)
			if err != nil {
				t.Fatal(err)
			}
			for _, m := range regexp.MustCompile(`"GET (/[^" ]*\{[^" ]*)"`).FindAllStringSubmatch(string(src), -1) {
				path := param.ReplaceAllString(m[1], "987654")
				if seen[path] {
					continue
				}
				seen[path] = true
				if code, _ := s.get(path); code >= http.StatusInternalServerError {
					t.Errorf("GET %s (%s): %d", path, m[1], code)
				}
			}
		}
		if len(seen) < 20 {
			t.Fatalf("only %d GET routes with a parameter found", len(seen))
		}
		// Private messages: a recipient who isn't a user is the form's
		// mistake (400), a reply to no thread a 404.
		token := s.csrf("/me/messages/new")
		if resp := s.post("/me/messages/new", url.Values{"csrf_token": {token}, "subject": {"Hi"}, "body": {"<p>Hi</p>"},
			"recipients[]": {"987654"}}); resp.StatusCode != http.StatusBadRequest {
			t.Errorf("a message to no user: %d, want 400", resp.StatusCode)
		}
		if resp := s.post("/me/messages/thread/987654/reply", url.Values{"csrf_token": {token}, "body": {"<p>Hi</p>"}}); resp.StatusCode != http.StatusNotFound {
			t.Errorf("a reply to no thread: %d, want 404", resp.StatusCode)
		}
		s.checkLog()
	})
}
