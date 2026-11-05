package interfaces

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/png"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gorilla/sessions"
	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/files"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/internal/interfaces/middleware"
	"github.com/kosatka-cms/kosatka/pkg/csrf"
	"github.com/kosatka-cms/kosatka/pkg/filestore"
)

// userFilesAuth: user 1 is signed in; admin decides whether they hold
// the permission to manage other users' files.
type userFilesAuth struct {
	service.AuthService
	admin bool
}

func (a userFilesAuth) GetCurrentUser(context.Context, *http.Request) (*entity.User, error) {
	return &entity.User{ID: 1}, nil
}

func (a userFilesAuth) HasPermission(_ context.Context, _ *entity.User, perm string) bool {
	return a.admin && perm == service.PermAdministerUserFiles
}

// TestUserFilesOtherUser - ?user= opens another user's files only with
// the permission; without it the request is refused.
func TestUserFilesOtherUser(t *testing.T) {
	tmp := t.TempDir()
	for _, f := range []string{"u1/mine.pdf", "u2/theirs.pdf"} {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(tmp, f)), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(tmp, f), []byte("x"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	store := sessions.NewCookieStore([]byte("test-secret-key-0123456789abcdef"))
	svc := files.NewUserFilesService(filestore.NewLocal(tmp), nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	login := httptest.NewRecorder()
	loginReq := httptest.NewRequest(http.MethodGet, "/", nil)
	sess, _ := store.New(loginReq, "user-session")
	sess.Values["user_id"] = 1
	if err := sess.Save(loginReq, login); err != nil {
		t.Fatal(err)
	}
	cookie := login.Result().Cookies()[0]

	list := func(admin bool, query string) (int, string) {
		c := NewUserFilesController(svc, store, slog.New(slog.NewTextHandler(io.Discard, nil)), userFilesAuth{admin: admin}, nil, nil)
		req := httptest.NewRequest(http.MethodGet, "/api/user/files"+query, nil)
		req.AddCookie(cookie)
		rec := httptest.NewRecorder()
		middleware.AuthJSONMiddleware(store)(http.HandlerFunc(c.List)).ServeHTTP(rec, req)
		return rec.Code, rec.Body.String()
	}

	if code, body := list(false, ""); code != http.StatusOK || !strings.Contains(body, "mine.pdf") {
		t.Fatalf("own files: %d %s", code, body)
	}
	if code, body := list(false, "?user=1"); code != http.StatusOK || !strings.Contains(body, "mine.pdf") {
		t.Fatalf("own files by id: %d %s", code, body)
	}
	if code, body := list(false, "?user=2"); code != http.StatusForbidden || strings.Contains(body, "theirs.pdf") {
		t.Fatalf("other user's files without permission: %d %s", code, body)
	}
	if code, body := list(true, "?user=2"); code != http.StatusOK || !strings.Contains(body, "theirs.pdf") || !strings.Contains(body, "/u2") {
		t.Fatalf("other user's files with permission: %d %s", code, body)
	}
	if code, _ := list(true, "?user=abc"); code != http.StatusBadRequest {
		t.Fatalf("bad user id: %d", code)
	}
}

type docsAuth struct {
	service.AuthService
	canUpload bool
}

func (a docsAuth) GetCurrentUser(context.Context, *http.Request) (*entity.User, error) {
	return &entity.User{ID: 1}, nil
}

func (a docsAuth) HasPermission(_ context.Context, _ *entity.User, perm string) bool {
	return a.canUpload && perm == service.PermUploadDocuments
}

type docsUploadSettings struct{}

func (docsUploadSettings) Get(context.Context) (*entity.UploadSettings, error) {
	return &entity.UploadSettings{
		MaxFileSizeBytes:   5 << 20,
		MaxImageDimension:  2000,
		AllowedMimeTypes:   []string{"image/png", "image/jpeg"},
		UserFileExtensions: []string{"pdf"},
	}, nil
}

func (docsUploadSettings) Update(context.Context, *entity.UploadSettings) error { return nil }

// TestUserFilesDocumentsNeedPermission - without "upload documents" a
// user uploads images only; with it, documents of the policy's types too.
func TestUserFilesDocumentsNeedPermission(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	store := sessions.NewCookieStore([]byte("test-secret-key-0123456789abcdef"))
	tmp := t.TempDir()
	uploads := files.NewFileUploadService(filestore.NewLocal(tmp), docsUploadSettings{}, nil, nil, logger)
	svc := files.NewUserFilesService(filestore.NewLocal(tmp), uploads, logger)

	// A session with user 1 and a CSRF token.
	loginReq := httptest.NewRequest(http.MethodGet, "/", nil)
	login := httptest.NewRecorder()
	sess, _ := store.Get(loginReq, "user-session")
	sess.Values["user_id"] = 1
	// Same session: EnsureToken stores the token next to user_id and saves.
	token := csrf.EnsureToken(store, login, loginReq)
	cookies := login.Result().Cookies()

	var pngBuf bytes.Buffer
	if err := png.Encode(&pngBuf, image.NewRGBA(image.Rect(0, 0, 4, 4))); err != nil {
		t.Fatal(err)
	}

	send := func(canUpload bool, name string, content []byte) (int, map[string]any) {
		c := NewUserFilesController(svc, store, logger, docsAuth{canUpload: canUpload}, nil, nil)
		var body bytes.Buffer
		mw := multipart.NewWriter(&body)
		_ = mw.WriteField("csrf_token", token)
		fw, _ := mw.CreateFormFile("file", name)
		_, _ = fw.Write(content)
		_ = mw.Close()
		req := httptest.NewRequest(http.MethodPost, "/api/user/files/upload", &body)
		req.Header.Set("Content-Type", mw.FormDataContentType())
		for _, ck := range cookies {
			req.AddCookie(ck)
		}
		rec := httptest.NewRecorder()
		middleware.AuthJSONMiddleware(store)(http.HandlerFunc(c.Upload)).ServeHTTP(rec, req)
		var out map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &out)
		return rec.Code, out
	}

	pdf := []byte("%PDF-1.7\n1 0 obj\n")
	if code, out := send(false, "report.pdf", pdf); code != http.StatusBadRequest || !strings.Contains(out["error"].(string), "images only (PNG, JPEG)") {
		t.Fatalf("pdf without permission: %d %v", code, out)
	}
	if code, out := send(true, "report.pdf", pdf); code != http.StatusOK || out["kind"] != "file" {
		t.Fatalf("pdf with permission: %d %v", code, out)
	}
	if code, out := send(false, "pic.png", pngBuf.Bytes()); code != http.StatusOK || out["kind"] != "image" {
		t.Fatalf("image without permission: %d %v", code, out)
	}
}

// formatAuth - docsAuth that lets the user use text formats in may.
type formatAuth struct {
	docsAuth
	may map[int]bool
}

func (a formatAuth) CanUseTextFormat(_ context.Context, _ *entity.User, f entity.TextFormatDef) bool {
	return a.may[f.ID]
}

// knownFormats finds every format with a positive ID.
type knownFormats struct{}

func (knownFormats) TextFormatByID(_ context.Context, id int) (*entity.TextFormatDef, error) {
	if id <= 0 {
		return nil, entity.ErrEntityNotFound
	}
	return &entity.TextFormatDef{ID: id}, nil
}

// TestUserFilesFormatRules - an editor's upload follows a text format's
// upload rules only when the user may use that format: a request naming
// another gets the site's.
func TestUserFilesFormatRules(t *testing.T) {
	c := NewUserFilesController(nil, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), formatAuth{may: map[int]bool{2: true}}, nil, knownFormats{})
	r := httptest.NewRequest(http.MethodPost, "/api/user/files/upload", nil)
	for id, want := range map[int]bool{2: true, 3: false, 0: false} {
		if got := c.mayUseFormat(r, id); got != want {
			t.Errorf("format %d: %v, want %v", id, got, want)
		}
	}
	if NewUserFilesController(nil, nil, nil, formatAuth{}, nil, nil).mayUseFormat(r, 2) {
		t.Error("without the formats, a format's rules")
	}
}
