package entity

// MaxMetaDesc - the longest meta description, in characters.
const MaxMetaDesc = 160

// PageSEO - a page's head metadata: the title, the meta description, the
// canonical URL, the Open Graph image and URL, the image to preload and
// noindex.
type PageSEO struct {
	Title        string
	Description  string
	Canonical    string
	OGImage      string
	OGURL        string
	PreloadImage string // LCP image
	// PreloadImageSrcset - PreloadImage's srcset, as its <img> has it.
	PreloadImageSrcset string
	NoIndex            bool // pages without content of their own (login and the like): <meta name="robots" content="noindex, follow">
}
