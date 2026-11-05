package files

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

func TestExtractUserFilePaths(t *testing.T) {
	html := `<p><img src="/uploads/u42/docs/a_1.jpg" style="width: 10px"></p>` +
		`<a href="https://example.com/uploads/u42/My%20File.pdf">file</a>` +
		`<img src='/uploads/u7/x.png?itok=1'> <img src="/uploads/u42/docs/a_1.jpg">` +
		`<img src="/uploads/user_pictures/p.jpg"> <img src="/uploads/content/images/b.jpg">`
	got := extractUserFilePaths(html)
	want := []string{"u42/docs/a_1.jpg", "u42/My File.pdf", "u7/x.png"}
	if !slices.Equal(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
	// Non-ASCII names are kept; bytes that aren't UTF-8 are dropped.
	if got := extractUserFilePaths(`<img src="/uploads/u3/%D1%84%D0%BE%D1%82%D0%BE.jpg"><img src="/uploads/u3/%FF.jpg">`); !slices.Equal(got, []string{"u3/фото.jpg"}) {
		t.Fatalf("decoded paths: %q", got)
	}
	if extractUserFilePaths("<p>no files</p>") != nil {
		t.Fatal("paths in a text without files")
	}
}

// fakeUsageRepo serves fixed texts.
type fakeUsageRepo struct {
	texts []struct {
		source string
		id     int
		text   string
	}
}

func (f *fakeUsageRepo) ScanTextsFor(_ context.Context, path string, fn func(string, int, string)) error {
	for _, t := range f.texts {
		if strings.Contains(t.text, "/uploads/"+path) {
			fn(t.source, t.id, t.text)
		}
	}
	return nil
}

func (f *fakeUsageRepo) Describe(_ context.Context, usages []entity.UserFileUsage) ([]entity.UserFileRef, error) {
	out := make([]entity.UserFileRef, len(usages))
	for i, u := range usages {
		out[i] = entity.UserFileRef{Source: u.Source, EntityID: u.EntityID, Title: "Harbour"}
	}
	return out, nil
}

// TestUserFileUsageRefs - the check finds every text linking to the
// exact file, not to one whose name merely starts the same.
func TestUserFileUsageRefs(t *testing.T) {
	repo := &fakeUsageRepo{}
	add := func(source string, id int, text string) {
		repo.texts = append(repo.texts, struct {
			source string
			id     int
			text   string
		}{source, id, text})
	}
	add(entity.UserFileSourceNodeField, 1, `<img src="/uploads/u5/a.jpg"><img src="/uploads/u5/a.jpg">`)
	add(entity.UserFileSourceComment, 2, `<img src="/uploads/u5/a.jpg"> <img src="/uploads/u5/docs/b.pdf">`)
	add(entity.UserFileSourceSignature, 3, `<img src="/uploads/u5/a.jpg.bak">`)
	add(entity.UserFileSourceComment, 4, `<p>no files</p>`)
	s := NewUserFileUsageService(repo)
	ctx := t.Context()

	refs, err := s.Refs(ctx, 5, "a.jpg")
	if err != nil || len(refs) != 2 {
		t.Fatalf("refs: %+v %v", refs, err)
	}
	if refs[0].Title != "Comment on \"Harbour\"" || refs[1].Title != "Content \"Harbour\"" {
		t.Errorf("labels: %q, %q", refs[0].Title, refs[1].Title)
	}
	if refs, _ := s.Refs(ctx, 5, "missing.jpg"); len(refs) != 0 {
		t.Errorf("refs of an unused file: %+v", refs)
	}
}
