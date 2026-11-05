package form

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"unicode/utf8"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// Limits of a link (the field_value_link columns).
const (
	linkURLMaxLen   = 2048
	linkTitleMaxLen = 255
)

// ParseLinkRows - the links of a link field's rows as submitted:
// field_<id>_url and field_<id>_title, one pair per row in the form's
// order (the order saved). A row without an address and a title is left
// out; an address without a scheme gets https://.
func ParseLinkRows(r *http.Request, fieldID int) []entity.Link {
	urls := r.Form[fmt.Sprintf("field_%d_url", fieldID)]
	titles := r.Form[fmt.Sprintf("field_%d_title", fieldID)]
	var links []entity.Link
	for i, u := range urls {
		l := entity.Link{URL: strings.TrimSpace(u)}
		if i < len(titles) {
			l.Title = strings.TrimSpace(titles[i])
		}
		if l.URL == "" && l.Title == "" {
			continue
		}
		if l.URL != "" && !strings.Contains(l.URL, "://") && !strings.HasPrefix(l.URL, "/") && strings.Contains(l.URL, ".") {
			l.URL = "https://" + l.URL
		}
		links = append(links, l)
	}
	return links
}

// validateLinks - what's wrong with a link field's links ("" - nothing):
// required and none, more than it holds, an address that isn't an
// absolute http(s) one or leads to a site the field doesn't allow, a
// title missing where required, too long values.
func validateLinks(ctx context.Context, field entity.FieldInstanceDef, links []entity.Link) string {
	if len(links) == 0 {
		if field.Required && !field.HiddenInForm {
			return i18n.T(ctx, "@field is required", "@field", field.Label)
		}
		return ""
	}
	limit := 1
	if field.Multiple() {
		limit = field.MaxValues()
	}
	if limit > 0 && len(links) > limit {
		return i18n.Plural(ctx, limit, "@field: at most @count link.", "@field: at most @count links.", "@field", field.Label)
	}
	for _, l := range links {
		u, err := url.Parse(l.URL)
		if l.URL == "" || err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return i18n.T(ctx, "@field: \"@url\" is not a web address (it starts with http:// or https://).", "@field", field.Label, "@url", l.URL)
		}
		if len(l.URL) > linkURLMaxLen {
			return i18n.T(ctx, "@field: the address is too long.", "@field", field.Label)
		}
		if host := l.Host(); !field.HostAllowed(host) {
			return i18n.T(ctx, "@field: @host is not among the sites allowed here (@hosts).", "@field", field.Label, "@host", host, "@hosts", strings.Join(field.AllowedHosts(), ", "))
		}
		switch {
		case field.LinkTitle() == entity.LinkTitleRequired && l.Title == "":
			return i18n.T(ctx, "@field: every link needs a title.", "@field", field.Label)
		case utf8.RuneCountInString(l.Title) > linkTitleMaxLen:
			return i18n.Plural(ctx, linkTitleMaxLen, "@field: a title is at most @count character.", "@field: a title is at most @count characters.", "@field", field.Label)
		}
	}
	return ""
}
