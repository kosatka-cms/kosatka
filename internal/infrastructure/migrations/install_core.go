package migrations

import (
	"context"
	"fmt"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// installCore - the rows a new site needs:
// the single row of every settings table, the base and administrator
// roles, two text formats, the avatar image styles and the date formats.
// Values are minimal and neutral; the site sets them in the admin. Labels
// are in English, marked for the catalog: app install translates them to
// the site's language. Runs only on the first install of the core (an
// empty database), never on an existing site. Settings columns not set
// take the schema defaults; the account e-mail texts left empty fall back
// to the built-in ones.
func installCore(ctx context.Context, env Env) error {
	c := env.Client
	now := time.Now()
	steps := []struct {
		name string
		run  func() error
	}{
		// The site's name: the schema default, Kosatka CMS.
		{"site_information", func() error { return c.SiteInformation.Create().SetID(1).SetTimezone("UTC").Exec(ctx) }},
		{"account_settings", func() error {
			return c.AccountSettings.Create().SetID(1).
				SetRegistrationMode(entity.RegistrationModeVisitorsApproval).
				SetBodyWelcomeAdmin("").SetBodyWelcomePending("").SetBodyWelcomeOpen("").
				SetBodyActivation("").SetBodyPasswordReset("").SetBodyEmailConfirm("").
				SetBodyAdminNotify("").
				SetRegistrationBlockedDomains("").
				Exec(ctx)
		}},
		{"captcha_settings", func() error { return c.CaptchaSettings.Create().SetID(1).Exec(ctx) }},
		{"mail_settings", func() error { return c.MailSettings.Create().SetID(1).Exec(ctx) }},
		{"integration_settings", func() error {
			return c.IntegrationSettings.Create().SetID(1).SetVerificationTags("").SetHeadCode("").SetBodyEndCode("").Exec(ctx)
		}},
		{"maintenance_settings", func() error { return c.MaintenanceSettings.Create().SetID(1).SetMessage("").Exec(ctx) }},
		{"statistics_settings", func() error { return c.StatisticsSettings.Create().SetID(1).Exec(ctx) }},
		{"upload_settings", func() error {
			return c.UploadSettings.Create().SetID(1).
				SetMaxFileSizeBytes(10485760).SetMaxImageDimension(4000).
				SetAllowedMimeTypes("image/jpeg,image/png,image/gif,image/webp").
				Exec(ctx)
		}},
		{"search_settings", func() error { return c.SearchSettings.Create().SetID(1).Exec(ctx) }},

		// Text formats: basic HTML with the editor first (a form offers the
		// first format, by weight, its user may use), plain text - for
		// everyone - last.
		{"text_formats", func() error {
			return c.TextFormatDef.CreateBulk(
				c.TextFormatDef.Create().SetMachineName(entity.TextFormatPlainText).SetLabel(i18n.N("Plain text")).
					SetWeight(10).SetAllowedTags("").SetAllowClassAttribute(false).SetHasRichEditor(false).
					SetCreatedAt(now).SetUpdatedAt(now),
				c.TextFormatDef.Create().SetMachineName(entity.TextFormatBasicHTML).SetLabel(i18n.N("Basic HTML")).
					SetWeight(0).SetAllowedTags("p br strong em u s a ul ol li blockquote h2 h3 h4 img").
					SetAllowClassAttribute(false).SetHasRichEditor(true).
					SetCreatedAt(now).SetUpdatedAt(now),
			).Exec(ctx)
		}},

		// Roles: every signed-in user has the base one; the administrator
		// can do everything.
		{"roles", func() error {
			return c.Role.CreateBulk(
				c.Role.Create().SetName(entity.RoleUser).SetDescription(i18n.N("Signed-in user")).
					SetPermissions(map[string]any{"use_text_format_plain_text": true}).
					SetCreatedAt(now).SetUpdatedAt(now),
				c.Role.Create().SetName(entity.RoleAdmin).SetDescription(i18n.N("Administrator")).
					SetPermissions(map[string]any{"full_access": true}).
					SetCreatedAt(now).SetUpdatedAt(now),
			).Exec(ctx)
		}},

		// The users' avatar styles (list, profile page), the media
		// library's tiles' and the lightbox's the engine uses (its copy
		// fits a large screen: 2048 px on the longest side, no upscaling).
		{"image_styles", func() error {
			return c.ImageStyle.CreateBulk(
				c.ImageStyle.Create().SetName(entity.MediaThumbnailStyle).SetLabel(i18n.N("Media library")).
					SetWidth(320).SetHeight(240).SetMode("fill").SetAnchor("center").SetQuality(80).
					SetCreatedAt(now).SetUpdatedAt(now),
				c.ImageStyle.Create().SetName(entity.LightboxStyle).SetLabel(i18n.N("Lightbox")).
					SetWidth(2048).SetHeight(2048).SetMode("fit").SetAnchor("center").SetQuality(85).
					SetCreatedAt(now).SetUpdatedAt(now),
				c.ImageStyle.Create().SetName("user_avatar_list").SetLabel(i18n.N("User avatar")).
					SetWidth(100).SetHeight(100).SetMode("fill").SetAnchor("center").
					SetCreatedAt(now).SetUpdatedAt(now),
				c.ImageStyle.Create().SetName("user_avatar_profile").SetLabel(i18n.N("User avatar (profile)")).
					SetWidth(150).SetHeight(150).SetMode("fill").SetAnchor("center").
					SetCreatedAt(now).SetUpdatedAt(now),
			).Exec(ctx)
		}},

		// Date formats; the default one (entity.DefaultDateFormat) can't be
		// deleted.
		{"date_formats", func() error {
			return c.DateFormat.CreateBulk(
				dateFormat(c, entity.DefaultDateFormat, i18n.N("Short"), "Y-m-d", now),
				dateFormat(c, "medium", i18n.N("Medium"), "j F Y", now),
				dateFormat(c, entity.DateTimeFormat, i18n.N("Date and time"), "Y-m-d H:i", now),
			).Exec(ctx)
		}},
	}
	for _, st := range steps {
		if err := st.run(); err != nil {
			return fmt.Errorf("%s: %w", st.name, err)
		}
	}
	return nil
}

func dateFormat(c *generated.Client, machineName, label, pattern string, now time.Time) *generated.DateFormatCreate {
	return c.DateFormat.Create().SetMachineName(machineName).SetLabel(label).SetPattern(pattern).SetCreatedAt(now).SetUpdatedAt(now)
}

func init() {
	RegisterInstall(CoreSet, installCore)
}
