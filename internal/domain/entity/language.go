package entity

import (
	"strings"
	"sync"
)

// SiteLanguage - a language of the site besides its default one: its code
// (i18n, "en") and the prefix of its addresses ("en" - /en/…).
type SiteLanguage struct {
	Code   string `json:"code"`
	Prefix string `json:"prefix"`
}

// The site's languages (SetSiteLanguages): the default one, its addresses
// without a prefix, and the others. None besides the default - one
// language, no prefixes anywhere; so too while on (SetLanguagesSwitch)
// says they are off.
var siteLanguages struct {
	mu     sync.RWMutex
	def    string
	others []SiteLanguage
	on     func() bool
}

// SetLanguagesSwitch sets what says whether the site's languages besides
// the default one are in use (the languages module is enabled); none set -
// they are not.
func SetLanguagesSwitch(on func() bool) {
	siteLanguages.mu.Lock()
	defer siteLanguages.mu.Unlock()
	siteLanguages.on = on
}

// activeOthers - the other languages in use; the caller holds the lock.
func activeOthers() []SiteLanguage {
	if siteLanguages.on == nil || !siteLanguages.on() {
		return nil
	}
	return siteLanguages.others
}

// SetSiteLanguages sets the site's languages: def - the default one,
// others - with their prefixes (one equal to def, or without a prefix, is
// left out).
func SetSiteLanguages(def string, others []SiteLanguage) {
	var kept []SiteLanguage
	for _, l := range others {
		l.Prefix = strings.Trim(l.Prefix, "/")
		if l.Code != "" && l.Code != def && l.Prefix != "" {
			kept = append(kept, l)
		}
	}
	siteLanguages.mu.Lock()
	defer siteLanguages.mu.Unlock()
	siteLanguages.def, siteLanguages.others = def, kept
}

// SiteLanguages - the default language and the others in use
// (SetSiteLanguages, SetLanguagesSwitch).
func SiteLanguages() (string, []SiteLanguage) {
	siteLanguages.mu.RLock()
	defer siteLanguages.mu.RUnlock()
	return siteLanguages.def, append([]SiteLanguage(nil), activeOthers()...)
}

// LanguagePrefix - the prefix of code's addresses, "en"; "" for the
// default language or one the site hasn't.
func LanguagePrefix(code string) string {
	siteLanguages.mu.RLock()
	defer siteLanguages.mu.RUnlock()
	for _, l := range activeOthers() {
		if l.Code == code {
			return l.Prefix
		}
	}
	return ""
}

// LanguageOfPrefix - the language whose addresses start with prefix;
// false for none.
func LanguageOfPrefix(prefix string) (string, bool) {
	siteLanguages.mu.RLock()
	defer siteLanguages.mu.RUnlock()
	for _, l := range activeOthers() {
		if l.Prefix == prefix {
			return l.Code, true
		}
	}
	return "", false
}

// LocalPath - path ("/news/x", "/") in code's addresses: with its prefix
// ("/en/news/x", "/en/"), as is for the default language.
func LocalPath(code, path string) string {
	p := LanguagePrefix(code)
	if p == "" {
		return path
	}
	return "/" + p + "/" + strings.TrimPrefix(path, "/")
}

// MoveDefaultLanguage - others once newDef becomes the default language
// instead of oldDef: newDef leaves them, oldDef joins them with its code
// as the prefix (its addresses move under /<code>/).
func MoveDefaultLanguage(others []SiteLanguage, oldDef, newDef string) []SiteLanguage {
	out := make([]SiteLanguage, 0, len(others)+1)
	for _, l := range others {
		if l.Code != newDef && l.Code != oldDef {
			out = append(out, l)
		}
	}
	if oldDef != "" && oldDef != newDef {
		out = append(out, SiteLanguage{Code: oldDef, Prefix: oldDef})
	}
	return out
}

// LocalizeSelfURL - u, a page's own address (its canonical, og:url: the
// site URL and a path) under code's prefix when its path is current, the
// request's path without its language prefix (a list's, a View's, the
// front page's); any other u - content with its language's address - as
// it is.
func LocalizeSelfURL(code, u, current string) string {
	domain := NewSystemSettings().Domain
	base, query, hasQuery := strings.Cut(u, "?")
	if current == "" || base != domain+current {
		return u
	}
	out := domain + LocalPath(code, current)
	if hasQuery {
		out += "?" + query
	}
	return out
}

// LocalizePath - an address of the site (path, "/news?page=2") for a page
// in code: LocalPath's, unless it isn't the site's own ("", "//host",
// "https://…", "#…") or already starts with a language's prefix.
func LocalizePath(code, path string) string {
	if !strings.HasPrefix(path, "/") || strings.HasPrefix(path, "//") {
		return path
	}
	first, _, _ := strings.Cut(strings.TrimPrefix(path, "/"), "/")
	first, _, _ = strings.Cut(first, "?")
	if _, ok := LanguageOfPrefix(first); ok {
		return path
	}
	return LocalPath(code, path)
}
