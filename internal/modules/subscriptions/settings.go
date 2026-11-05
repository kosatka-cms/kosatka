package subscriptions

import (
	"context"
	"net/http"

	"github.com/kosatka-cms/kosatka/internal/modules/subscriptions/manifest"
	"github.com/kosatka-cms/kosatka/modulekit"
)

// UserSettingsSection - the user's notification settings on their
// profile form (template "user_settings_subscriptions"): how often the
// e-mails come, the replies, the automatic subscriptions. The form marks
// the section shown (subscriptions_user_settings): a form without it (a
// theme's own) leaves them as they are.
func (r *Repo) UserSettingsSection() modulekit.UserSettingsSection {
	return modulekit.UserSettingsSection{
		Module: manifest.Name,
		Load: func(ctx context.Context, u modulekit.User) (any, error) {
			s, err := r.UserSettings(ctx, []int{u.ID})
			return s[u.ID], err
		},
		Save: func(ctx context.Context, u modulekit.User, req *http.Request) error {
			if req.FormValue("subscriptions_user_settings") != "1" {
				return nil
			}
			s := UserSettings{
				Mail:          req.FormValue("subscriptions_mail"),
				Replies:       req.FormValue("subscriptions_replies") == "1",
				AutoOwn:       req.FormValue("subscriptions_auto_own") == "1",
				AutoCommented: req.FormValue("subscriptions_auto_commented") == "1",
			}
			if s.Mail != MailDaily && s.Mail != MailNever {
				s.Mail = MailNow
			}
			return r.SaveUserSettings(ctx, u.ID, s)
		},
	}
}
