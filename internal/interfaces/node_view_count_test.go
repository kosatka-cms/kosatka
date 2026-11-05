package interfaces

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gorilla/sessions"
	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/pkg/csrf"
)

// viewCountMissing - counting finds no node (as the service reports it).
type viewCountMissing struct{ service.ViewCountingService }

func (viewCountMissing) CountView(_ context.Context, id int) error {
	return fmt.Errorf("counting a view (node %d): %w", id, fmt.Errorf("%w: node with ID %d", entity.ErrEntityNotFound, id))
}

// TestPostViewMissingNode - a view of a node that doesn't exist (often a
// bot's made-up ID) answers 404 without logging an error.
func TestPostViewMissingNode(t *testing.T) {
	store := sessions.NewCookieStore([]byte("test-secret-key-0123456789abcdef"))
	w := httptest.NewRecorder()
	token := csrf.EnsureToken(store, w, httptest.NewRequest(http.MethodGet, "/", nil))
	var logs bytes.Buffer
	c := NewViewCountController(slog.New(slog.NewTextHandler(&logs, nil)), viewCountMissing{})

	req := httptest.NewRequest(http.MethodPost, "/api/node/view", strings.NewReader(url.Values{"nid": {"999"}, "csrf_token": {token}}.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for _, ck := range w.Result().Cookies() {
		req.AddCookie(ck)
	}
	rec := httptest.NewRecorder()
	c.PostView(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("code %d, want 404", rec.Code)
	}
	if strings.Contains(logs.String(), "level=ERROR") {
		t.Errorf("logged an error: %s", logs.String())
	}
}

// viewCountRecorder counts the views it is given; nid 2 is unpublished.
type viewCountRecorder struct {
	service.ViewCountingService
	counted []int
}

func (v *viewCountRecorder) CountView(_ context.Context, id int) error {
	if id == 2 {
		return service.ErrNodeNotPublic
	}
	v.counted = append(v.counted, id)
	return nil
}

// TestPostViewOncePerClient - a client's repeated view of a node isn't
// counted; another node's or another client's is; an unpublished node
// answers like a missing one.
func TestPostViewOncePerClient(t *testing.T) {
	rec := &viewCountRecorder{}
	c := NewViewCountController(slog.New(slog.NewTextHandler(io.Discard, nil)), rec)
	post := func(ip, nid string) int {
		req := httptest.NewRequest(http.MethodPost, "/api/node/view", strings.NewReader(url.Values{"nid": {nid}}.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.RemoteAddr = ip + ":1234"
		w := httptest.NewRecorder()
		c.PostView(w, req)
		return w.Code
	}
	for _, v := range [][2]string{{"192.0.2.1", "1"}, {"192.0.2.1", "1"}, {"192.0.2.1", "3"}, {"192.0.2.2", "1"}} {
		if code := post(v[0], v[1]); code != http.StatusNoContent {
			t.Fatalf("%v: %d", v, code)
		}
	}
	if fmt.Sprint(rec.counted) != "[1 3 1]" {
		t.Errorf("counted %v", rec.counted)
	}
	if code := post("192.0.2.1", "2"); code != http.StatusNotFound {
		t.Errorf("unpublished: %d, want 404", code)
	}
}
