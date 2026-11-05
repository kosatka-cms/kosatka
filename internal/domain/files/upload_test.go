package files

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"io"
	"log/slog"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gen2brain/webp"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/pkg/filestore"
)

// TestUploadSameSecond - two images uploaded within a second get names of
// their own (the second _1), avatars too; neither is overwritten.
func TestUploadSameSecond(t *testing.T) {
	svc := NewFileUploadService(filestore.NewLocal(t.TempDir()), fakeUploadSettingsRepo{}, fakeAccountSettingsRepo{}, fakeTextFormatRepo{}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	var enc bytes.Buffer
	if err := png.Encode(&enc, image.NewRGBA(image.Rect(0, 0, 4, 4))); err != nil {
		t.Fatal(err)
	}
	upload := func() string {
		header := &multipart.FileHeader{Filename: "a.png", Size: int64(enc.Len())}
		uri, _, _, _, _, err := svc.Upload(context.Background(), readSeekCloser{Reader: bytes.NewReader(enc.Bytes())}, header, "galleries/1")
		if err != nil {
			t.Fatal(err)
		}
		return uri
	}
	avatar := func() string {
		header := &multipart.FileHeader{Filename: "me.png", Size: int64(enc.Len())}
		uri, err := svc.UploadUserAvatar(context.Background(), readSeekCloser{Reader: bytes.NewReader(enc.Bytes())}, header, 7)
		if err != nil {
			t.Fatal(err)
		}
		return uri
	}
	if a, b := upload(), upload(); a == b {
		t.Errorf("two uploads share %s", a)
	}
	if a, b := avatar(), avatar(); a == b {
		t.Errorf("two avatars share %s", a)
	}
}

// fakeUploadSettingsRepo returns a fixed upload policy (2000px resize
// threshold, jpeg/png/gif/webp allowed) the test expectations rely on.
type fakeUploadSettingsRepo struct{}

func (fakeUploadSettingsRepo) Get(context.Context) (*entity.UploadSettings, error) {
	return &entity.UploadSettings{
		MaxFileSizeBytes:   5 * 1024 * 1024,
		MaxImageDimension:  2000,
		AllowedMimeTypes:   []string{"image/jpeg", "image/png", "image/gif", "image/webp"},
		UserFileExtensions: []string{"pdf", "txt"},
	}, nil
}

func (fakeUploadSettingsRepo) Update(context.Context, *entity.UploadSettings) error {
	return nil
}

type fakeAccountSettingsRepo struct{}

func (fakeAccountSettingsRepo) Get(context.Context) (*entity.AccountSettings, error) {
	return &entity.AccountSettings{}, nil
}

func (fakeAccountSettingsRepo) Update(context.Context, *entity.AccountSettings) error {
	return nil
}

type fakeTextFormatRepo struct{}

func (fakeTextFormatRepo) TextFormats(context.Context) (entity.TextFormatDefs, error) {
	return nil, nil
}

func (fakeTextFormatRepo) TextFormatByID(context.Context, int) (*entity.TextFormatDef, error) {
	return nil, entity.ErrEntityNotFound
}

func (fakeTextFormatRepo) TextFormatByMachineName(context.Context, string) (*entity.TextFormatDef, error) {
	return nil, entity.ErrEntityNotFound
}

func (fakeTextFormatRepo) CreateTextFormat(context.Context, entity.TextFormatDef) (*entity.TextFormatDef, error) {
	return nil, nil
}

func (fakeTextFormatRepo) UpdateTextFormat(context.Context, entity.TextFormatDef) (*entity.TextFormatDef, error) {
	return nil, nil
}

func (fakeTextFormatRepo) DeleteTextFormat(context.Context, int) error {
	return nil
}

func TestValidateAndProcess_resizeWebPSavesAsJPEG(t *testing.T) {
	base := t.TempDir()
	svc := NewFileUploadService(filestore.NewLocal(base), fakeUploadSettingsRepo{}, fakeAccountSettingsRepo{}, fakeTextFormatRepo{}, slog.New(slog.NewTextHandler(io.Discard, nil)))

	img := image.NewRGBA(image.Rect(0, 0, 1365, 2048))
	var enc bytes.Buffer
	if err := webp.Encode(&enc, img, webp.Options{Lossless: true}); err != nil {
		t.Fatal(err)
	}

	targetPath := "galleries/1/test.webp"
	body := readSeekCloser{Reader: bytes.NewReader(enc.Bytes())}
	header := &multipart.FileHeader{
		Filename: "test.webp",
		Size:     int64(enc.Len()),
		Header:   map[string][]string{"Content-Type": {"image/webp"}},
	}

	result, err := svc.ValidateAndProcess(context.Background(), body, header, targetPath)
	if err != nil {
		t.Fatal(err)
	}
	if result.MimeType != "image/jpeg" {
		t.Fatalf("mime: got %q want image/jpeg", result.MimeType)
	}
	if result.Height > 2000 {
		t.Fatalf("height should be resized: %d", result.Height)
	}

	jpgPath := filepath.Join(base, "galleries", "1", "test.jpg")
	if _, err := os.Stat(jpgPath); err != nil {
		t.Fatalf("expected %s on disk: %v", jpgPath, err)
	}
	if _, err := os.Stat(filepath.Join(base, targetPath)); err == nil {
		t.Fatal("original .webp path should not exist when saved as jpeg")
	}
}

func TestEnsureJPEGExtension(t *testing.T) {
	if got := ensureJPEGExtension("tmp/foo.webp"); got != "tmp/foo.jpg" {
		t.Fatalf("got %q", got)
	}
	if got := ensureJPEGExtension("tmp/foo.jpg"); got != "tmp/foo.jpg" {
		t.Fatalf("got %q", got)
	}
}

type readSeekCloser struct {
	*bytes.Reader
}

func (readSeekCloser) Close() error { return nil }

func TestValidateAndProcess_smallWebPUnchanged(t *testing.T) {
	base := t.TempDir()
	svc := NewFileUploadService(filestore.NewLocal(base), fakeUploadSettingsRepo{}, fakeAccountSettingsRepo{}, fakeTextFormatRepo{}, slog.New(slog.NewTextHandler(io.Discard, nil)))

	img := image.NewNRGBA(image.Rect(0, 0, 100, 100))
	var enc bytes.Buffer
	if err := png.Encode(&enc, img); err != nil {
		t.Fatal(err)
	}

	targetPath := "small.png"
	body := readSeekCloser{Reader: bytes.NewReader(enc.Bytes())}
	header := &multipart.FileHeader{
		Filename: "small.png",
		Size:     int64(enc.Len()),
		Header:   map[string][]string{"Content-Type": {"image/png"}},
	}

	result, err := svc.ValidateAndProcess(context.Background(), body, header, targetPath)
	if err != nil {
		t.Fatal(err)
	}
	if result.MimeType != "image/png" {
		t.Fatalf("mime: %q", result.MimeType)
	}
}

func newTestUploadService(t *testing.T) (*FileUploadService, string) {
	t.Helper()
	base := t.TempDir()
	return NewFileUploadService(filestore.NewLocal(base), fakeUploadSettingsRepo{}, fakeAccountSettingsRepo{}, fakeTextFormatRepo{}, slog.New(slog.NewTextHandler(io.Discard, nil))), base
}

func pngUpload(t *testing.T, name string) (multipart.File, *multipart.FileHeader) {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 10, 10))); err != nil {
		t.Fatal(err)
	}
	return readSeekCloser{Reader: bytes.NewReader(buf.Bytes())}, &multipart.FileHeader{
		Filename: name, Size: int64(buf.Len()), Header: map[string][]string{"Content-Type": {"image/png"}},
	}
}

