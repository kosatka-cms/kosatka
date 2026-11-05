package entity

import (
	"net/url"
	"slices"
	"strings"
)

// Link - one value of a FieldTypeLink field: an absolute http(s) address
// and the text it shows with (Title; "" - the address itself).
type Link struct {
	URL   string `json:"url"`
	Title string `json:"title,omitempty"`
}

// Text - what the link shows: its title, else its address.
func (l Link) Text() string {
	if l.Title != "" {
		return l.Title
	}
	return l.URL
}

// Host - the link's host in lower case, without "www."; "" - not an
// absolute http(s) address.
func (l Link) Host() string {
	u, err := url.Parse(l.URL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return ""
	}
	return strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
}

// Links - a link field's value: its links in the editor's order (a field
// with one value holds at most one); none for anything else.
func Links(v any) []Link {
	links, _ := v.([]Link)
	return links
}

// LinkTitleMode - whether a link field asks for the links' titles.
type LinkTitleMode string

const (
	// LinkTitleNone - no title: the link shows its address (or what a
	// formatter makes of it, a video, say).
	LinkTitleNone LinkTitleMode = ""
	// LinkTitleOptional - a title may be given.
	LinkTitleOptional LinkTitleMode = "optional"
	// LinkTitleRequired - every link needs one.
	LinkTitleRequired LinkTitleMode = "required"
)

// LinkTitleModes - the choices of a link field's title setting, in the
// order shown.
var LinkTitleModes = []LinkTitleMode{LinkTitleNone, LinkTitleOptional, LinkTitleRequired}

// LinkTitle - FieldTypeLink: whether the links have titles (storage
// config "link_title"); an unknown value - none.
func (f FieldInstanceDef) LinkTitle() LinkTitleMode {
	m, _ := f.Config["link_title"].(string)
	if slices.Contains(LinkTitleModes, LinkTitleMode(m)) {
		return LinkTitleMode(m)
	}
	return LinkTitleNone
}

// AllowedHosts - FieldTypeLink: the sites its links may lead to (storage
// config "allowed_hosts"), lower case, without "www."; none - any site.
func (f FieldInstanceDef) AllowedHosts() []string {
	raw, _ := f.Config["allowed_hosts"].([]any)
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		if s, ok := item.(string); ok && s != "" {
			out = append(out, s)
		}
	}
	return out
}

// AllowedHostsString - AllowedHosts space-separated, the format of the
// field's "Allowed sites" input (ParseHosts).
func (f FieldInstanceDef) AllowedHostsString() string {
	return strings.Join(f.AllowedHosts(), " ")
}

// HostAllowed reports whether a link to host may be saved in the field:
// any host when it names none, else one of them or a subdomain of one.
func (f FieldInstanceDef) HostAllowed(host string) bool {
	allowed := f.AllowedHosts()
	if len(allowed) == 0 {
		return true
	}
	for _, a := range allowed {
		if host == a || strings.HasSuffix(host, "."+a) {
			return true
		}
	}
	return false
}

// ParseHosts - the hosts of an "Allowed sites" input (space, comma or
// newline separated; an address is cut to its host), lower case, without
// "www.", each once.
func ParseHosts(raw string) []string {
	var out []string
	for _, tok := range strings.FieldsFunc(raw, func(r rune) bool { return r == ' ' || r == ',' || r == '\n' || r == '\r' || r == '\t' }) {
		tok = strings.ToLower(strings.TrimSpace(tok))
		if strings.Contains(tok, "://") {
			if u, err := url.Parse(tok); err == nil {
				tok = u.Hostname()
			}
		}
		tok = strings.TrimPrefix(strings.Trim(tok, "/."), "www.")
		if tok != "" && !strings.ContainsAny(tok, "/?#@") && !slices.Contains(out, tok) {
			out = append(out, tok)
		}
	}
	return out
}

// FieldLayout - how a field with several values shows them, whatever
// shows each (the field's formatter, FieldFormatter).
type FieldLayout string

const (
	// FieldLayoutAll - all of them, one after another.
	FieldLayoutAll FieldLayout = ""
	// FieldLayoutFirst - only the first.
	FieldLayoutFirst FieldLayout = "first"
	// FieldLayoutFeatured - the first one large, the others small after
	// it (a formatter without a small form shows them as it shows the
	// first).
	FieldLayoutFeatured FieldLayout = "featured"
)

// The layouts of a date field's several dates (the others' too where
// they fit): only those not ended yet (all ended - the last), or the
// nearest of them and how many more.
const (
	FieldLayoutUpcoming FieldLayout = "upcoming"
	FieldLayoutNearest  FieldLayout = "nearest"
)

// FieldLayoutGrid - a blocks field's items as cards in a grid (the
// others' - one after another).
const FieldLayoutGrid FieldLayout = "grid"

// FieldLayouts - the choices of FieldLayout, in the order shown.
var FieldLayouts = []FieldLayout{FieldLayoutAll, FieldLayoutFeatured, FieldLayoutFirst, FieldLayoutUpcoming, FieldLayoutNearest, FieldLayoutGrid}

// ParseFieldLayout - s as a FieldLayout; unknown - FieldLayoutAll.
func ParseFieldLayout(s string) FieldLayout {
	if slices.Contains(FieldLayouts, FieldLayout(s)) {
		return FieldLayout(s)
	}
	return FieldLayoutAll
}
