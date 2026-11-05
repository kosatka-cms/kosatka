//go:build dbintegration

package app

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/testdb"
)

// TestCookieConsent - the banner turned on in "Counters and codes": the
// Yandex Metrica counter (analytics) waits on a guest's page as an inert
// template, the banner offers Analytics with the policy's link; "Ask
// everyone again" raises the version; turned off - the counter runs at
// once, no banner.
func TestCookieConsent(t *testing.T) {
	testdb.Each(t, func(t *testing.T, db testdb.DB) {
		s := serveSite(t, db)
		s.signIn()
		const page = "/admin/config/integrations"
		save := func(extra url.Values) *http.Response {
			t.Helper()
			form := url.Values{"csrf_token": {s.csrf(page)}, "metrika_id": {"12345678"}, "metrika_group": {"analytics"},
				"ga_group": {"analytics"}, "consent_mode": {"consent"}, "consent_policy_url": {"/privacy"}}
			for k, v := range extra {
				form[k] = v
			}
			return s.post(page, form)
		}
		if resp := save(url.Values{"consent_enabled": {"1"}}); !strings.Contains(resp.Header.Get("Location"), "saved=1") {
			t.Fatalf("saving: %s", resp.Header.Get("Location"))
		}
		_, home := s.visitor().get("/")
		for _, want := range []string{`<template data-consent="analytics">`, `data-consent-banner data-consent-version="1"`, `href="/privacy"`, `value="analytics" data-consent-group`} {
			if !strings.Contains(home, want) {
				t.Errorf("the guest's page has no %s", want)
			}
		}
		if resp := save(url.Values{"consent_enabled": {"1"}, "consent_reset": {"1"}}); !strings.Contains(resp.Header.Get("Location"), "saved=reset") {
			t.Errorf("reset: %s", resp.Header.Get("Location"))
		}
		if _, home := s.visitor().get("/"); !strings.Contains(home, `data-consent-version="2"`) {
			t.Error("the reset didn't raise the version")
		}
		save(nil)
		if _, home := s.visitor().get("/"); strings.Contains(home, "data-consent") || !strings.Contains(home, "mc.yandex.ru/metrika/tag.js") {
			t.Error("off: the counter waits or a banner shows")
		}
		s.checkLog()
	})
}
