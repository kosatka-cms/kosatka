package interfaces

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/service"
)

// A theme template missing from the theme falls back to the View's own
// field grid (ok=false), without building anything.
func TestRenderThemeViewListMissingFile(t *testing.T) {
	root, _ := filepath.Abs(filepath.Join("..", ".."))
	t.Chdir(root)
	c := &DynamicContentController{teasers: &service.TeaserBuilder{}}
	if _, ok, err := c.renderThemeViewList(context.Background(), "no_such_template", nil, nil); ok || err != nil {
		t.Fatalf("missing theme template: ok=%v err=%v", ok, err)
	}
	c.teasers = nil
	if _, ok, _ := c.renderThemeViewList(context.Background(), "profile_cards", nil, nil); ok {
		t.Fatal("no teaser builder must fall back too")
	}
}
