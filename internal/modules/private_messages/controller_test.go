package privatemessages

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/sessions"

	"github.com/kosatka-cms/kosatka/internal/modules/private_messages/manifest"
	"github.com/kosatka-cms/kosatka/modulekit"
	"github.com/kosatka-cms/kosatka/modulekit/testkit"
)

// fakeThreads - the threads of the controller's tests: unread - the
// unread count, threads - the list; filter - the last list's filter.
type fakeThreads struct {
	Service
	unread  int
	threads Threads
	filter  *ThreadFilter
}

func (f *fakeThreads) UnreadCount(context.Context, int) (int, error) { return f.unread, nil }

func (f *fakeThreads) ThreadTotalCount(context.Context, ThreadFilter) (int, error) {
	return len(f.threads), nil
}

func (f *fakeThreads) ThreadList(_ context.Context, filter ThreadFilter) (*Threads, error) {
	*f.filter = filter
	out := append(Threads(nil), f.threads...)
	return &out, nil
}

func testController(threads Service) *Controller {
	return NewController(modulekit.NewBaseController(modulekit.BaseDeps{Logger: testkit.Logger(),
		Store: sessions.NewCookieStore([]byte("test-secret-key-0123456789abcdef"))}), threads, nil, nil)
}

// withModule sets the module's state for the test.
func withModule(t *testing.T, enabled bool) {
	t.Helper()
	testkit.WithModules(t, map[string]bool{manifest.Name: enabled})
}

// TestProfileTabAllowed - the messages tab is the profile owner's and the
// users' administrators'; another user and a guest don't get it.
func TestProfileTabAllowed(t *testing.T) {
	tab := testController(&fakeThreads{}).ProfileTab()
	if tab.Module != manifest.Name || tab.Path != manifest.TabPath {
		t.Fatalf("the tab: %+v", tab)
	}
	viewer, profile := &modulekit.User{ID: 1}, &modulekit.User{ID: 2}
	for name, c := range map[string]struct {
		viewer                 *modulekit.User
		isOwner, canAdminister bool
		want                   bool
	}{
		"owner":         {viewer, true, false, true},
		"administrator": {viewer, false, true, true},
		"another user":  {viewer, false, false, false},
		"guest":         {nil, false, false, false},
	} {
		if got := tab.Allowed(context.Background(), c.viewer, profile, c.isOwner, c.canAdminister); got != c.want {
			t.Errorf("%s: %v, want %v", name, got, c.want)
		}
	}
}

// TestPageData - every page of a signed-in user gets their unread
// threads, added to the user menu's number; a guest's page - nothing.
func TestPageData(t *testing.T) {
	c := testController(&fakeThreads{unread: 4})
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	data := map[string]any{"user": &modulekit.User{ID: 1}, "userMenuCount": 1}
	c.PageData(req, data)
	if data["unreadCount"] != 4 || data["userMenuCount"] != 5 {
		t.Errorf("signed in: unread %v, menu %v", data["unreadCount"], data["userMenuCount"])
	}
	guest := map[string]any{"user": (*modulekit.User)(nil)}
	c.PageData(req, guest)
	if _, ok := guest["unreadCount"]; ok {
		t.Errorf("a guest's page: %v", guest)
	}
}

// TestLoadTab - the tab lists the profile user's active threads (archived
// with ?status=archived), the unread marked, the page not indexed.
func TestLoadTab(t *testing.T) {
	var filter ThreadFilter
	threads := &fakeThreads{filter: &filter, threads: Threads{
		{ID: 1, Members: []Member{{UserID: 7, IsNew: true}}},
		{ID: 2, Members: []Member{{UserID: 7}}},
	}}
	load := testController(threads).ProfileTab().Load
	profile := &modulekit.User{ID: 7}

	data := map[string]any{"pageSEO": modulekit.PageSEO{Title: "Messages"}}
	req := httptest.NewRequest(http.MethodGet, "/me/messages", nil)
	if err := load(context.Background(), httptest.NewRecorder(), req, profile, data); err != nil {
		t.Fatal(err)
	}
	list := *data["threads"].(*Threads)
	if filter.UserID != 7 || filter.Deleted == nil || *filter.Deleted || data["filterStatus"] != "active" {
		t.Errorf("active: filter %+v, status %v", filter, data["filterStatus"])
	}
	if len(list) != 2 || !list[0].IsUnread || list[1].IsUnread {
		t.Errorf("unread marks: %+v", list)
	}
	if seo := data["pageSEO"].(modulekit.PageSEO); !seo.NoIndex || seo.Title != "Messages" {
		t.Errorf("SEO: %+v", seo)
	}
	if data["CSRFToken"] == "" {
		t.Error("no CSRF token for the archive form")
	}

	req = httptest.NewRequest(http.MethodGet, "/me/messages?status=archived", nil)
	if err := load(context.Background(), httptest.NewRecorder(), req, profile, map[string]any{}); err != nil {
		t.Fatal(err)
	}
	if filter.Deleted == nil || !*filter.Deleted {
		t.Errorf("archived: filter %+v", filter)
	}
}
