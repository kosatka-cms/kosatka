package privatemessages

import (
	"context"

	"github.com/kosatka-cms/kosatka/internal/modules/private_messages/manifest"
	"github.com/kosatka-cms/kosatka/modulekit"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
	"github.com/kosatka-cms/kosatka/pkg/ratelimit"
)

// Wire - the threads' service on the module's tables, the e-mail of a new
// message (its text - on the Mail page), the controller: its routes,
// the profile's messages tab, the unread count of every page (on the user
// menu); a user's messages go with their account's deletion with all
// content.
func Wire(d modulekit.Deps) modulekit.Parts {
	messages := NewMessageRepo(d.DB, d.Users)
	threads := NewService(NewThreadRepo(d.DB, d.Users), messages, NewMemberRepo(d.DB, d.Users), d.Users, d.Logger)
	settings := NewSettingsRepo(d.DB)
	modulekit.AddMail(settings.MailDef())
	modulekit.AddUserSettingsSection(settings.UserSettingsSection())
	mailer := NewMailer(MailerDeps{Threads: threads, Users: d.Users, Site: d.SiteInformation,
		MailOff: settings.MailOff, Mail: d.Mail, Logger: d.Logger.With("component", "private_messages.mail")})
	c := NewController(modulekit.NewBaseController(d.Base), threads, mailer, d.Users)
	modulekit.AddProfileTab(c.ProfileTab())
	modulekit.AddPageData(manifest.Name, c.PageData)
	return modulekit.Parts{
		// Ten messages a minute per user, three at once.
		Routes: func(r modulekit.Routes) { RegisterRoutes(r.Mux, r.Store, c, ratelimit.NewRateLimiter(10, 3)) },
		UserContent: &modulekit.UserContent{
			Label: i18n.N("private messages: @count"),
			Count: func(ctx context.Context, userID int) (int, error) {
				return messages.TotalCount(ctx, MessageFilter{AuthorID: &userID})
			},
			Delete: messages.DeleteByAuthor,
		},
	}
}
