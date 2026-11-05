package admin

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/internal/interfaces"
)

// stubTermUpdates - the taxonomy service calls of TaxonomyTermUpdate:
// GenerateTermSlug answers "<vid>/generated-<termID>", UpdateTerm
// records the slug and returns updateErr.
type stubTermUpdates struct {
	service.TaxonomyService
	updateErr   error
	updatedSlug string
	generated   []int // termID of every GenerateTermSlug call
}

func (s *stubTermUpdates) GenerateTermSlug(_ context.Context, vid int, _ string, termID int) (string, error) {
	s.generated = append(s.generated, termID)
	return fmt.Sprintf("%d/generated-%d", vid, termID), nil
}

func (s *stubTermUpdates) UpdateTerm(_ context.Context, id int, name, slug, _ string, _, _ int) (*entity.TaxonomyTerm, error) {
	if s.updateErr != nil {
		return nil, s.updateErr
	}
	s.updatedSlug = slug
	return &entity.TaxonomyTerm{ID: id, Name: name, Slug: slug}, nil
}

func (s *stubTermUpdates) UpdateTermPageTitle(context.Context, int, string) error { return nil }

func postTermUpdate(t *testing.T, stub *stubTermUpdates, form url.Values) string {
	t.Helper()
	c := testController(interfaces.BaseDeps{}, func(c *Controller) { c.taxonomyService = stub })
	r := httptest.NewRequest(http.MethodPost, "/admin/config/taxonomy/3/terms/7", strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.SetPathValue("vid", "3")
	r.SetPathValue("id", "7")
	w := httptest.NewRecorder()
	c.TaxonomyTermUpdate(w, r)
	if w.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", w.Code)
	}
	return w.Header().Get("Location")
}

// TestTaxonomyTermUpdateEmptySlug - an emptied slug on the term edit
// page is generated as on the create form (GenerateTermSlug: vocabulary
// prefix, unique), with the term's own ID kept out of the uniqueness
// check; a typed slug is kept as is.
func TestTaxonomyTermUpdateEmptySlug(t *testing.T) {
	stub := &stubTermUpdates{}
	loc := postTermUpdate(t, stub, url.Values{"name": {"Go"}, "slug": {"  "}})
	if loc != "/admin/config/taxonomy/3/terms/7/edit?success=1" {
		t.Errorf("redirect = %q", loc)
	}
	if stub.updatedSlug != "3/generated-7" || len(stub.generated) != 1 || stub.generated[0] != 7 {
		t.Errorf("slug = %q, GenerateTermSlug termIDs %v; want 3/generated-7 from [7]", stub.updatedSlug, stub.generated)
	}

	stub = &stubTermUpdates{}
	postTermUpdate(t, stub, url.Values{"name": {"Go"}, "slug": {" topics/go "}})
	if stub.updatedSlug != "topics/go" || len(stub.generated) != 0 {
		t.Errorf("slug = %q, GenerateTermSlug calls %v; want topics/go, none", stub.updatedSlug, stub.generated)
	}
}

// TestTaxonomyTermUpdateParentErrors - the service's parent refusals
// come back to the edit form as their own messages.
func TestTaxonomyTermUpdateParentErrors(t *testing.T) {
	for _, tc := range []struct {
		err  error
		want string
	}{
		{fmt.Errorf("updating term: %w", service.ErrTaxonomyTermParentLoop), "parent-loop"},
		{fmt.Errorf("updating term: %w", service.ErrTaxonomyTermParentInvalid), "parent-invalid"},
		{fmt.Errorf("updating term: boom"), "update"},
	} {
		loc := postTermUpdate(t, &stubTermUpdates{updateErr: tc.err}, url.Values{"name": {"Go"}, "slug": {"tags/go"}, "parent_id": {"9"}})
		if want := "/admin/config/taxonomy/3/terms/7/edit?error=" + tc.want; loc != want {
			t.Errorf("%v: redirect = %q, want %q", tc.err, loc, want)
		}
	}
}
