package views

import (
	"context"
	"errors"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

// TestResolveGlossaryTarget - the nil case only (no
// entity.ContentTypeRepository needed): nil is always valid and means
// Node.Title.
func TestResolveGlossaryTarget(t *testing.T) {
	s := &DefaultService{}
	ctx := t.Context()
	if err := s.resolveGlossaryTarget(ctx, entity.ViewDisplayDef{}); err != nil {
		t.Fatalf("nil GlossaryFieldStorageID must always be valid, got %v", err)
	}
}

// TestCreateView_BlockDisplayHasNoURLPath - a block has no URL of its own:
// url_path is never set (explicitly or generated) and isn't checked for
// duplicates.
func TestCreateView_BlockDisplayHasNoURLPath(t *testing.T) {
	repo := newFakeViewRepo()
	s := newTestViewService(repo)

	created, err := s.CreateView(t.Context(), entity.ViewDisplayDef{
		Label:       "Popular today",
		DisplayType: entity.ViewDisplayTypeBlock,
	})
	if err != nil {
		t.Fatalf("unexpected error creating a block display: %v", err)
	}
	if created.URLPath != "" {
		t.Fatalf("got URLPath %q, want empty for a block display", created.URLPath)
	}

	// A second block with the same (empty) URLPath is not a conflict (the
	// DB allows it via NULL + UNIQUE; the service must not treat empty
	// paths as duplicates).
	second, err := s.CreateView(t.Context(), entity.ViewDisplayDef{
		Label:       "Another block",
		DisplayType: entity.ViewDisplayTypeBlock,
	})
	if err != nil {
		t.Fatalf("unexpected error creating a second block display: %v", err)
	}
	if second.URLPath != "" {
		t.Fatalf("got URLPath %q, want empty for a second block display", second.URLPath)
	}
}

// TestUpdateView_SwitchingToBlockClearsURLPath - switching page→block
// clears URLPath even if the caller (admin.Controller.ViewUpdate) passes a
// value (the form keeps it in a hidden but enabled input).
func TestUpdateView_SwitchingToBlockClearsURLPath(t *testing.T) {
	repo := newFakeViewRepo()
	repo.displays[1] = entity.ViewDisplayDef{ID: 1, DisplayType: entity.ViewDisplayTypePage, URLPath: "videos", Label: "Videos"}
	s := newTestViewService(repo)

	updated, err := s.UpdateView(t.Context(), entity.ViewDisplayDef{
		ID:          1,
		Label:       "Videos",
		DisplayType: entity.ViewDisplayTypeBlock,
		URLPath:     "videos", // as if the hidden input still sent the old value
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.URLPath != "" {
		t.Fatalf("got URLPath %q after switching to block, want empty", updated.URLPath)
	}
}

// TestAttachmentDisplayHasNoURLPath - an attachment, like a block, has no
// URL of its own: URLPath is cleared on create and on page→attachment.
func TestAttachmentDisplayHasNoURLPath(t *testing.T) {
	repo := newFakeViewRepo()
	repo.displays[1] = entity.ViewDisplayDef{ID: 1, DisplayType: entity.ViewDisplayTypePage, URLPath: "videos", Label: "Videos"}
	s := newTestViewService(repo)

	created, err := s.CreateView(t.Context(), entity.ViewDisplayDef{
		Label:       "Similar",
		DisplayType: entity.ViewDisplayTypeAttachment,
		URLPath:     "similar",
	})
	if err != nil {
		t.Fatalf("unexpected error creating an attachment display: %v", err)
	}
	if created.URLPath != "" {
		t.Fatalf("got URLPath %q, want empty for an attachment display", created.URLPath)
	}

	updated, err := s.UpdateView(t.Context(), entity.ViewDisplayDef{
		ID:                 1,
		Label:              "Videos",
		DisplayType:        entity.ViewDisplayTypeAttachment,
		URLPath:            "videos",
		AttachContentTypes: "article,group",
		AttachWeight:       5,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.URLPath != "" || updated.AttachContentTypes != "article,group" || updated.AttachWeight != 5 {
		t.Fatalf("got URLPath=%q attach=%q weight=%d", updated.URLPath, updated.AttachContentTypes, updated.AttachWeight)
	}
}

func TestCheckProfileTab(t *testing.T) {
	repo := &fakeViewRepo{displays: map[int]entity.ViewDisplayDef{
		1: {ID: 1, DisplayType: entity.ViewDisplayTypeUserTab, ProfileTabPath: "content"},
	}}
	s := newTestViewService(repo)
	ctx := context.Background()
	// A module's profile tab (interfaces.AddProfileTab).
	entity.ReserveProfileTabPath("inbox")
	t.Cleanup(entity.ResetProfileTabPaths)
	for _, tc := range []struct {
		path string
		want error
	}{
		{"moderation", nil},
		{"/Moderation/", nil}, // trimmed and lowercased
		{"content", ErrProfileTabPathTaken},
		{"inbox", ErrProfileTabPathInvalid},
		{"edit", ErrProfileTabPathInvalid},
		{"files", ErrProfileTabPathInvalid},
		{"", ErrProfileTabPathInvalid},
		{"мои", ErrProfileTabPathInvalid}, // Cyrillic on purpose: non-Latin paths are rejected
		{"a b", ErrProfileTabPathInvalid},
	} {
		v := entity.ViewDisplayDef{ID: 2, DisplayType: entity.ViewDisplayTypeUserTab, ProfileTabPath: tc.path}
		if err := s.checkProfileTab(ctx, &v); !errors.Is(err, tc.want) {
			t.Errorf("%q: %v, want %v", tc.path, err, tc.want)
		}
		if tc.want == nil && (v.ProfileTabPath != "moderation" || v.ProfileTabAccess != entity.ViewProfileTabAccessAll) {
			t.Errorf("%q: normalized to %q/%q", tc.path, v.ProfileTabPath, v.ProfileTabAccess)
		}
	}
	// The display's own path isn't a conflict; other types keep no path.
	own := entity.ViewDisplayDef{ID: 1, DisplayType: entity.ViewDisplayTypeUserTab, ProfileTabPath: "content", ProfileTabAccess: entity.ViewProfileTabAccessOwner}
	if err := s.checkProfileTab(ctx, &own); err != nil || own.ProfileTabAccess != entity.ViewProfileTabAccessOwner {
		t.Errorf("own path: %v, access %q", err, own.ProfileTabAccess)
	}
	page := entity.ViewDisplayDef{DisplayType: entity.ViewDisplayTypePage, ProfileTabPath: "x"}
	if err := s.checkProfileTab(ctx, &page); err != nil || page.ProfileTabPath != "" {
		t.Errorf("page display: %v, path %q", err, page.ProfileTabPath)
	}
}

func TestUpdateViewThemeTemplateName(t *testing.T) {
	for name, ok := range map[string]bool{"profile_cards": true, "editorial2": true, "Bad-Name": false, "a b": false, "_x": false} {
		if got := themeTemplateRe.MatchString(name); got != ok {
			t.Errorf("%q: %v, want %v", name, got, ok)
		}
	}
}

// TestNewPagePathOwner - a page display created can't take an address
// something else answers; one added or copied gets a free one.
func TestNewPagePathOwner(t *testing.T) {
	repo := newFakeViewRepo()
	s := newTestViewService(repo)
	s.pathOwner = func(_ context.Context, path string) (string, error) {
		return map[string]string{"login": "routes", "videos-page": "content", "videos-copy": "content"}[path], nil
	}
	if _, err := s.CreateView(t.Context(), entity.ViewDisplayDef{Label: "Sign in", DisplayType: entity.ViewDisplayTypePage, URLPath: "login"}); !errors.Is(err, ErrURLPathTaken) {
		t.Errorf("a route's address: %v, want ErrURLPathTaken", err)
	}
	for _, path := range []string{"videos-page", "videos-copy"} {
		if taken, err := s.pageURLTaken(t.Context(), path); err != nil || !taken {
			t.Errorf("%s: %v %v, want taken", path, taken, err)
		}
	}
	if got, err := s.uniqueViewSlug(t.Context(), "videos-page", s.pageURLTaken); err != nil || got != "videos-page-2" {
		t.Errorf("a free address: %q %v", got, err)
	}
}

func TestCheckURLArgument(t *testing.T) {
	for path, ok := range map[string]bool{"post": true, "posts/%": true, "%/post": true, "posts/%/%": false, "posts/a%": false} {
		if err := checkURLArgument(path); (err == nil) != ok {
			t.Errorf("%q: %v", path, err)
		}
	}
}

// TestUpdateViewPathOwner - a page display can't move to an address
// something else on the site answers; the address it has already, and
// one with a "%" argument, aren't checked.
func TestUpdateViewPathOwner(t *testing.T) {
	repo := newFakeViewRepo()
	repo.displays[1] = entity.ViewDisplayDef{ID: 1, DisplayType: entity.ViewDisplayTypePage, URLPath: "rss.xml", Label: "Videos"}
	s := newTestViewService(repo)
	s.pathOwner = func(_ context.Context, path string) (string, error) {
		return map[string]string{"rss.xml": "routes", "about": "content"}[path], nil
	}
	update := func(path string) error {
		_, err := s.UpdateView(t.Context(), entity.ViewDisplayDef{ID: 1, Label: "Videos", DisplayType: entity.ViewDisplayTypePage, URLPath: path})
		return err
	}
	if err := update("about"); !errors.Is(err, ErrURLPathTaken) {
		t.Errorf("content's address: %v, want ErrURLPathTaken", err)
	}
	if err := update("rss.xml"); err != nil {
		t.Errorf("its own address, unchanged: %v", err)
	}
	for _, free := range []string{"videos", "about/%"} {
		if err := update(free); err != nil {
			t.Errorf("%s: %v", free, err)
		}
	}
}
