package service

import (
	"context"
	"errors"
	"html/template"
	"io"
	"log/slog"
	"regexp"
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

type fakeIntegrationRepo struct {
	s     entity.IntegrationSettings
	reads int
}

func (f *fakeIntegrationRepo) Get(context.Context) (*entity.IntegrationSettings, error) {
	f.reads++
	s := f.s
	return &s, nil
}

func (f *fakeIntegrationRepo) Update(_ context.Context, s entity.IntegrationSettings) error {
	f.s = s
	return nil
}

func TestIntegrationSettingsSave(t *testing.T) {
	ctx := context.Background()
	repo := &fakeIntegrationRepo{s: entity.IntegrationSettings{HeadCode: "<script>old()</script>", BodyEndSkipServicePages: true}}
	s := NewIntegrationSettingsService(repo, slog.New(slog.NewTextHandler(io.Discard, nil)))

	if err := s.Save(ctx, entity.IntegrationSettings{MetrikaID: "12ab"}, true); !errors.Is(err, ErrIntegrationMetrikaID) {
		t.Errorf("metrika: %v", err)
	}
	if err := s.Save(ctx, entity.IntegrationSettings{GAID: "G-<x>"}, true); !errors.Is(err, ErrIntegrationGAID) {
		t.Errorf("ga: %v", err)
	}

	// Without full access the raw code is kept as stored.
	in := entity.IntegrationSettings{VerificationTags: " yandex-verification = abc \n\nbroken line\ngoogle-site-verification=xyz", MetrikaID: " 12345678 ", GAID: "G-TEST1", HeadCode: "<script>evil()</script>"}
	if err := s.Save(ctx, in, false); err != nil {
		t.Fatal(err)
	}
	if repo.s.HeadCode != "<script>old()</script>" || !repo.s.BodyEndSkipServicePages {
		t.Errorf("raw code must be kept without full access: %+v", repo.s)
	}
	if repo.s.VerificationTags != "yandex-verification=abc\ngoogle-site-verification=xyz" || repo.s.MetrikaID != "12345678" {
		t.Errorf("normalized: %+v", repo.s)
	}

	// Cached after the first read, refreshed after a save.
	reads := repo.reads
	_ = s.Current(ctx)
	_ = s.Current(ctx)
	if repo.reads != reads+1 {
		t.Errorf("Current should cache: %d reads", repo.reads-reads)
	}
	if err := s.Save(ctx, entity.IntegrationSettings{HeadCode: "<script>new()</script>"}, true); err != nil {
		t.Fatal(err)
	}
	if got := s.Current(ctx).HeadCode; got != "<script>new()</script>" {
		t.Errorf("after save: %q", got)
	}
}

// TestLayoutIntegrations - the header/footer render the settings: the
// verification metas, GA, Metrika with its options, the raw code; the
// before-</body> code stays off service pages when told so; nothing at
// all without settings.
func TestLayoutIntegrations(t *testing.T) {
	zzChdirToProjectRoot(t)
	render := func(s *entity.IntegrationSettings, data map[string]any) string {
		svc := &DefaultTemplateService{cache: map[string]*template.Template{}}
		if s != nil {
			svc.SetIntegrations(NewIntegrationSettingsService(&fakeIntegrationRepo{s: *s}, slog.New(slog.NewTextHandler(io.Discard, nil))))
		}
		files, err := svc.pageFilesToParse("dynamic/page.html", []string{"dynamic", "page.html"})
		if err != nil {
			t.Fatal(err)
		}
		set, err := svc.getOrParse("layout-integrations", nil, func() ([]string, error) { return files, nil })
		if err != nil {
			t.Fatal(err)
		}
		if data["blockPlacements"] == nil {
			data["blockPlacements"] = map[entity.Region][]entity.BlockWithPlacement{}
		}
		var b strings.Builder
		for _, name := range []string{"header", "footer"} {
			if err := set.ExecuteTemplate(&b, name, data); err != nil {
				t.Fatalf("%s: %v", name, err)
			}
		}
		return b.String()
	}

	settings := &entity.IntegrationSettings{
		VerificationTags: "yandex-verification=abc123\ngoogle-site-verification=g456",
		MetrikaID:        "12345678", MetrikaClickmap: true, MetrikaTrackLinks: true, MetrikaAccurateBounce: true,
		GAID:     "G-TEST1",
		HeadCode: "<script>headLoader()</script>", BodyEndCode: "<script>fullscreenAd()</script>", BodyEndSkipServicePages: true,
	}
	out := render(settings, map[string]any{})
	for _, want := range []string{
		`<meta name="yandex-verification" content="abc123">`,
		`<meta name="google-site-verification" content="g456">`,
		`<script>headLoader()</script>`,
		`googletagmanager.com/gtag/js?id=G-TEST1`,
		`gtag('config', "G-TEST1")`,
		// Yandex's current snippet: the counter in the loader's address,
		// ssr; html/template pads values in JS with spaces.
		`mc.yandex.ru/metrika/tag.js?id=12345678'`,
		`ym( 12345678 , 'init', {ssr:true, clickmap: true `,
		`trackLinks: true `,
		`mc.yandex.ru/watch/12345678`,
		`<script>fullscreenAd()</script>`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %s", want)
		}
	}
	if strings.Contains(out, "webvisor") {
		t.Error("webvisor is off")
	}

	if service := render(settings, map[string]any{"servicePage": true}); strings.Contains(service, "fullscreenAd") || !strings.Contains(service, "ym( 12345678 ") {
		t.Error("service pages: body-end code off, Metrika still on")
	}
	// A page whose address carries a secret: no code at all, the
	// verification metas stay.
	secret := render(settings, map[string]any{"tokenInURL": true})
	for _, bad := range []string{"headLoader", "gtag", "mc.yandex.ru", "fullscreenAd"} {
		if strings.Contains(secret, bad) {
			t.Errorf("a token in the address: %s", bad)
		}
	}
	if !strings.Contains(secret, `content="abc123"`) {
		t.Error("a token in the address: the verification metas are gone")
	}

	// The banner asking: Metrika and GA (analytics) and the body-end code
	// (advertising) wait as inert templates, the head code doesn't; the
	// banner offers both groups.
	asking := *settings
	asking.Consent = entity.Consent{Enabled: true, Mode: entity.ConsentAsk, Version: 3, PolicyURL: "/privacy",
		MetrikaGroup: entity.ConsentAnalytics, GAGroup: entity.ConsentAnalytics, BodyEndGroup: entity.ConsentMarketing}
	out = render(&asking, map[string]any{})
	waiting := ""
	for _, m := range regexp.MustCompile(`(?s)<template data-consent="analytics">(.*?)</template>`).FindAllStringSubmatch(out, -1) {
		waiting += m[1]
	}
	if !strings.Contains(waiting, "googletagmanager.com/gtag/js?id=G-TEST1") || !strings.Contains(waiting, "mc.yandex.ru/metrika/tag.js") {
		t.Errorf("asking: GA and Metrika don't wait as analytics:\n%s", waiting)
	}
	for _, want := range []string{
		`<template data-consent="marketing"><script>fullscreenAd()</script></template>`,
		`data-consent-banner data-consent-version="3" data-consent-mode="consent"`,
		`<a href="/privacy">Learn more</a>`,
		`value="analytics" data-consent-group`,
		`value="marketing" data-consent-group`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("asking: missing %s", want)
		}
	}
	if strings.Contains(out, `<template data-consent="analytics"><script>headLoader()`) || !strings.Contains(out, `<script>headLoader()</script>`) {
		t.Error("asking: the head code needs no consent")
	}
	// Notice only: the codes run at once, the banner says "Got it".
	notice := asking
	notice.Consent.Mode = entity.ConsentNotice
	if out := render(&notice, map[string]any{}); strings.Contains(out, "<template data-consent") || !strings.Contains(out, "data-consent-ok") {
		t.Error("notice: codes wrapped or no Got it")
	}
	// Asking with no code waiting: no banner.
	nothing := asking
	nothing.Consent.MetrikaGroup, nothing.Consent.GAGroup, nothing.Consent.BodyEndGroup = "", "", ""
	if out := render(&nothing, map[string]any{}); strings.Contains(out, "data-consent-banner") {
		t.Error("nothing to ask: a banner")
	}

	empty := render(nil, map[string]any{})
	for _, bad := range []string{"yandex-verification", "gtag", "mc.yandex.ru", "context.js", "data-consent"} {
		if strings.Contains(empty, bad) {
			t.Errorf("no settings - no %s", bad)
		}
	}
}

