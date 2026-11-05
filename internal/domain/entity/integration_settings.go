package entity

import (
	"context"
	"slices"
	"strconv"
	"strings"
)

// IntegrationSettings - external services wired into every page
// (/admin/config/integrations): site verification
// meta tags, Yandex Metrika, Google Analytics, and raw code for the rest
// (ad loaders etc.), on one page. One row.
type IntegrationSettings struct {
	// VerificationTags - one "name=content" per line, each a
	// <meta name="…" content="…"> in <head> (yandex-verification,
	// google-site-verification, msvalidate.01…).
	VerificationTags string
	// MetrikaID - the Yandex Metrika counter number ("" - none) and its
	// options.
	MetrikaID             string
	MetrikaClickmap       bool
	MetrikaTrackLinks     bool
	MetrikaAccurateBounce bool
	MetrikaWebvisor       bool
	// GAID - the Google Analytics measurement ID, "G-…" ("" - none).
	GAID string
	// HeadCode / BodyEndCode - raw HTML/JS put as is at the end of <head>
	// and right before </body>; editable by full-access roles only.
	HeadCode    string
	BodyEndCode string
	// BodyEndSkipServicePages - leave BodyEndCode off service pages
	// (admin pages, 404), e.g. for fullscreen ads.
	BodyEndSkipServicePages bool
	// Consent - the visitors' consent to the codes: the banner and each
	// code's group.
	Consent Consent
}

// ConsentGroup - a group of codes a visitor allows or not: ""
// (ConsentNone) - needs no consent, runs on every page.
type ConsentGroup string

// The consent groups: none (always runs), analytics, marketing.
const (
	ConsentNone      ConsentGroup = ""
	ConsentAnalytics ConsentGroup = "analytics"
	ConsentMarketing ConsentGroup = "marketing"
)

// ConsentGroups - the groups a code can be in, in the banner's order.
var ConsentGroups = []ConsentGroup{ConsentNone, ConsentAnalytics, ConsentMarketing}

// ParseConsentGroup - g if it's one of ConsentGroups, else ConsentNone.
func ParseConsentGroup(g string) ConsentGroup {
	for _, k := range ConsentGroups {
		if string(k) == g {
			return k
		}
	}
	return ConsentNone
}

// ConsentMode - what the banner asks: ConsentAsk - the codes of a group
// run only once the visitor allows it; ConsentNotice - they run at once,
// the banner tells about them.
type ConsentMode string

// The banner's modes (ConsentMode).
const (
	ConsentAsk    ConsentMode = "consent"
	ConsentNotice ConsentMode = "notice"
)

// Consent - the cookie banner (IntegrationSettings.Consent): on or off,
// its mode, its text ("" - the default, in the site's language), its
// "More" page, its version (raised - every visitor is asked again), and
// each code's group.
type Consent struct {
	Enabled      bool
	Mode         ConsentMode
	Text         string
	PolicyURL    string
	Version      int
	MetrikaGroup ConsentGroup
	GAGroup      ConsentGroup
	HeadGroup    ConsentGroup
	BodyEndGroup ConsentGroup
}

// Waits - whether a code of group g waits for the visitor's consent: the
// banner on, asking, g a group.
func (c Consent) Waits(g ConsentGroup) bool {
	return c.Enabled && c.Mode == ConsentAsk && g != ConsentNone
}

// AskedGroups - the groups the banner offers: those of the site's codes
// (on, in s) that wait for consent, in ConsentGroups' order.
func (s IntegrationSettings) AskedGroups() []ConsentGroup {
	c := s.Consent
	var used []ConsentGroup
	if s.MetrikaCounter() != 0 {
		used = append(used, c.MetrikaGroup)
	}
	if s.GAID != "" {
		used = append(used, c.GAGroup)
	}
	if strings.TrimSpace(s.HeadCode) != "" {
		used = append(used, c.HeadGroup)
	}
	if strings.TrimSpace(s.BodyEndCode) != "" {
		used = append(used, c.BodyEndGroup)
	}
	var out []ConsentGroup
	for _, g := range ConsentGroups {
		if g != ConsentNone && slices.Contains(used, g) {
			out = append(out, g)
		}
	}
	return out
}

// MetaTag - a <meta name content> pair.
type MetaTag struct {
	Name    string
	Content string
}

// VerificationMetas - VerificationTags parsed ("name=content" lines;
// blank and malformed lines skipped).
func (s IntegrationSettings) VerificationMetas() []MetaTag {
	var out []MetaTag
	for line := range strings.SplitSeq(s.VerificationTags, "\n") {
		name, content, ok := strings.Cut(strings.TrimSpace(line), "=")
		name, content = strings.TrimSpace(name), strings.TrimSpace(content)
		if ok && name != "" && content != "" {
			out = append(out, MetaTag{Name: name, Content: content})
		}
	}
	return out
}

// MetrikaCounter - MetrikaID as a number (0 - none or not a number).
func (s IntegrationSettings) MetrikaCounter() int {
	n, err := strconv.Atoi(strings.TrimSpace(s.MetrikaID))
	if err != nil || n <= 0 {
		return 0
	}
	return n
}

// IntegrationSettingsRepository stores the integration settings row.
type IntegrationSettingsRepository interface {
	Get(ctx context.Context) (*IntegrationSettings, error)
	Update(ctx context.Context, s IntegrationSettings) error
}
