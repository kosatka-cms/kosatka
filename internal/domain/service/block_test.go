package service

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

func TestMatchesVisibilityPages(t *testing.T) {
	tests := []struct {
		name       string
		path       string
		pagesText  string
		mode       entity.BlockVisibilityPagesMode
		wantVisble bool
	}{
		{"exact match, show", "news", "news", entity.BlockVisibilityPagesShow, true},
		{"exact mismatch, show", "forum", "news", entity.BlockVisibilityPagesShow, false},
		{"wildcard matches nested path", "articles/foo", "articles/*", entity.BlockVisibilityPagesShow, true},
		{"wildcard does not match the bare prefix itself", "articles", "articles/*", entity.BlockVisibilityPagesShow, false},
		{"<front> matches only the homepage", "", "<front>", entity.BlockVisibilityPagesShow, true},
		{"<front> does not match a non-homepage path", "news", "<front>", entity.BlockVisibilityPagesShow, false},
		{"hide mode inverts a match", "news", "news", entity.BlockVisibilityPagesHide, false},
		{"hide mode inverts a non-match", "forum", "news", entity.BlockVisibilityPagesHide, true},
		{"empty pagesText, show mode: nothing visible", "news", "", entity.BlockVisibilityPagesShow, false},
		{"empty pagesText, hide mode: visible everywhere", "news", "", entity.BlockVisibilityPagesHide, true},
		{"blank line among patterns is skipped, not a panic", "news", "\n\nnews\n", entity.BlockVisibilityPagesShow, true},
		{"leading/trailing slashes on both sides don't block a match", "/articles/foo/", "/articles/*/", entity.BlockVisibilityPagesShow, true},
		{"multiple lines, second one matches", "forum", "news\nforum\nevents", entity.BlockVisibilityPagesShow, true},
		{"star in the middle", "news/2026/harbour", "news/*/harbour", entity.BlockVisibilityPagesShow, true},
		{"star in the middle, other end", "news/2026/market", "news/*/harbour", entity.BlockVisibilityPagesShow, false},
		{"two stars", "shop/cat/item-12/reviews", "shop/*/item-*/reviews", entity.BlockVisibilityPagesShow, true},
		{"leading star", "about/team", "*/team", entity.BlockVisibilityPagesShow, true},
		{"a dot is literal", "newsXhtml", "news.html", entity.BlockVisibilityPagesShow, false},
		{"the suffix doesn't reuse the prefix", "ab", "ab*b", entity.BlockVisibilityPagesShow, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MatchesVisibilityPages(tt.path, tt.pagesText, tt.mode)
			if got != tt.wantVisble {
				t.Fatalf("MatchesVisibilityPages(%q, %q, %q) = %v, want %v", tt.path, tt.pagesText, tt.mode, got, tt.wantVisble)
			}
		})
	}
}

type themeBlocksRepo struct {
	entity.BlockRepository
	byTheme map[string][]entity.BlockWithPlacement
	copied  []entity.BlockPlacementDef
}

func (r *themeBlocksRepo) ThemeHasPlacements(_ context.Context, theme string) (bool, error) {
	return len(r.byTheme[theme]) > 0, nil
}

func (r *themeBlocksRepo) Blocks(_ context.Context, theme string) ([]entity.BlockWithPlacement, error) {
	return r.byTheme[theme], nil
}

func (r *themeBlocksRepo) CopyPlacement(_ context.Context, p entity.BlockPlacementDef) (*entity.BlockPlacementDef, error) {
	r.copied = append(r.copied, p)
	return &p, nil
}

type noTxUOW struct{}

func (noTxUOW) Do(ctx context.Context, f func(context.Context) error) error { return f(ctx) }

