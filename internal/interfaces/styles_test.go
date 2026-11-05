package interfaces

import (
	"bytes"
	"errors"
	"image"
	"image/png"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/disintegration/imaging"
	"github.com/kosatka-cms/kosatka/internal/paths"
	"github.com/kosatka-cms/kosatka/pkg/filestore"
	"github.com/kosatka-cms/kosatka/pkg/imagestyle"
)

type fakeStyleRepo struct {
	imagestyle.ImageStyleRepository
	err error
}

func (f fakeStyleRepo) FindByName(string) (*imagestyle.ImageStyle, error) { return nil, f.err }

// TestServeStyledImageNotFoundIsQuiet - an unknown image style (stale URLs
// requested by bots) is a 404 logged below error level; any other failure
// is still logged as an error.
func TestServeStyledImageNotFoundIsQuiet(t *testing.T) {
	for _, tc := range []struct {
		err       error
		wantError bool
	}{
		{imagestyle.ErrStyleNotFound, false},
		{errors.New("db is down"), true},
	} {
		var logs bytes.Buffer
		c := NewStylesController(imagestyle.NewStyleService(fakeStyleRepo{err: tc.err}, nil, nil), slog.New(slog.NewTextHandler(&logs, nil)))
		req := httptest.NewRequest(http.MethodGet, "/uploads/styles/large/a.jpg", nil)
		req.SetPathValue("style", "large")
		req.SetPathValue("filepath", "a.jpg")
		rec := httptest.NewRecorder()
		c.ServeStyledImage(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Errorf("%v: status %d, want 404", tc.err, rec.Code)
		}
		if got := strings.Contains(logs.String(), "level=ERROR"); got != tc.wantError {
			t.Errorf("%v: error logged = %v, want %v (log: %q)", tc.err, got, tc.wantError, logs.String())
		}
	}
}

type oneStyleRepo struct {
	imagestyle.ImageStyleRepository
	style *imagestyle.ImageStyle
}

func (r oneStyleRepo) FindByName(string) (*imagestyle.ImageStyle, error) { return r.style, nil }

// TestServeStyledImageRedirectsToStore - with a store serving its files
// itself, the derivative is made, stored and the visitor sent there.
func TestServeStyledImageRedirectsToStore(t *testing.T) {
	dir := t.TempDir()
	store := filestore.NewLocal(dir)
	var src bytes.Buffer
	if err := png.Encode(&src, image.NewRGBA(image.Rect(0, 0, 40, 20))); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Put(t.Context(), "img/a.png", &src, filestore.PutOptions{}); err != nil {
		t.Fatal(err)
	}
	paths.SetPublicStore(store, "https://cdn.example.com/files/")
	t.Cleanup(func() { paths.SetPublicStore(nil, "") })
	style, err := imagestyle.NewImageStyle("small", "Small", 10, 10, imagestyle.ResizeModeFill, imaging.Center, 80)
	if err != nil {
		t.Fatal(err)
	}
	svc := imagestyle.NewStyleService(oneStyleRepo{style: style}, imagestyle.NewStorageRepository(store), imagestyle.NewImagingProcessor())
	c := NewStylesController(svc, slog.New(slog.NewTextHandler(io.Discard, nil)))
	req := httptest.NewRequest(http.MethodGet, "/uploads/styles/small/img/a.webp?v=3", nil)
	req.SetPathValue("style", "small")
	req.SetPathValue("filepath", "img/a.webp")
	rec := httptest.NewRecorder()
	c.ServeStyledImage(rec, req)
	if rec.Code != http.StatusFound || rec.Header().Get("Location") != "https://cdn.example.com/files/styles/small/img/a.webp?v=3" {
		t.Fatalf("%d %q", rec.Code, rec.Header().Get("Location"))
	}
	if ok, _ := filestore.Exists(t.Context(), store, "styles/small/img/a.webp"); !ok {
		t.Error("the derivative wasn't stored")
	}
}
