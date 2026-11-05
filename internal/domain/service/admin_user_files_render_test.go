package service

import (
	"html/template"
	"strings"
	"testing"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/files"
	"github.com/kosatka-cms/kosatka/pkg/pagination"
)

// TestAdminUserFilesRender - the users' files list links each folder to
// its browser page; the browser page mounts the component on that user.
func TestAdminUserFilesRender(t *testing.T) {
	zzChdirToProjectRoot(t)
	render := func(page string, data map[string]any) string {
		t.Helper()
		svc := &DefaultTemplateService{cache: map[string]*template.Template{}}
		parts := strings.Split(page, "/")
		files, err := svc.pageFilesToParse(page, parts)
		if err != nil {
			t.Fatal(err)
		}
		set, err := svc.getOrParse(page, nil, func() ([]string, error) { return files, nil })
		if err != nil {
			t.Fatal(err)
		}
		var buf strings.Builder
		if err := set.ExecuteTemplate(&buf, "content", data); err != nil {
			t.Fatal(err)
		}
		return buf.String()
	}
	type row struct {
		files.UserFolderUsage
		User     *entity.User
		SizeText string
	}
	out := render("admin/people/user_files_list.html", map[string]any{
		"rows": []row{
			{files.UserFolderUsage{UserID: 7, Files: 3, Modified: time.Now()}, &entity.User{ID: 7, Username: "Ann", Slug: "users/ann"}, "1.5 MB"},
			{files.UserFolderUsage{UserID: 9, Files: 1, Modified: time.Now()}, nil, "10 B"},
		},
		"folders": 2, "totalFiles": 4, "totalSize": "1.5 MB", "pagination": pagination.New(1, 2, 50, "/admin/content/files", nil),
	})
	for _, want := range []string{`href="/admin/content/files/7"`, ">Ann</a>", "1.5 MB", "Deleted user #9"} {
		if !strings.Contains(out, want) {
			t.Errorf("list lacks %q", want)
		}
	}
	out = render("admin/people/user_files.html", map[string]any{"filesUser": &entity.User{ID: 7, Username: "Ann", Slug: "users/ann"}, "CSRFToken": "tok"})
	if !strings.Contains(strings.Join(strings.Fields(out), " "), "userID: 7 ,") || !strings.Contains(out, `rootLabel: "Files of Ann"`) || !strings.Contains(out, "KosatkaFileBrowser.mount") {
		t.Errorf("browser page:\n%s", out)
	}
}