func TestPickUniqueFilenameChecksAltExtensions(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "shot.jpg"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	// shot.png is free, but a re-encode would land on the taken shot.jpg.
	if got, err := pickUniqueFilename(context.Background(), filestore.NewLocal(dir), "", "shot", ".png", ".jpg"); err != nil || got != "shot_1.png" {
		t.Fatalf("got %q, %v; want shot_1.png", got, err)
	}
}

func TestPickUniqueImageFilename(t *testing.T) {
	dir := t.TempDir()
	name, err := pickUniqueFilename(context.Background(), filestore.NewLocal(dir), "", "shot", ".jpg")
	if err != nil || name != "shot.jpg" {
		t.Fatalf("first: %q %v", name, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "shot.jpg"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	name, err = pickUniqueFilename(context.Background(), filestore.NewLocal(dir), "", "shot", ".jpg")
	if err != nil || name != "shot_1.jpg" {
		t.Fatalf("second: %q %v", name, err)
	}
}

func TestRemoveStoredFile(t *testing.T) {
	base := t.TempDir()
	write := func(rel string) string {
		p := filepath.Join(base, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	orig := write("news/[a].jpg")
	derivative := write("styles/thumb/news/[a].jpg")
	webpDerivative := write("styles/news_list/news/[a].webp")
	// The style's 2x one (imagestyle.DoublePath).
	variant := write("styles/news_list/@2x/news/[a].webp")
	other := write("news/b.jpg")
	otherVariant := write("styles/news_list/@2x/news/b.webp")
	outside := write("../outside.jpg")
	// A style's derivatives may be missing.
	if err := os.MkdirAll(filepath.Join(base, "styles", "empty"), 0o755); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	s := &FileUploadService{store: filestore.NewLocal(base)}
	if err := s.RemoveStoredFile(ctx, "public://news/[a].jpg"); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{orig, derivative, webpDerivative, variant} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Fatalf("%s should be removed", p)
		}
	}
	// Only the derivatives: the file stays.
	kept := write("news/c.jpg")
	keptDerivative := write("styles/thumb/news/c.jpg")
	keptDouble := write("styles/news_list/@2x/news/c.webp")
	if err := s.RemoveDerivatives(ctx, "public://news/c.jpg"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(kept); err != nil {
		t.Fatalf("RemoveDerivatives removed the file: %v", err)
	}
	for _, p := range []string{keptDerivative, keptDouble} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Fatalf("%s should be removed", p)
		}
	}
	_ = s.RemoveStoredFile(ctx, "public://../outside.jpg")
	_ = s.RemoveStoredFile(ctx, "uploads/news/b.jpg")
	for _, p := range []string{other, otherVariant, outside} {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("%s must be kept: %v", p, err)
		}
	}
}

