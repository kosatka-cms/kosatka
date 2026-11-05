// Package htmlimages rewrites <img> tags in HTML content into resized-preview
// links (with PhotoSwipe captions), using an imagestyle.ResizeService.
package htmlimages

import (
	"bytes"
	"crypto/sha256"
	"regexp"
	"strconv"
	"strings"

	"github.com/kosatka-cms/kosatka/internal/lib/photoswipe"
	"github.com/kosatka-cms/kosatka/pkg/imagestyle"
	"golang.org/x/net/html"
)

// ContentProcessor rewrites content images into preview links.
type ContentProcessor struct {
	resizeService *imagestyle.ResizeService
	// texts - the rewritten texts: a text is shown again and again, and
	// rewriting it reads each image from the store (its size, whether its
	// preview is there).
	texts *textCache
	// lightbox - what the link opens of the original (SetLightbox); nil -
	// the original itself.
	lightbox LightboxFunc
}

// LightboxFunc - what the lightbox opens of an original at src (its
// address on the site) of width × height: an address and that image's
// size; ok false - the original itself.
type LightboxFunc func(src string, width, height int) (url string, w, h int, ok bool)

// textCacheBytes - the most the rewritten texts kept take.
const textCacheBytes = 32 << 20

// NewContentProcessor - a ContentProcessor making the previews with
// resizeService.
func NewContentProcessor(resizeService *imagestyle.ResizeService) *ContentProcessor {
	return &ContentProcessor{
		resizeService: resizeService,
		texts:         newTextCache(textCacheBytes),
	}
}

// Forget drops the rewritten texts kept: the previews they link to were
// deleted.
func (cp *ContentProcessor) Forget() { cp.texts.clear() }

// SetLightbox makes the links open what fn gives of an original (a large
// copy of it) instead of the original. Set it before the first text is
// rewritten.
func (cp *ContentProcessor) SetLightbox(fn LightboxFunc) { cp.lightbox = fn }

// ProcessImages replaces local <img> tags in htmlContent with resized
// previews wrapped in PhotoSwipe links. Unparseable HTML is returned as is.
// The result is kept for the same text, unless an image of it failed (its
// store unreachable, say: the next view tries again).
func (cp *ContentProcessor) ProcessImages(htmlContent string) string {
	if htmlContent == "" {
		return ""
	}
	key := sha256.Sum256([]byte(htmlContent))
	if out, ok := cp.texts.get(key); ok {
		return out
	}

	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return htmlContent
	}

	complete := cp.processNode(doc)

	var buf bytes.Buffer
	_ = html.Render(&buf, doc) // bytes.Buffer.Write never fails

	result := buf.String()

	// html.Parse wraps the fragment in html/head/body; strip them.
	result = strings.TrimPrefix(result, "<html><head></head><body>")
	result = strings.TrimSuffix(result, "</body></html>")

	if complete {
		cp.texts.put(key, result)
	}
	return result
}

// processNode walks the HTML tree, wrapping every <img>; false - an image
// failed.
func (cp *ContentProcessor) processNode(n *html.Node) bool {
	ok := true
	if n.Type == html.ElementNode && n.Data == "img" {
		ok = cp.wrapImageWithLink(n)
	}

	// Take NextSibling before recursing: wrapImageWithLink moves the <img>
	// into a new <a>, which clears its NextSibling and would skip the rest.
	for c := n.FirstChild; c != nil; {
		next := c.NextSibling
		if !cp.processNode(c) {
			ok = false
		}
		c = next
	}
	return ok
}

