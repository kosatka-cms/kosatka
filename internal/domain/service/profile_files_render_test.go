package service

import (
	"html/template"
	"strings"
	"testing"
)

// TestProfileFilesTabRender - the own profile has a "Files" tab; the
// files tab mounts the file browser.
func TestProfileFilesTabRender(t *testing.T) {
	zzChdirToProjectRoot(t)
	svc := &DefaultTemplateService{cache: map[string]*template.Template{}}
	files, err := svc.pageFilesToParse("user/profile_page.html", []string{"user", "profile_page.html"})
	if err != nil {
		t.Fatal(err)
	}
	set, err := svc.getOrParse("user/profile_page.html", nil, func() ([]string, error) { return files, nil })
	if err != nil {
		t.Fatal(err)
	}
	var buf strings.Builder
	data := map[string]any{"isCurrentUser": true, "activeTab": "files", "CSRFToken": "tok", "profileUser": map[string]any{"Slug": "users/ann"}}
	if err := set.ExecuteTemplate(&buf, "content", data); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	for _, want := range []string{`<a href="/me/files" class="tab-item active">`, `id="user-files-browser"`, "file-browser.min.js", "KosatkaFileBrowser.mount"} {
		if !strings.Contains(out, want) {
			t.Errorf("no %q", want)
		}
	}
}
