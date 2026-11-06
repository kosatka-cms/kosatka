//go:build dbintegration

package database_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/database"
	"github.com/kosatka-cms/kosatka/internal/testdb"
)

// TestSettingsRepositories - each settings repository saves and reads
// back its row (created by the core's install step) on MySQL and
// PostgreSQL.
func TestSettingsRepositories(t *testing.T) {
	schemaDB(t, func(t *testing.T, db testdb.DB) {
		ctx := context.Background()

		si := database.NewSiteInformationRepository(db.DB)
		want := &entity.SiteInformation{SiteName: "Site", Slogan: "s", SiteEmail: "a@example.com", Logo: entity.File{URI: "/logo.png"},
			FrontFeedView: "front", Timezone: "Europe/Berlin", DefaultLanguage: "ru", Page404: "/404", Page403: "/403"}
		if err := si.Update(ctx, want); err != nil {
			t.Fatal(err)
		}
		if err := si.SetTheme(ctx, "dark"); err != nil {
			t.Fatal(err)
		}
		got, err := si.Get(ctx)
		if err != nil || got.SiteName != "Site" || got.Logo.URI != "/logo.png" || got.Page403 != "/403" || got.Theme == nil || *got.Theme != "dark" {
			t.Errorf("site information: %+v %v", got, err)
		}
		want.Logo.URI = ""
		_ = si.Update(ctx, want)
		if got, _ := si.Get(ctx); got.Logo.URI != "" {
			t.Errorf("logo not cleared: %+v", got.Logo)
		}

		acc := database.NewAccountSettingsRepository(db.DB)
		wantAcc := &entity.AccountSettings{RegistrationMode: "admin", RequireEmailVerification: true, AvatarAllowedExtensions: []string{"jpg", "png"}, AvatarMaxFileSizeBytes: 1024, SubjActivation: "New", BodyActivation: "Body"}
		if err := acc.Update(ctx, wantAcc); err != nil {
			t.Fatal(err)
		}
		if got, err := acc.Get(ctx); err != nil || !reflect.DeepEqual(got, wantAcc) {
			t.Errorf("account settings:\n%+v\n%+v %v", got, wantAcc, err)
		}

		// Without the row (the install step creates it) Update creates it.
		seedRow(t, db, `DELETE FROM captcha_settings`)
		seedRow(t, db, `DELETE FROM mail_settings`)
		capRepo := database.NewCaptchaSettingsRepository(db.DB)
		wantCap := &entity.CaptchaSettings{Enabled: true, SiteKey: "k", SecretKey: "s", Forms: map[string]bool{
			entity.CaptchaFormLogin: true, entity.CaptchaFormRegister: false, entity.CaptchaFormPassword: true, "error_report": false, "webform": true}}
		for range 2 { // creates the row, then updates it
			if err := capRepo.Update(ctx, wantCap); err != nil {
				t.Fatal(err)
			}
		}
		if got, err := capRepo.Get(ctx); err != nil || !reflect.DeepEqual(got, wantCap) {
			t.Errorf("captcha: %+v %v", got, err)
		}

		mail := database.NewMailSettingsRepository(db.DB)
		// The modules' ways of sending's settings come back as saved, an
		// empty one not kept.
		wantMail := &entity.MailSettings{Enabled: true, Transport: "smtp", FromAddress: "a@example.com", SMTPHost: "h", SMTPPort: 587,
			TransportSettings: map[string]map[string]string{"mailapi": {"api_key": "k", "region": "eu"}}}
		saveMail := *wantMail
		saveMail.TransportSettings = map[string]map[string]string{"mailapi": {"api_key": "k", "region": "eu", "unset": ""}, "other": {"unset": ""}}
		for range 2 {
			if err := mail.Update(ctx, &saveMail); err != nil {
				t.Fatal(err)
			}
		}
		if got, err := mail.Get(ctx); err != nil || !reflect.DeepEqual(got, wantMail) {
			t.Errorf("mail: %+v %v", got, err)
		}
		uninstall := database.NewModuleUninstallRepository(db.DB)
		saveMail.Transport = "mailapi"
		if err := mail.Update(ctx, &saveMail); err != nil {
			t.Fatal(err)
		}
		if err := uninstall.DeleteMailTransportSettings(ctx, []string{"mailapi"}); err != nil {
			t.Fatal(err)
		}
		if got, err := mail.Get(ctx); err != nil || got.Transport != "" || len(got.TransportSettings) != 0 || got.SMTPHost != "h" {
			t.Errorf("after a module's transport is gone: %+v %v", got, err)
		}

		integ := database.NewIntegrationSettingsRepository(db.DB)
		wantInt := entity.IntegrationSettings{MetrikaID: "123", MetrikaWebvisor: true, GAID: "G-1", HeadCode: "<x>"}
		if err := integ.Update(ctx, wantInt); err != nil {
			t.Fatal(err)
		}
		if got, err := integ.Get(ctx); err != nil || *got != wantInt {
			t.Errorf("integration: %+v %v", got, err)
		}

		maint := database.NewMaintenanceRepository(db.DB)
		if err := maint.Update(ctx, &entity.MaintenanceSettings{Enabled: true, Message: "later"}); err != nil {
			t.Fatal(err)
		}
		if got, err := maint.Get(ctx); err != nil || !got.Enabled || got.Message != "later" {
			t.Errorf("maintenance: %+v %v", got, err)
		}

		comments := database.NewCommentSettingsRepository(db.DB)
		if got, err := comments.Get(ctx); err != nil || got != entity.DefaultCommentSettings() {
			t.Errorf("comments before saving: %+v %v", got, err)
		}
		for _, want := range []entity.CommentSettings{
			{NewAuthorChecks: -1, LinksLimit: 0, StopWords: "casino\nquick money", StopWordsAction: entity.StopWordsReject, DuplicateHours: 1, SpamKeepDays: 7},
			{NewAuthorChecks: 0, LinksLimit: -1, StopWordsAction: entity.StopWordsHold},
		} {
			if err := comments.Update(ctx, want); err != nil {
				t.Fatal(err)
			}
			if got, err := comments.Get(ctx); err != nil || got != want {
				t.Errorf("comments: %+v %v, want %+v", got, err, want)
			}
		}

		stats := database.NewStatisticsSettingsRepository(db.DB)
		if err := stats.Update(ctx, &entity.StatisticsSettings{ViewCountingEnabled: false}); err != nil {
			t.Fatal(err)
		}
		if got, err := stats.Get(ctx); err != nil || got.ViewCountingEnabled {
			t.Errorf("statistics: %+v %v", got, err)
		}

		up := database.NewUploadSettingsRepository(db.DB)
		wantUp := &entity.UploadSettings{MaxFileSizeBytes: 10, MaxImageDimension: 20, AllowedMimeTypes: []string{"image/png", "image/jpeg"}, UserFileExtensions: []string{"pdf", "zip"}}
		if err := up.Update(ctx, wantUp); err != nil {
			t.Fatal(err)
		}
		if got, err := up.Get(ctx); err != nil || !reflect.DeepEqual(got, wantUp) {
			t.Errorf("upload: %+v %v", got, err)
		}

		// A vocabulary (search badges below reference it).
		const vocab = 1
		seedRow(t, db, `INSERT INTO taxonomy_vocabulary (id, name, slug, created_at, updated_at) VALUES (1, 'Topics', 'topics', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`)

		search := database.NewSearchConfigRepository(db.DB)
		st := entity.SearchTypeDef{NodeType: "news", Enabled: true, Weight: 2, Limit: 5, Label: "News", MatchFields: []string{"title", "body"}, BadgeVocabularyID: vocab}
		for range 2 {
			if err := search.SaveSearchType(ctx, st); err != nil {
				t.Fatal(err)
			}
		}
		if err := search.UpdateSearchTypeWeight(ctx, "news", 7); err != nil {
			t.Fatal(err)
		}
		types, err := search.SearchTypes(ctx)
		st.Weight = 7
		if err != nil || len(types) != 1 || !reflect.DeepEqual(types[0], st) {
			t.Errorf("search types: %+v %v", types, err)
		}
		st.Enabled = false
		if err := search.SaveSearchType(ctx, st); err != nil {
			t.Fatal(err)
		}
		if types, err := search.SearchTypes(ctx); err != nil || len(types) != 1 || types[0].Enabled {
			t.Errorf("search type switched off: %+v %v", types, err)
		}
		if err := search.SaveSearchSettings(ctx, entity.SearchSettings{PlaceholderImage: "/p.png", CategoriesMenu: "main"}); err != nil {
			t.Fatal(err)
		}
		if got, err := search.SearchSettings(ctx); err != nil || got.CategoriesMenu != "main" {
			t.Errorf("search settings: %+v %v", got, err)
		}

		oauth := database.NewOAuthSettingsRepository(db.DB)
		for range 2 {
			if err := oauth.Upsert(ctx, entity.OAuthProviderSettings{Provider: "google", Enabled: true, ClientID: "id", ClientSecret: "s"}); err != nil {
				t.Fatal(err)
			}
		}
		if list, err := oauth.List(ctx); err != nil || list["google"].ClientID != "id" || len(list) != 1 {
			t.Errorf("oauth: %+v %v", list, err)
		}
		if linked, err := oauth.LinkedAccounts(ctx); err != nil || len(linked) != 0 {
			t.Errorf("linked accounts: %+v %v", linked, err)
		}

		mod := database.NewModuleRepository(db.DB)
		for _, on := range []bool{false, true, false} {
			if err := mod.SetEnabled(ctx, "complaints", on); err != nil {
				t.Fatal(err)
			}
		}
		if states, err := mod.States(ctx); err != nil || states["complaints"] {
			t.Errorf("module states: %+v %v", states, err)
		}
		if n, err := mod.CountRows(ctx, "complaint_settings"); err != nil || n != 1 {
			t.Errorf("count rows: %d %v", n, err)
		}
		if refs, err := mod.PluginPlacements(ctx, []entity.PluginBlockID{entity.PluginBlockID("poll")}); err != nil || len(refs) != 0 {
			t.Errorf("plugin placements: %+v %v", refs, err)
		}
		// A module's uninstall removes its plugin blocks' placements only.
		for _, id := range []string{string(entity.PluginBlockID("poll")), string(entity.PluginBlockID("error_report_hint"))} {
			if _, err := db.DB.Exec(db.Dialect.Rebind(`INSERT INTO block_placements (placement_type, plugin_id, theme, region, weight, created_at, updated_at)
				VALUES ('plugin', ?, 'default', 'sidebar', 0, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`), id); err != nil {
				t.Fatal(err)
			}
		}
		if err := mod.DeletePluginPlacements(ctx, []entity.PluginBlockID{entity.PluginBlockID("poll")}); err != nil {
			t.Fatal(err)
		}
		refs, err := mod.PluginPlacements(ctx, []entity.PluginBlockID{entity.PluginBlockID("poll"), entity.PluginBlockID("error_report_hint")})
		if err != nil || len(refs) != 1 || refs[0].PluginID != entity.PluginBlockID("error_report_hint") {
			t.Errorf("after delete: %+v %v", refs, err)
		}
	})
}
