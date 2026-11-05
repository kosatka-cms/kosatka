package imageurl

import (
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/paths"
	"github.com/kosatka-cms/kosatka/pkg/filestore"
	"github.com/kosatka-cms/kosatka/pkg/imagestyle"
)

func TestStyledUsesWebP(t *testing.T) {
	got := Styled(entity.File{URI: "public://news/pack001/banner.jpg"}, "news_list")
	if !strings.HasSuffix(got.URI, "banner.webp") {
		t.Fatalf("expected .webp URI, got %q", got.URI)
	}
	if !strings.Contains(got.URI, "/styles/news_list/") {
		t.Fatalf("expected style in path, got %q", got.URI)
	}
}

func TestOriginal(t *testing.T) {
	const want = "/uploads/articles/foo.jpg"
	cases := map[string]string{
		"public:// scheme":                        "public://articles/foo.jpg",
		"legacy absolute path (already resolved)": want,
		"legacy path without leading slash":       "uploads/articles/foo.jpg",
	}
	for name, uri := range cases {
		t.Run(name, func(t *testing.T) {
			got := Original(entity.File{URI: uri}).URI
			if got != want {
				t.Fatalf("Original(%q) = %q, want %q", uri, got, want)
			}
		})
	}
}

func TestOriginalEmpty(t *testing.T) {
	got := Original(entity.File{})
	if got.URI != "" {
		t.Fatalf("expected empty URI for empty input, got %q", got.URI)
	}
}

func TestPublicURI(t *testing.T) {
	tests := []struct {
		name string
		uri  string
		want string
	}{
		{"empty stays empty", "", ""},
		{"public:// converts to a servable path", "public://site/logo/1-123.png", "/uploads/site/logo/1-123.png"},
		// Unlike Original, a value not under public:// (a seeded
		// static default, see entity.SiteInformation.Logo) is unchanged.
		{"non-public:// path passes through unchanged", "/static/images/logo.png", "/static/images/logo.png"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := PublicURI(tt.uri); got != tt.want {
				t.Fatalf("PublicURI(%q) = %q, want %q", tt.uri, got, tt.want)
			}
		})
	}
}

// TestStyledFormat - a JPEG style's derivative URL ends in
// .jpg, every other style's in .webp; derivative cleanup covers both.
func TestStyledFormat(t *testing.T) {
	SetStyleFormats(map[string]imagestyle.Format{"feed_jpeg": imagestyle.FormatJPEG})
	t.Cleanup(func() { SetStyleFormats(nil) })

	img := entity.File{URI: "public://images/foo.png"}
	if got := Styled(img, "feed_jpeg").URI; got != "/uploads/styles/feed_jpeg/images/foo.jpg" {
		t.Errorf("jpeg style: %s", got)
	}
	if got := Styled(img, "other").URI; got != "/uploads/styles/other/images/foo.webp" {
		t.Errorf("webp style: %s", got)
	}
	if got := Styled(entity.File{}, "feed_jpeg").URI; got != "" {
		t.Errorf("placeholder: %s", got)
	}

	SetStyleFormat("feed_jpeg", imagestyle.FormatWebP)
	if got := Styled(img, "feed_jpeg").URI; got != "/uploads/styles/feed_jpeg/images/foo.webp" {
		t.Errorf("after switching back to webp: %s", got)
	}

	paths := DerivativeRelPaths("images/foo.png")
	if len(paths) != 2 || paths[0] != "images/foo.webp" || paths[1] != "images/foo.jpg" {
		t.Errorf("derivative paths: %v", paths)
	}
}

// TestAvatarPlaceholder - the engine's avatar placeholder is a
// static asset, kept as is; an uploaded avatar gets the style.
func TestAvatarPlaceholder(t *testing.T) {
	if got := Avatar(entity.File{URI: entity.AvatarPlaceholderURL}, AvatarProfileStyle); got.URI != entity.AvatarPlaceholderURL {
		t.Errorf("placeholder: %q", got.URI)
	}
	got := Avatar(entity.File{URI: "public://pictures/a.jpg"}, AvatarProfileStyle)
	if !strings.Contains(got.URI, "/styles/"+AvatarProfileStyle+"/") {
		t.Errorf("uploaded avatar: %q", got.URI)
	}
}

// TestImagePathsCustomFilesURL - the public files URL from the config
// (paths.SetPublicFiles) is used for originals, styles and public:// URIs.
func TestImagePathsCustomFilesURL(t *testing.T) {
	paths.SetPublicFiles("", "/files")
	t.Cleanup(func() { paths.SetPublicFiles("", "") })
	SetStyleFormat("thumb", imagestyle.FormatWebP)
	img := entity.File{URI: "public://images/foo.jpg"}
	if got := Original(img).URI; got != "/files/images/foo.jpg" {
		t.Errorf("original: %s", got)
	}
	if got := Original(entity.File{URI: "/files/images/foo.jpg"}).URI; got != "/files/images/foo.jpg" {
		t.Errorf("original twice: %s", got)
	}
	if got := Styled(img, "thumb").URI; got != "/files/styles/thumb/images/foo.webp" {
		t.Errorf("styled: %s", got)
	}
	if got := Avatar(entity.File{URI: "/files/styles/other/u1/a.webp"}, "thumb").URI; got != "/files/styles/thumb/u1/a.webp" {
		t.Errorf("avatar: %s", got)
	}
	if got := PublicURI("public://logo.png"); got != "/files/logo.png" {
		t.Errorf("resolve: %s", got)
	}
}