// wrapImageWithLink points imgNode at a resized preview and wraps it in a
// PhotoSwipe link to the original (or what SetLightbox gives of it) when
// the original is larger than shown;
// false - the image couldn't be read or resized (left as it is, or with
// guessed sizes).
func (cp *ContentProcessor) wrapImageWithLink(imgNode *html.Node) bool {
	src := cp.getAttr(imgNode, "src")
	prefix := cp.resizeService.URLPrefix()
	if src == "" || !strings.Contains(src, prefix) {
		// Only images from the local files directory.
		return true
	}
	// Normalize relative paths (../../<files dir>/...) to absolute.
	if i := strings.Index(src, prefix); i > 0 {
		src = src[i:]
	}

	widthStr := cp.getAttr(imgNode, "width")
	heightStr := cp.getAttr(imgNode, "height")

	width, _ := strconv.Atoi(widthStr)
	height, _ := strconv.Atoi(heightStr)

	// Fall back to dimensions in the style attribute.
	if width <= 0 || height <= 0 {
		styleWidth, styleHeight := cp.parseDimensionsFromStyle(imgNode)
		if styleWidth > 0 {
			width = styleWidth
		}
		if styleHeight > 0 {
			height = styleHeight
		}
	}

	// origWidth/origHeight are reused below for PhotoSwipe.
	var origWidth, origHeight int
	if width <= 0 || height <= 0 {
		ow, oh, err := cp.resizeService.GetOriginalDimensions(src)
		if err != nil {
			return false
		}
		origWidth, origHeight = ow, oh

		switch {
		case width <= 0 && height <= 0:
			// No size at all: use the original's.
			width, height = origWidth, origHeight
		case height <= 0:
			// Only width set (editors often resize this way): derive the
			// height from the original's aspect ratio, keeping the width.
			if origWidth > 0 {
				height = width * origHeight / origWidth
			} else {
				height = origHeight
			}
		default: // width <= 0
			if origHeight > 0 {
				width = height * origWidth / origHeight
			} else {
				width = origWidth
			}
		}
	}

	// Explicit width/height let the browser reserve space (avoids CLS).
	cp.setAttr(imgNode, "width", strconv.Itoa(width))
	cp.setAttr(imgNode, "height", strconv.Itoa(height))

	thumbnailURL, err := cp.resizeService.GetResizedImageURL(src, width, height)
	if err != nil {
		// No preview: leave the image untouched.
		return false
	}

	cp.setAttr(imgNode, "src", thumbnailURL)

	// Already inside <a>: preview only, no nested content-image-link.
	if cp.isInsideAnchor(imgNode) {
		return true
	}

	// Original dimensions for PhotoSwipe, unless already fetched above.
	complete := true
	if origWidth <= 0 || origHeight <= 0 {
		ow, oh, err := cp.resizeService.GetOriginalDimensions(src)
		if err != nil {
			origWidth = width * 2
			origHeight = height * 2
			complete = false
		} else {
			origWidth, origHeight = ow, oh
		}
	}
	// The original no larger than shown: nothing more to see in the
	// lightbox, no link.
	if complete && origWidth <= width && origHeight <= height {
		return true
	}

	title := strings.TrimSpace(cp.getAttr(imgNode, "title"))
	alt := strings.TrimSpace(cp.getAttr(imgNode, "alt"))

	href, boxWidth, boxHeight := cp.resizeService.OriginalURL(src), origWidth, origHeight
	if cp.lightbox != nil && complete {
		if u, w, h, ok := cp.lightbox(href, origWidth, origHeight); ok {
			href, boxWidth, boxHeight = u, w, h
		}
	}
	linkAttrs := []html.Attribute{
		{Key: "href", Val: href},
		{Key: "data-pswp-width", Val: strconv.Itoa(boxWidth)},
		{Key: "data-pswp-height", Val: strconv.Itoa(boxHeight)},
		{Key: "class", Val: "content-image-link"},
	}

	if capHTML := photoswipe.CaptionHTML(title, alt); capHTML != "" {
		linkAttrs = append(linkAttrs, html.Attribute{Key: "data-pswp-caption-html", Val: capHTML})
	}

	link := &html.Node{
		Type: html.ElementNode,
		Data: "a",
		Attr: linkAttrs,
	}

	// Put the link where the image was, with the image inside it.
	parent := imgNode.Parent
	if parent != nil {
		nextSibling := imgNode.NextSibling

		parent.RemoveChild(imgNode)

		link.AppendChild(imgNode)

		if nextSibling != nil {
			parent.InsertBefore(link, nextSibling)
		} else {
			parent.AppendChild(link)
		}
	}
	return complete
}

// isInsideAnchor reports whether n has an <a> ancestor (not necessarily the parent).
func (cp *ContentProcessor) isInsideAnchor(n *html.Node) bool {
	for p := n.Parent; p != nil; p = p.Parent {
		if p.Type == html.ElementNode && p.Data == "a" {
			return true
		}
	}
	return false
}

// parseDimensionsFromStyle extracts pixel width and height from the style
// attribute, e.g. "height: 335px; width: 220px; float: right;" -> (220, 335).
func (cp *ContentProcessor) parseDimensionsFromStyle(n *html.Node) (width, height int) {
	style := cp.getAttr(n, "style")
	if style == "" {
		return 0, 0
	}

	widthRe := regexp.MustCompile(`width:\s*(\d+)px`)
	heightRe := regexp.MustCompile(`height:\s*(\d+)px`)

	if matches := widthRe.FindStringSubmatch(style); len(matches) > 1 {
		width, _ = strconv.Atoi(matches[1])
	}

	if matches := heightRe.FindStringSubmatch(style); len(matches) > 1 {
		height, _ = strconv.Atoi(matches[1])
	}

	return width, height
}

// getAttr returns the value of n's attribute key, or "".
func (cp *ContentProcessor) getAttr(n *html.Node, key string) string {
	for _, attr := range n.Attr {
		if attr.Key == key {
			return attr.Val
		}
	}
	return ""
}

// setAttr sets n's attribute key, adding it if missing.
func (cp *ContentProcessor) setAttr(n *html.Node, key, val string) {
	for i, attr := range n.Attr {
		if attr.Key == key {
			n.Attr[i].Val = val
			return
		}
	}
	n.Attr = append(n.Attr, html.Attribute{Key: key, Val: val})
}
