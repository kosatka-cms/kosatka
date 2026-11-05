package service

import (
	"context"
	"errors"
	"html/template"
	"slices"
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

func TestParseErrorPagePaths(t *testing.T) {
	got := ParseErrorPagePaths(" page/oops \n\n/page/denied/\r\nhttps://example.com/page/full\n")
	want := []string{"/page/oops", "/page/denied", "/page/full"}
	if !slices.Equal(got, want) {
		t.Fatalf("got %q", got)
	}
}

func TestErrorPagesRegistry(t *testing.T) {
	t.Cleanup(func() { SetErrorPages(404, nil); SetErrorPages(403, nil) })
	bySlug := func(_ context.Context, slug string) (*entity.Node, error) {
		switch slug {
		case "page/oops":
			return &entity.Node{ID: 7, Slug: slug, Status: 1}, nil
		case "page/denied":
			return &entity.Node{ID: 9, Slug: slug, Status: 1}, nil
		}
		return nil, errors.New("not found")
	}
	nodes, missing := ResolveErrorPages(t.Context(), bySlug, []string{"/page/oops", "/nope"})
	if len(nodes) != 1 || nodes[0].ID != 7 || !slices.Equal(missing, []string{"/nope"}) {
		t.Fatalf("resolve: %v %v", nodes, missing)
	}
	LoadErrorPages(t.Context(), bySlug, &entity.SiteInformation{Page404: "/page/oops\n/nope", Page403: "/page/denied"})
	if ids := ErrorPageIDs(404); !slices.Equal(ids, []int{7}) {
		t.Errorf("404 ids %v", ids)
	}
	if ErrorPageStatus(7) != 404 || ErrorPageStatus(9) != 403 || ErrorPageStatus(1) != 0 {
		t.Errorf("status: %d %d %d", ErrorPageStatus(7), ErrorPageStatus(9), ErrorPageStatus(1))
	}
}

// TestForbiddenTemplate - the engine's 403 page renders.
func TestForbiddenTemplate(t *testing.T) {
	zzChdirToProjectRoot(t)
	svc := &DefaultTemplateService{cache: map[string]*template.Template{}}
	var buf strings.Builder
	if err := svc.RenderNamed(&buf, "content", map[string]any{}, "errors/forbidden.html"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "403 - Access denied") {
		t.Errorf("forbidden page: %s", buf.String())
	}
}

// TestBadRequestTemplate - the engine's 400 page shows why the request
// was refused and links back to where it came from (else home).
func TestBadRequestTemplate(t *testing.T) {
	zzChdirToProjectRoot(t)
	svc := &DefaultTemplateService{cache: map[string]*template.Template{}}
	render := func(data map[string]any) string {
		t.Helper()
		var buf strings.Builder
		if err := svc.RenderNamed(&buf, "content", data, "errors/bad_request.html"); err != nil {
			t.Fatal(err)
		}
		return buf.String()
	}
	out := render(map[string]any{"message": "Enter the message.", "back": "/me/messages"})
	if !strings.Contains(out, "Enter the message.") || !strings.Contains(out, `href="/me/messages"`) {
		t.Errorf("with back: %s", out)
	}
	if out := render(map[string]any{"message": "x"}); !strings.Contains(out, `href="/"`) {
		t.Errorf("without back: %s", out)
	}
}