// TestStyledVersion - a style with a version has it in its derivatives'
// URLs, one without none; Avatar restyles a versioned URI by its path.
func TestStyledVersion(t *testing.T) {
	SetStyleVersions(map[string]string{"cover": "1a2b3c4d", AvatarProfileStyle: "99aa00ff"})
	t.Cleanup(func() { SetStyleVersions(nil) })
	img := entity.File{URI: "public://images/foo.png"}
	if got := Styled(img, "cover").URI; got != "/uploads/styles/cover/images/foo.webp?v=1a2b3c4d" {
		t.Errorf("versioned style: %s", got)
	}
	if got := Styled(img, "other").URI; got != "/uploads/styles/other/images/foo.webp" {
		t.Errorf("style without a version: %s", got)
	}
	if got := Avatar(Styled(img, "cover"), AvatarProfileStyle).URI; got != "/uploads/styles/"+AvatarProfileStyle+"/images/foo.webp?v=99aa00ff" {
		t.Errorf("avatar from a versioned URI: %s", got)
	}
	SetStyleVersion("cover", "")
	if got := Styled(img, "cover").URI; strings.Contains(got, "?v=") {
		t.Errorf("version removed: %s", got)
	}
}

// TestStoreURLs - with a store serving its files itself, an original's
// address is the store's; a derivative's stays the site's (made on its
// first request); every built address maps back to the file.
func TestStoreURLs(t *testing.T) {
	paths.SetPublicStore(filestore.NewLocal(t.TempDir()), "https://cdn.example.com/blog/")
	t.Cleanup(func() { paths.SetPublicStore(nil, "") })

	orig := Original(entity.File{URI: "public://content/images/a.png"}).URI
	if orig != "https://cdn.example.com/blog/content/images/a.png" {
		t.Fatalf("Original = %q", orig)
	}
	if again := Original(entity.File{URI: orig}).URI; again != orig {
		t.Errorf("Original twice = %q", again)
	}
	if got := Original(entity.File{URI: "/uploads/content/images/a.png"}).URI; got != orig {
		t.Errorf("Original of a site path = %q", got)
	}
	if got := PublicURI("public://site/logo/l.png"); got != "https://cdn.example.com/blog/site/logo/l.png" {
		t.Errorf("PublicURI = %q", got)
	}
	if got := PublicURI("/static/logo.svg"); got != "/static/logo.svg" {
		t.Errorf("PublicURI of a static file = %q", got)
	}
	if got := Styled(entity.File{URI: orig}, "thumb").URI; got != "/uploads/styles/thumb/content/images/a.webp" {
		t.Errorf("Styled of the store's address = %q", got)
	}
	if got := Avatar(entity.File{URI: "https://cdn.example.com/blog/user_pictures/7.jpg"}, "small").URI; got != "/uploads/styles/small/user_pictures/7.webp" {
		t.Errorf("Avatar of the store's address = %q", got)
	}
}

// TestPrivateURLs - a private file's address is always the site's
// (PrivateFilesURL), with a store serving its public files itself too.
func TestPrivateURLs(t *testing.T) {
	paths.SetPublicStore(filestore.NewLocal(t.TempDir()), "https://cdn.example.com/")
	t.Cleanup(func() { paths.SetPublicStore(nil, "") })
	got := Original(entity.File{URI: "private://content/files/a.pdf"}).URI
	if got != "/files/private/content/files/a.pdf" {
		t.Fatalf("Original = %q", got)
	}
	if again := Original(entity.File{URI: got}).URI; again != got {
		t.Errorf("Original twice = %q", again)
	}
	if got := PublicURI("private://content/files/a.pdf"); got != "/files/private/content/files/a.pdf" {
		t.Errorf("PublicURI = %q", got)
	}
}

// TestPrivateStyled - a private image's style address keeps the
// original's key (the site checks access by it) and the style's version;
// restyling a built address doesn't nest it.
func TestPrivateStyled(t *testing.T) {
	SetStyleVersion("card", "ab12")
	t.Cleanup(func() { SetStyleVersion("card", "") })
	got := Styled(entity.File{URI: "private://content/images/a.png"}, "card").URI
	if got != "/files/private/styles/card/content/images/a.png?v=ab12" {
		t.Fatalf("Styled = %q", got)
	}
	for _, uri := range []string{got, "/files/private/content/images/a.png"} {
		if again := Styled(entity.File{URI: uri}, "thumb").URI; again != "/files/private/styles/thumb/content/images/a.png" {
			t.Errorf("Styled(%q) = %q", uri, again)
		}
	}
	if style, double, key, ok := ParsePrivateStyled("styles/card/content/images/a.png"); !ok || style != "card" || double || key != "content/images/a.png" {
		t.Errorf("parse: %q %q %v", style, key, ok)
	}
	for _, rel := range []string{"content/images/a.png", "styles/card", "styles//a.png"} {
		if _, _, _, ok := ParsePrivateStyled(rel); ok {
			t.Errorf("parse(%q) ok", rel)
		}
	}
}