// TestConsentSettings - the saved consent: a known mode and groups, the
// text trimmed, the "More" page a site path or an http(s) address, the
// version the stored one; ResetConsent raises it; a role without the
// code fields keeps their groups.
func TestConsentSettings(t *testing.T) {
	ctx := context.Background()
	repo := &fakeIntegrationRepo{s: entity.IntegrationSettings{Consent: entity.Consent{Version: 4, HeadGroup: entity.ConsentMarketing}}}
	s := NewIntegrationSettingsService(repo, slog.New(slog.NewTextHandler(io.Discard, nil)))
	in := entity.IntegrationSettings{Consent: entity.Consent{Enabled: true, Mode: "whatever", Text: "  Hi  ", PolicyURL: "/privacy",
		MetrikaGroup: "bogus", GAGroup: entity.ConsentAnalytics, Version: 99}}
	if err := s.Save(ctx, in, true); err != nil {
		t.Fatal(err)
	}
	c := repo.s.Consent
	if c.Mode != entity.ConsentAsk || c.Text != "Hi" || c.MetrikaGroup != entity.ConsentNone || c.GAGroup != entity.ConsentAnalytics || c.Version != 4 || c.HeadGroup != "" {
		t.Errorf("saved: %+v", c)
	}
	for _, bad := range []string{"//evil.example", "javascript:alert(1)", "privacy"} {
		in.Consent.PolicyURL = bad
		if err := s.Save(ctx, in, true); !errors.Is(err, ErrConsentPolicyURL) {
			t.Errorf("%q: %v", bad, err)
		}
	}
	in.Consent.PolicyURL = "https://example.com/privacy"
	repo.s.Consent.HeadGroup = entity.ConsentMarketing
	if err := s.Save(ctx, in, false); err != nil || repo.s.Consent.HeadGroup != entity.ConsentMarketing {
		t.Errorf("without the code fields: %v %+v", err, repo.s.Consent)
	}
	if err := s.ResetConsent(ctx); err != nil || repo.s.Consent.Version != 5 || s.Current(ctx).Consent.Version != 5 {
		t.Errorf("reset: %v %d", err, repo.s.Consent.Version)
	}
}
