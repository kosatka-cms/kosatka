package service

import (
	"fmt"
	"html"
	"html/template"
	"regexp"
	"strconv"
	"strings"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/imageurl"
)

// imgAttrName - an attribute name img takes: lowercase, no event
// handlers.
var imgAttrName = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// imgTag - the template function img: an <img> of p (imageurl.Picture)
// with alt (text, or a translation's HTML: t) - src, its srcset (2x),
// width and height when known, loading="lazy" and decoding="async";
// attrs - more attributes as name, value pairs ("class", "photo",
// "loading", "eager"…), which replace those it sets (width, height,
// loading, decoding), not src, srcset and alt. No picture - nothing.
func imgTag(p imageurl.Picture, alt any, attrs ...string) (template.HTML, error) {
	if p.Src == "" {
		return "", nil
	}
	if len(attrs)%2 != 0 {
		return "", fmt.Errorf("img: attributes go in name, value pairs, got %d values", len(attrs))
	}
	given := map[string]string{}
	var order []string
	for i := 0; i < len(attrs); i += 2 {
		name := attrs[i]
		if !imgAttrName.MatchString(name) || strings.HasPrefix(name, "on") || name == "src" || name == "srcset" || name == "alt" {
			return "", fmt.Errorf("img: attribute %q isn't allowed", name)
		}
		if _, dup := given[name]; !dup {
			order = append(order, name)
		}
		given[name] = attrs[i+1]
	}
	var b strings.Builder
	attr := func(name, value string) {
		b.WriteString(" " + name + `="` + template.HTMLEscapeString(value) + `"`)
	}
	own := func(name, value string) {
		if v, ok := given[name]; ok {
			value = v
			delete(given, name)
		}
		if value != "" {
			attr(name, value)
		}
	}
	b.WriteString("<img")
	attr("src", p.Src)
	if p.Srcset != "" {
		attr("srcset", p.Srcset)
	}
	own("width", positive(p.Width))
	own("height", positive(p.Height))
	attr("alt", altText(alt))
	own("loading", "lazy")
	own("decoding", "async")
	for _, name := range order {
		if v, ok := given[name]; ok {
			attr(name, v)
		}
	}
	b.WriteString(">")
	return template.HTML(b.String()), nil
}

// altText - alt as plain text: a string itself, HTML (a translation)
// unescaped, anything else printed.
func altText(alt any) string {
	switch v := alt.(type) {
	case string:
		return v
	case template.HTML:
		return html.UnescapeString(string(v))
	case nil:
		return ""
	}
	return fmt.Sprint(alt)
}

// positive - n as text, "" for 0 and less.
func positive(n int) string {
	if n <= 0 {
		return ""
	}
	return strconv.Itoa(n)
}

// styledPicture - the template function: image in an image style
// (imageurl.StyledPicture); style "" - the image itself
// (imageurl.OriginalPicture).
func styledPicture(image entity.File, style string) imageurl.Picture {
	if style == "" {
		return imageurl.OriginalPicture(image)
	}
	return imageurl.StyledPicture(image, style)
}

// lightboxPicture - imageurl.LightboxPicture of an image file, given as
// it is or by pointer (a gallery item's); nil - an empty Picture.
func lightboxPicture(image any) imageurl.Picture {
	switch f := image.(type) {
	case entity.File:
		return imageurl.LightboxPicture(f)
	case *entity.File:
		if f != nil {
			return imageurl.LightboxPicture(*f)
		}
	}
	return imageurl.Picture{}
}
