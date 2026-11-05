package subscriptions

import (
	"context"

	"github.com/kosatka-cms/kosatka/internal/modules/subscriptions/manifest"
	"github.com/kosatka-cms/kosatka/modulekit"
)

// Wire - the module's tables, the notifier on the core's events (a
// comment posted, content becoming public), the e-mails' background jobs,
// the profile's settings and Notifications tab, the e-mail (its text on
// the Mail page), the buttons of content and terms, the unread count on
// the user menu.
func Wire(d modulekit.Deps) modulekit.Parts {
	repo := NewRepo(d.DB)
	notifier := NewNotifier(repo, d.NodeRepo, d.Comments, d.Taxonomy)
	d.Comments.OnCommentPosted(manifest.Name, func(ctx context.Context, c modulekit.Comment) error {
		return notifier.CommentPosted(ctx, c)
	})
	modulekit.OnNodePublished(manifest.Name, notifier.NodePublished)

	desc := describeDeps{users: d.Users, nodes: d.NodeRepo, comments: d.Comments, terms: d.Taxonomy}
	mailer := NewMailer(MailerDeps{Repo: repo, Describe: desc, Users: d.Users, Site: d.SiteInformation,
		Mail: d.Mail, BaseURL: modulekit.MailBaseURL, Logger: d.Logger.With("component", "subscriptions.mail"),
		Visitor: d.Auth.ContentVisitor})
	for _, j := range mailer.Jobs() {
		modulekit.AddJob(manifest.Name, j)
	}

	modulekit.AddMail(repo.MailDef())
	modulekit.AddUserSettingsSection(repo.UserSettingsSection())
	c := NewController(modulekit.NewBaseController(d.Base), repo, desc, d.NodeRepo, d.Taxonomy)
	modulekit.AddProfileTab(c.ProfileTab())
	modulekit.AddPageData(manifest.Name, c.PageData)
	modulekit.AddNodePage(manifest.Name, c.NodePage)
	modulekit.AddTermPage(manifest.Name, c.TermPage)
	return modulekit.Parts{
		Routes: func(r modulekit.Routes) { RegisterRoutes(r.Mux, r.Store, c) },
	}
}
