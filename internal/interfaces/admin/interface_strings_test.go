package admin

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// TestInterfaceStringsFilter - the strings tab's filter from the address
// (a form's fields on a POST): a language the interface hasn't - the
// site's, an unknown "show" - all, a page below 1 - the first; its query
// back without the first page.
func TestInterfaceStringsFilter(t *testing.T) {
	for _, c := range []struct {
		query string
		want  service.InterfaceStringsFilter
		back  string
	}{
		{"lang=ru&q=+comment+&show=edited&from=core&page=3",
			service.InterfaceStringsFilter{Lang: "ru", Query: "comment", Show: "edited", From: "core", Page: 3, PerPage: interfaceStringsPerPage},
			"from=core&lang=ru&page=3&q=comment&show=edited"},
		{"lang=xx&show=all&page=-2",
			service.InterfaceStringsFilter{Lang: i18n.Default(), Page: 1, PerPage: interfaceStringsPerPage},
			"lang=" + i18n.Default()},
	} {
		r := httptest.NewRequest(http.MethodGet, "/admin/config/translations/strings?"+c.query, nil)
		got := interfaceStringsFilter(r)
		if got != c.want {
			t.Errorf("%s: %+v, want %+v", c.query, got, c.want)
		}
		if q := filterQuery(got).Encode(); q != c.back {
			t.Errorf("%s: back %s, want %s", c.query, q, c.back)
		}
	}
	r := httptest.NewRequest(http.MethodPost, "/admin/config/translations/strings?lang=en", strings.NewReader(url.Values{"lang": {"ru"}, "show": {"untranslated"}}.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if err := r.ParseForm(); err != nil {
		t.Fatal(err)
	}
	if f := interfaceStringsFilter(r); f.Lang != "ru" || f.Show != "untranslated" {
		t.Errorf("POST: %+v", f)
	}
}

// TestInterfaceStringError - why a translation was refused, for the
// page; an error not the translation's - "".
func TestInterfaceStringError(t *testing.T) {
	c := &Controller{}
	r := httptest.NewRequest(http.MethodPost, "/", nil)
	for err, want := range map[error]string{
		&i18n.TranslationError{Placeholder: ":login"}:           "The translation must keep :login: the value goes in its place.",
		&i18n.TranslationError{Tag: "<"}:                        `The translation has a tag not closed: "<" before a letter.`,
		fmt.Errorf("x: %w", &i18n.TranslationError{Tag: "<b>"}): "The translation can have only the tags of the English string, as they are: <b> isn't one.",
		service.ErrEmptyTranslation:                             "Enter the translation (every form of it).",
		service.ErrUnknownString:                                "The site has no such string.",
		fmt.Errorf("saving: broken"):                            "",
	} {
		if got := c.interfaceStringError(r, err); got != want {
			t.Errorf("%v: %q, want %q", err, got, want)
		}
	}
}

// TestInterfacePackRedirect - back to the Languages tab after an action:
// done with its figures, or the error's code (the service's known ones,
// else the action's own) with the language alone; each code's message.
func TestInterfacePackRedirect(t *testing.T) {
	c := &Controller{}
	for _, tc := range []struct {
		err      error
		fallback string
		want     string
	}{
		{nil, "download", "/admin/config/translations?done=downloaded&kept=3&lang=de"},
		{fmt.Errorf("x: %w", service.ErrPackChecksum), "download", "/admin/config/translations?error=checksum&lang=de"},
		{service.ErrPackInUse, "delete", "/admin/config/translations?error=in_use&lang=de"},
		{service.ErrPackNotInstalled, "delete", "/admin/config/translations?error=not_installed&lang=de"},
		{service.ErrUnknownPack, "download", "/admin/config/translations?error=unknown&lang=de"},
		{service.ErrNoTranslationsCatalog, "download", "/admin/config/translations?error=no_catalog&lang=de"},
		{fmt.Errorf("network down"), "download", "/admin/config/translations?error=download&lang=de"},
	} {
		w := httptest.NewRecorder()
		c.interfacePackRedirect(w, httptest.NewRequest(http.MethodPost, "/", nil), "de", url.Values{"done": {"downloaded"}, "kept": {"3"}}, tc.err, tc.fallback)
		if got := w.Header().Get("Location"); got != tc.want || w.Code != http.StatusSeeOther {
			t.Errorf("%v: %d %s, want %s", tc.err, w.Code, got, tc.want)
		}
	}
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	for code, want := range map[string]string{
		"":              "",
		"no_catalog":    "No translations catalog is configured.",
		"unknown":       "The catalog has no Deutsch translation to download.",
		"checksum":      "The Deutsch translation's file isn't what the catalog says (its checksum differs): nothing was installed.",
		"in_use":        "Deutsch is the site's interface language: choose another one in Site information first.",
		"not_installed": "Deutsch isn't a downloaded language.",
		"catalog":       "The translations catalog couldn't be read. Try again later.",
		"delete":        "The Deutsch translation couldn't be removed. Try again later.",
		"download":      "The Deutsch translation couldn't be installed. Try again later.",
	} {
		if got := c.interfacePackError(r, code, "Deutsch"); got != want {
			t.Errorf("%q: %q, want %q", code, got, want)
		}
	}
}
