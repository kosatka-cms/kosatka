package admin

import (
	"context"
	"io"
	"log/slog"
	"net/url"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/interfaces"
)

// assertSameQueryURL compares path and query params ignoring encoding/ordering differences.
func assertSameQueryURL(t *testing.T, got, want string) {
	t.Helper()
	gu, err := url.Parse(got)
	if err != nil {
		t.Fatalf("got %q is not a valid URL: %v", got, err)
	}
	wu, err := url.Parse(want)
	if err != nil {
		t.Fatalf("want %q is not a valid URL: %v", want, err)
	}
	if gu.Path != wu.Path {
		t.Errorf("path = %q, want %q", gu.Path, wu.Path)
	}
	if gu.Query().Encode() != wu.Query().Encode() {
		t.Errorf("query = %q, want %q", gu.Query().Encode(), wu.Query().Encode())
	}
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// testController - the admin controller built as the app builds it
// (NewController) over d (a discarding logger when d has none); set fills
// the fields a test needs.
func testController(d interfaces.BaseDeps, set func(c *Controller)) *Controller {
	if d.Logger == nil {
		d.Logger = testLogger()
	}
	c := NewController(d, Deps{})
	if set != nil {
		set(c)
	}
	return c
}

// stubSiteInformation - the site information held in memory.
type stubSiteInformation struct {
	site entity.SiteInformation
}

func (s *stubSiteInformation) Get(context.Context) (*entity.SiteInformation, error) {
	site := s.site
	return &site, nil
}

func (s *stubSiteInformation) Update(context.Context, *entity.SiteInformation) error {
	return nil
}

func (s *stubSiteInformation) SetTheme(_ context.Context, theme string) error {
	s.site.Theme = &theme
	return nil
}

func (s *stubSiteInformation) SetRobotsExtra(_ context.Context, rules string) error {
	s.site.RobotsExtra = rules
	return nil
}

func (s *stubSiteInformation) SetLanguages(context.Context, []entity.SiteLanguage) error { return nil }

func (s *stubSiteInformation) SetSchemaOrg(context.Context, entity.SiteSchemaOrg) error { return nil }
