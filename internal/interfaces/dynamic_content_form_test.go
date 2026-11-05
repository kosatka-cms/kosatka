package interfaces

import (
	"context"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/interfaces/form"
)

// TestValidateSlugFree - a slug typed by hand, on the edit or the create
// form, can't be another node's; an invalid or an empty slug, and one not
// used (the automatic one on), aren't checked.
func TestValidateSlugFree(t *testing.T) {
	c := &DynamicContentController{nodeRepo: slugsTaken{taken: map[string]bool{"news/taken": true}}}
	cases := []struct {
		name    string
		f       form.DynamicForm
		errs    map[string]string
		wantErr bool
	}{
		{"another node's slug", form.DynamicForm{NodeID: 7, Slug: "/news/taken/"}, map[string]string{}, true},
		{"a free slug", form.DynamicForm{NodeID: 7, Slug: "news/free"}, map[string]string{}, false},
		{"emptied (keeps the current one)", form.DynamicForm{NodeID: 7, Slug: " "}, map[string]string{}, false},
		{"the create form", form.DynamicForm{Slug: "news/taken"}, map[string]string{}, true},
		{"automatic: not used", form.DynamicForm{NodeID: 7, Slug: "news/taken", PathAutoShown: true, PathAuto: true}, map[string]string{}, false},
	}
	for _, tc := range cases {
		c.validateSlugFree(context.Background(), &tc.f, tc.errs)
		if _, got := tc.errs["slug"]; got != tc.wantErr {
			t.Errorf("%s: slug error %v, want %v", tc.name, got, tc.wantErr)
		}
	}
	errs := map[string]string{"slug": "format"}
	c.validateSlugFree(context.Background(), &form.DynamicForm{NodeID: 7, Slug: "news/taken"}, errs)
	if errs["slug"] != "format" {
		t.Errorf("a format error is kept: %q", errs["slug"])
	}
}
