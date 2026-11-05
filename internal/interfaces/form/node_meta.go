package form

import (
	"net/http"
	"strings"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

// The longest meta title and meta description, in characters, that a
// node form keeps (ParseNodeMeta cuts the rest).
const (
	MetaTitleMaxLen       = 200
	MetaDescriptionMaxLen = entity.MaxMetaDesc
)

// NodeMetaFields are shared by node edit forms (edit_node_settings.html).
type NodeMetaFields struct {
	MetaTitle       string
	MetaDescription string
}

// NodeMetaFromPageSEO fills meta fields from persisted node SEO (may be empty).
func NodeMetaFromPageSEO(ps entity.PageSEO) NodeMetaFields {
	return NodeMetaFields{
		MetaTitle:       ps.Title,
		MetaDescription: ps.Description,
	}
}

// ParseNodeMeta reads meta_title and meta_description from the request.
func (n *NodeMetaFields) ParseNodeMeta(r *http.Request) {
	n.MetaTitle = strings.TrimSpace(r.FormValue("meta_title"))
	if len([]rune(n.MetaTitle)) > MetaTitleMaxLen {
		n.MetaTitle = string([]rune(n.MetaTitle)[:MetaTitleMaxLen])
	}
	n.MetaDescription = strings.TrimSpace(r.FormValue("meta_description"))
	if len([]rune(n.MetaDescription)) > MetaDescriptionMaxLen {
		n.MetaDescription = string([]rune(n.MetaDescription)[:MetaDescriptionMaxLen])
	}
}
