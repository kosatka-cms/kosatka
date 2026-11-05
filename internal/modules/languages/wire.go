package languages

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/kosatka-cms/kosatka/internal/modules/languages/manifest"
	"github.com/kosatka-cms/kosatka/modulekit"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// Wire - the site's languages in use while the module is on (their
// prefixes, SetLanguagesSwitch); on every page the switcher's links, the
// alternates, a content item's translations; the Translations tab of a
// translated type's items (AddNodePage), their marks kept as their forms
// save them (OnNodeFormSaved); the content lists in the page's language
// (SetContentListFilter); the Language section of their forms; the
// site's texts in its languages (wireTexts); the Languages page. Its uninstall step
// forgets the languages.
func Wire(d modulekit.Deps) modulekit.Parts {
	logger := d.Logger.With("component", "languages")
	translations := NewTranslations(NewTranslationRepo(d.DB), d.NodeRepo)
	modulekit.SetLanguagesSwitch(manifest.Name)
	pages := pageData{translations: translations, logger: logger}
	modulekit.AddPageData(manifest.Name, func(r *http.Request, data map[string]any) {
		pages.add(r, data)
	})
	modulekit.AddNodePage(manifest.Name, func(ctx context.Context, node modulekit.Node) modulekit.NodePageParts {
		if _, others := modulekit.SiteLanguages(); len(others) == 0 {
			return modulekit.NodePageParts{}
		}
		ok, err := translations.Translatable(ctx, node.Type.ToString())
		modulekit.LogFallback(logger, err, "error loading the translatable types")
		return modulekit.NodePageParts{Data: map[string]any{"translatable": ok}}
	})
	modulekit.OnNodeFormSaved(manifest.Name, translations.formSaved)
	modulekit.SetContentListFilter(manifest.Name, listFilter(translations, logger))
	modulekit.AddNodeSettings(nodeLanguage{nodes: d.NodeRepo, translations: translations}.section(manifest.Name))
	texts := NewTexts(d.DB)
	wireTexts(d, texts, logger)
	site := d.SiteInformation
	changed := func() {
		if d.RefreshBlocks != nil {
			d.RefreshBlocks()
		}
	}
	c := NewController(modulekit.NewBaseController(d.Base), d.DB, NewRepo(d.DB), site, changed, translations, d.ContentTypes)
	tc := NewTranslationsController(modulekit.NewBaseController(d.Base), d.DB, d.NodeRepo, d.ContentTypes, translations)
	return modulekit.Parts{
		Routes: func(r modulekit.Routes) {
			RegisterRoutes(r.Mux, c, r.AdminOnly)
			RegisterTranslationRoutes(r.Mux, tc, modulekit.AuthMiddleware(r.Store))
		},
		Uninstall: func(ctx context.Context) error {
			if err := site.SetLanguages(ctx, nil); err != nil {
				return fmt.Errorf("forgetting the site's languages: %w", err)
			}
			info, err := site.Get(ctx)
			if err != nil {
				return fmt.Errorf("loading the site information: %w", err)
			}
			modulekit.ApplySiteLanguages(info.DefaultLanguage, nil)
			return nil
		},
	}
}

// wireTexts - the site's texts in its other languages and the things
// shown by language, for the core (SetTextTranslator,
// SetLanguageVisibility); a content item's address in a language (its
// translation's, SetNodeURLsInLanguage); the sections of the core's
// forms editing them.
func wireTexts(d modulekit.Deps, texts *Texts, logger *slog.Logger) {
	modulekit.SetTextTranslator(manifest.Name, func(lang, kind, ref string) (string, bool) {
		s, ok, err := texts.Text(context.Background(), lang, kind, ref)
		modulekit.LogFallback(logger, err, "error loading the site's texts")
		return s, ok
	})
	modulekit.SetLanguageVisibility(manifest.Name, func(kind, ref, lang string) bool {
		ok, err := texts.Visible(context.Background(), kind, ref, lang)
		modulekit.LogFallback(logger, err, "error loading the language visibility")
		return ok || err != nil
	})
	modulekit.SetNodeURLsInLanguage(manifest.Name, func(ctx context.Context, ids []int, lang string) map[int]string {
		urls, err := nodeURLs(ctx, d.DB, ids, lang)
		modulekit.LogFallback(logger, err, "error loading the addresses of translations")
		return urls
	})
	sections := []struct {
		form    string
		section textSection
	}{
		{modulekit.FormMenuLink, textSection{fields: []textField{{Kind: modulekit.TextMenuLinkTitle, Name: "text"}}, visibility: modulekit.VisibilityMenuLink}},
		{modulekit.FormBlock, textSection{fields: []textField{{Kind: modulekit.TextBlockTitle, Name: "block_title"}}}},
		{modulekit.FormBlockPlacement, textSection{fields: []textField{{Kind: modulekit.TextPlacementTitle, Name: "placement_title"}}, visibility: modulekit.VisibilityBlockPlacement}},
		{modulekit.FormTerm, textSection{fields: []textField{
			{Kind: modulekit.TextTermName, Name: "term_name", Label: i18n.N("Name")},
			{Kind: modulekit.TextTermDescription, Name: "term_description", Label: i18n.N("Description"), Multiline: true, HTML: true},
		}}},
		{modulekit.FormSiteInformation, textSection{fields: []textField{
			{Kind: modulekit.TextSiteName, Name: "site_name", Label: i18n.N("Site name")},
			{Kind: modulekit.TextSiteSlogan, Name: "site_slogan", Label: i18n.N("Slogan")},
		}}},
		{modulekit.FormView, textSection{fields: []textField{
			{Kind: modulekit.TextViewTitle, Name: "view_title", Label: i18n.N("Title")},
			{Kind: modulekit.TextViewDescription, Name: "view_description", Label: i18n.N("Meta description")},
		}}},
		{modulekit.FormField, textSection{fields: []textField{{Kind: modulekit.TextFieldLabel, Name: "field_label"}}}},
	}
	for _, s := range sections {
		s.section.texts = texts
		modulekit.AddFormSection(s.section.section(manifest.Name, s.form))
	}
}
