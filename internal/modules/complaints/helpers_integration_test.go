//go:build dbintegration

package complaints

import (
	"context"
	"testing"

	"github.com/kosatka-cms/kosatka/modulekit"
	"github.com/kosatka-cms/kosatka/modulekit/testkit"
	"github.com/kosatka-cms/kosatka/pkg/mailtemplate"
)

// insertRow runs an INSERT on db and returns the new row's id.
func insertRow(t *testing.T, db testkit.DB, query string, args ...any) int {
	t.Helper()
	id, err := db.Dialect.InsertID(context.Background(), db.DB, db.Dialect.Rebind(query), args...)
	if err != nil {
		t.Fatal(err)
	}
	return int(id)
}

// sentMail - an e-mail the service sent.
type sentMail struct {
	key, to string
	data    mailtemplate.TokenData
}

// fakeMail records the e-mails sent.
type fakeMail struct{ sent []sentMail }

func (m *fakeMail) Send(_ context.Context, key, to string, data mailtemplate.TokenData) error {
	m.sent = append(m.sent, sentMail{key, to, data})
	return nil
}

// fakeSite - the site's information: its name and address.
type fakeSite struct {
	modulekit.SiteInformationRepository
}

func (fakeSite) Get(context.Context) (*modulekit.SiteInformation, error) {
	return &modulekit.SiteInformation{SiteName: "Harbour Club", SiteEmail: "team@example.com"}, nil
}

// fixture - users ann (the author), bob, cat, dan and mod (a moderator);
// ann's public article "Harbour", her draft and page "About"; ann's
// comment on Harbour; the service with the settings: articles and
// comments take complaints, two hide.
type fixture struct {
	ann, bob, cat, dan, mod *modulekit.User
	harbour, draft, about   int
	comment                 int
	repo                    *Repo
	svc                     *Service
	mail                    *fakeMail
	deps                    modulekit.Deps
}

func newFixture(t *testing.T, db testkit.DB) fixture {
	t.Helper()
	ctx := context.Background()
	deps := testkit.Deps(db)
	user := func(name string) *modulekit.User {
		id := insertRow(t, db, "INSERT INTO users (username, slug, password_hash, email, status) VALUES (?, ?, '', ?, 1)", name, "users/"+name, name+"@example.com")
		u, err := deps.Users.ByID(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		return u
	}
	f := fixture{ann: user("ann"), bob: user("bob"), cat: user("cat"), dan: user("dan"), mod: user("mod"), deps: deps, mail: &fakeMail{}}
	node := func(typ, title, slug string, status int) int {
		return insertRow(t, db, "INSERT INTO nodes (type, title, slug, status, created_by) VALUES (?, ?, ?, ?, ?)", typ, title, slug, status, f.ann.ID)
	}
	f.harbour, f.draft, f.about = node("article", "Harbour", "harbour", 1), node("article", "Draft", "draft", 0), node("page", "About", "about", 1)
	f.comment = insertRow(t, db, "INSERT INTO comments (content, nid, created_by) VALUES ('<p>Cheap <b>boats</b> here</p>', ?, ?)", f.harbour, f.ann.ID)
	f.repo = NewRepo(db.DB)
	if err := f.repo.SaveSettings(ctx, Settings{Comments: true, NodeTypes: []string{"article"}, HideAfter: 2}); err != nil {
		t.Fatal(err)
	}
	deleter := testkit.NodeDeleter(deps.NodeRepo, func(ctx context.Context, id int) error {
		_, err := db.DB.ExecContext(ctx, db.Dialect.Rebind("DELETE FROM nodes WHERE id = ?"), id)
		return err
	})
	f.svc = NewService(ServiceDeps{Repo: f.repo, Nodes: deps.NodeRepo, Comments: deps.Comments, Deleter: deleter, Site: fakeSite{},
		Users: deps.Users, Mail: f.mail, Logger: testkit.Logger(), BaseURL: func() string { return "https://example.com/" }})
	return f
}

// target - the queue's row of kind/id (nil - none).
func (f fixture) target(t *testing.T, kind string, id int) *Target {
	t.Helper()
	for _, status := range []string{StatusNew, StatusResolved} {
		list, _, err := f.repo.List(context.Background(), status, 100, 0)
		if err != nil {
			t.Fatal(err)
		}
		for i := range list {
			if list[i].Kind == kind && list[i].ObjectID == id {
				return &list[i]
			}
		}
	}
	return nil
}

// nodeStatus - the content item's status.
func nodeStatus(t *testing.T, f fixture, id int) int {
	t.Helper()
	n, err := f.deps.NodeRepo.ByID(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	return n.Status
}
