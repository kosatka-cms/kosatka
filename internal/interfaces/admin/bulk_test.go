package admin

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/gorilla/sessions"
	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/internal/interfaces"
)

func TestParseBulkIDs(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/admin/content/bulk", nil)
	r.Form = url.Values{
		"ids": {"1", "2", "not-a-number", "-5", "0", "42"},
	}

	got := parseBulkIDs(r)
	want := []int{1, 2, 42}

	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("index %d: got %d, want %d", i, got[i], want[i])
		}
	}
}

func TestParseBulkIDs_Empty(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/admin/content/bulk", nil)
	got := parseBulkIDs(r)
	if len(got) != 0 {
		t.Fatalf("expected no ids, got %v", got)
	}
}

// bulkAuth - a user whose role has the permissions in perms.
type bulkAuth struct {
	captchaAuth
	perms map[string]bool
}

func (a bulkAuth) HasPermission(_ context.Context, _ *entity.User, key string) bool {
	return a.perms[key]
}
func (a bulkAuth) CanPublishNode(ctx context.Context, u *entity.User) bool {
	return a.HasPermission(ctx, u, service.PermPublishContent)
}
func (a bulkAuth) CanDeleteNode(ctx context.Context, n *entity.Node, u *entity.User) bool {
	return a.HasPermission(ctx, u, service.PermDeleteAny(n.Type.ToString()))
}

// bulkNodes - nodes by ID (ByIDs).
type bulkNodes struct {
	entity.NodesRepository
	nodes map[int]entity.Node
}

func (n bulkNodes) ByIDs(_ context.Context, ids []int) (map[int]entity.Node, error) {
	out := map[int]entity.Node{}
	for _, id := range ids {
		if node, ok := n.nodes[id]; ok {
			out[id] = node
		}
	}
	return out, nil
}

// bulkRecorder - what the bulk service was asked to do.
type bulkRecorder struct {
	service.AdminBulkService
	deleted, published []int
}

func (b *bulkRecorder) BulkDeleteNodes(_ context.Context, _ *entity.User, ids []int) (int, []string) {
	b.deleted = append(b.deleted, ids...)
	return len(ids), nil
}
func (b *bulkRecorder) BulkUpdateNodeStatus(_ context.Context, _ *entity.User, ids []int, _ int) (int, []string) {
	b.published = append(b.published, ids...)
	return len(ids), nil
}
func (b *bulkRecorder) BulkUpdateNodeModeration(_ context.Context, _ *entity.User, ids []int, _ int) (int, []string) {
	return len(ids), nil
}

// TestBulkExecutePermissions - a bulk action asks the permissions the
// single action does, not only access to the admin: deleting takes
// "delete any <type>" per node (the rest are reported, not deleted),
// publishing "publish content" or "administer content", the moderation
// flag "administer content" or "moderation", deleting comments
// "administer comments".
// TestBulkTitles - every bulk action of the lists has its own question
// on the confirmation page; an unknown one gets the general question.
func TestBulkTitles(t *testing.T) {
	ctx := context.Background()
	other := bulkOtherTitle(ctx, "x", 2)
	for name, titles := range map[string]struct {
		title   func(context.Context, string, int) string
		actions []string
	}{
		"content": {bulkContentTitle, []string{"delete", "status_publish", "status_unpublish", "moderation_on", "moderation_off"}},
		"comment": {bulkCommentTitle, []string{"delete", "approve", "spam"}},
		"people":  {bulkPeopleTitle, []string{"delete", peopleActionDeleteWithContent, "block", "unblock"}},
	} {
		for _, action := range titles.actions {
			if got := titles.title(ctx, action, 2); got == other || !strings.Contains(got, "2") {
				t.Errorf("%s %s: %q", name, action, got)
			}
		}
		if got := titles.title(ctx, "x", 2); got != other {
			t.Errorf("%s, an unknown action: %q", name, got)
		}
	}
}

func TestBulkExecutePermissions(t *testing.T) {
	nodes := bulkNodes{nodes: map[int]entity.Node{1: {ID: 1, Type: "article"}, 2: {ID: 2, Type: "page"}}}
	run := func(handler string, perms map[string]bool, body string) (*httptest.ResponseRecorder, *bulkRecorder) {
		rec := &bulkRecorder{}
		ac := testController(interfaces.BaseDeps{Auth: bulkAuth{perms: perms}, Users: captchaUsers{}, Nodes: nodes,
			Templates: service.NewTemplateService(false), Store: sessions.NewCookieStore([]byte("test-secret-key-0123456789abcdef"))},
			func(c *Controller) { c.adminBulkService = rec })
		req := httptest.NewRequest(http.MethodPost, "/api/admin/bulk/"+handler, strings.NewReader(body))
		w := httptest.NewRecorder()
		if handler == "comments" {
			ac.BulkExecuteComments(w, req)
		} else {
			ac.BulkExecuteContent(w, req)
		}
		return w, rec
	}

	w, rec := run("content", map[string]bool{service.PermDeleteAny("article"): true}, `{"action":"delete","ids":[1,2,3]}`)
	if w.Code != http.StatusOK || !reflect.DeepEqual(rec.deleted, []int{1, 3}) || !strings.Contains(w.Body.String(), "ID 2") {
		t.Errorf("delete: %d %v %s", w.Code, rec.deleted, w.Body)
	}
	if w, rec := run("content", nil, `{"action":"status_publish","ids":[1]}`); w.Code != http.StatusForbidden || len(rec.published) != 0 {
		t.Errorf("publish without the permission: %d %v", w.Code, rec.published)
	}
	for _, perm := range []string{service.PermPublishContent, service.PermAdministerContent} {
		if w, rec := run("content", map[string]bool{perm: true}, `{"action":"status_unpublish","ids":[1]}`); w.Code != http.StatusOK || len(rec.published) != 1 {
			t.Errorf("unpublish with %s: %d %v", perm, w.Code, rec.published)
		}
	}
	if w, _ := run("content", map[string]bool{service.PermPublishContent: true}, `{"action":"moderation_off","ids":[1]}`); w.Code != http.StatusForbidden {
		t.Errorf("moderation without the permission: %d", w.Code)
	}
	if w, _ := run("content", map[string]bool{service.PermModerateContent: true}, `{"action":"moderation_off","ids":[1]}`); w.Code != http.StatusOK {
		t.Errorf("moderation by a moderator: %d", w.Code)
	}
	if w, _ := run("comments", map[string]bool{service.PermAccessAdmin: true}, `{"action":"delete","ids":[1]}`); w.Code != http.StatusForbidden {
		t.Errorf("comments without the permission: %d", w.Code)
	}
}