// TestInitThemePlacements - a theme without placements gets a copy of the
// source theme's in the regions it shares (and the hidden ones); a theme
// with its own layout is left alone.
func TestInitThemePlacements(t *testing.T) {
	blockID := 3
	repo := &themeBlocksRepo{byTheme: map[string][]entity.BlockWithPlacement{
		"old": {
			{Placement: entity.BlockPlacementDef{ID: 1, Theme: "old", Region: "sidebar", BlockID: &blockID}},
			{Placement: entity.BlockPlacementDef{ID: 2, Theme: "old", Region: "old_only"}},
			{Placement: entity.BlockPlacementDef{ID: 3, Theme: "old", Region: entity.RegionDisabled}},
		},
		"busy": {{Placement: entity.BlockPlacementDef{ID: 9, Theme: "busy", Region: "sidebar"}}},
	}}
	s := NewBlockService(repo, noTxUOW{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	regions := entity.ThemeInfo{Regions: []entity.ThemeRegion{{ID: "sidebar"}, {ID: "footer"}}}

	done, err := s.InitThemePlacements(t.Context(), "new", "old", regions)
	if err != nil || !done {
		t.Fatalf("init: %v %v", done, err)
	}
	if len(repo.copied) != 2 || repo.copied[0].Theme != "new" || repo.copied[0].BlockID == nil || repo.copied[1].Region != entity.RegionDisabled {
		t.Fatalf("copied %+v", repo.copied)
	}

	repo.copied = nil
	if done, _ := s.InitThemePlacements(t.Context(), "busy", "old", regions); done || len(repo.copied) != 0 {
		t.Errorf("a theme with its own layout was overwritten: %+v", repo.copied)
	}
	if done, _ := s.InitThemePlacements(t.Context(), "old", "old", regions); done {
		t.Error("copied a theme onto itself")
	}
}

func TestRegionHasVisibleBlocks(t *testing.T) {
	r := entity.Region("hero_second")
	place := func(p entity.BlockPlacementDef) map[entity.Region][]entity.BlockWithPlacement {
		return map[entity.Region][]entity.BlockWithPlacement{r: {{Placement: p}}}
	}
	item := entity.DynamicContent{Node: entity.Node{Type: "article"}}
	for _, tc := range []struct {
		name string
		ctx  map[string]any
		want bool
	}{
		{"empty region", map[string]any{"blockPlacements": map[entity.Region][]entity.BlockWithPlacement{}}, false},
		{"no data", nil, false},
		{"demo mode shows empty regions", map[string]any{"blockDemoMode": true}, true},
		{"all", map[string]any{"blockPlacements": place(entity.BlockPlacementDef{VisibilityScope: entity.BlockVisibilityAll})}, true},
		{"homepage on home", map[string]any{"isHomepage": true, "blockPlacements": place(entity.BlockPlacementDef{VisibilityScope: entity.BlockVisibilityHomepage})}, true},
		{"homepage elsewhere", map[string]any{"blockPlacements": place(entity.BlockPlacementDef{VisibilityScope: entity.BlockVisibilityHomepage})}, false},
		{"content type match", map[string]any{"item": item, "blockPlacements": place(entity.BlockPlacementDef{VisibilityScope: entity.BlockVisibilityContentType, VisibilityContentType: strPtr("video,article")})}, true},
		{"content type other", map[string]any{"item": &item, "blockPlacements": place(entity.BlockPlacementDef{VisibilityScope: entity.BlockVisibilityContentType, VisibilityContentType: strPtr("video")})}, false},
		{"content type, no node", map[string]any{"blockPlacements": place(entity.BlockPlacementDef{VisibilityScope: entity.BlockVisibilityContentType, VisibilityContentType: strPtr("article")})}, false},
		{"roles: moderator", map[string]any{"currentRoles": []string{"user", "moderator"}, "blockPlacements": place(entity.BlockPlacementDef{VisibilityScope: entity.BlockVisibilityAll, VisibilityRoles: "moderator,admin"})}, true},
		{"roles: plain user", map[string]any{"currentRoles": []string{"user"}, "blockPlacements": place(entity.BlockPlacementDef{VisibilityScope: entity.BlockVisibilityAll, VisibilityRoles: "moderator,admin"})}, false},
		{"roles: guest", map[string]any{"currentRoles": []string{"anonymous"}, "blockPlacements": place(entity.BlockPlacementDef{VisibilityScope: entity.BlockVisibilityAll, VisibilityRoles: "anonymous"})}, true},
		{"roles and scope", map[string]any{"currentRoles": []string{"user", "admin"}, "blockPlacements": place(entity.BlockPlacementDef{VisibilityScope: entity.BlockVisibilityHomepage, VisibilityRoles: "admin"})}, false},
		{"pages match", map[string]any{"currentPath": "/videos", "blockPlacements": place(entity.BlockPlacementDef{VisibilityScope: entity.BlockVisibilityPages, VisibilityPages: "/videos", VisibilityPagesMode: entity.BlockVisibilityPagesShow})}, true},
	} {
		if got := RegionHasVisibleBlocks(r, tc.ctx); got != tc.want {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
}

// TestHasVisibleCarouselBlock - the Swiper stylesheet is needed when a
// carousel View placement is visible on the page, not when it is hidden
// there or when the visible blocks aren't carousels.
func TestHasVisibleCarouselBlock(t *testing.T) {
	placements := map[entity.Region][]entity.BlockWithPlacement{
		entity.Region("home_slider_main"): {{Placement: entity.BlockPlacementDef{ID: 7, VisibilityScope: entity.BlockVisibilityHomepage}}},
		entity.Region("sidebar"):          {{Placement: entity.BlockPlacementDef{ID: 8, VisibilityScope: entity.BlockVisibilityAll}}},
	}
	ctx := func(home bool, carousels map[int]bool) map[string]any {
		return map[string]any{"blockPlacements": placements, "carouselPlacements": carousels, "isHomepage": home}
	}
	if !HasVisibleCarouselBlock(ctx(true, map[int]bool{7: true})) {
		t.Fatal("homepage carousel on the homepage")
	}
	if HasVisibleCarouselBlock(ctx(false, map[int]bool{7: true})) {
		t.Fatal("homepage-only carousel on another page")
	}
	if HasVisibleCarouselBlock(ctx(true, nil)) {
		t.Fatal("no carousels")
	}
	if !HasVisibleCarouselBlock(ctx(false, map[int]bool{8: true})) {
		t.Fatal("sitewide carousel")
	}
}