// bigPNG - a PNG of w×h as an upload named name.
func bigPNG(t *testing.T, name string, w, h int) (multipart.File, *multipart.FileHeader) {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, w, h))); err != nil {
		t.Fatal(err)
	}
	return readSeekCloser{Reader: bytes.NewReader(buf.Bytes())}, &multipart.FileHeader{
		Filename: name, Size: int64(buf.Len()), Header: map[string][]string{"Content-Type": {"image/png"}},
	}
}

// TestSitewideMaxImageDimension - an image larger than the sitewide
// maximum (2000 in the fake policy) is scaled down when the upload has no
// size of its own: an editor's image (a text format sets none) and a
// module's (UploadImage with an empty policy).
func TestSitewideMaxImageDimension(t *testing.T) {
	svc, _ := newTestUploadService(t)
	ctx := context.Background()
	file, header := bigPNG(t, "editor.png", 3000, 1500)
	res, err := svc.ValidateAndProcessTextFormatImage(ctx, file, header, "u1/editor.png", 0)
	if err != nil || res.Width != 2000 || res.Height != 1000 {
		t.Errorf("an editor's image: %+v %v, want 2000×1000", res, err)
	}
	file, header = bigPNG(t, "photo.png", 1500, 3000)
	_, res, err = svc.UploadImage(ctx, file, header, "galleries/1", entity.FieldUploadPolicy{})
	if err != nil || res.Width != 1000 || res.Height != 2000 {
		t.Errorf("a module's image: %+v %v, want 1000×2000", res, err)
	}
}

// TestUploadUserAvatar - an avatar is stored as JPEG under user_pictures,
// scaled down to MaxAvatarDimension.
func TestUploadUserAvatar(t *testing.T) {
	svc, base := newTestUploadService(t)
	file, header := bigPNG(t, "me.png", 600, 400)
	uri, err := svc.UploadUserAvatar(t.Context(), file, header, 7)
	if err != nil {
		t.Fatal(err)
	}
	rel, ok := strings.CutPrefix(uri, "public://"+UserAvatarBasePath+"/7-")
	if !ok || !strings.HasSuffix(rel, ".jpg") {
		t.Fatalf("uri = %q", uri)
	}
	f, err := os.Open(filepath.Join(base, strings.TrimPrefix(uri, "public://")))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	cfg, format, err := image.DecodeConfig(f)
	if err != nil || format != "jpeg" || cfg.Width != MaxAvatarDimension || cfg.Height != 200 {
		t.Errorf("stored avatar: %s %dx%d %v", format, cfg.Width, cfg.Height, err)
	}
}
