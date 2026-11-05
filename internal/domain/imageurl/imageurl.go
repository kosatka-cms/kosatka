// Package imageurl builds the web paths of public files (public:// URIs)
// and of their image-style derivatives.
package imageurl

import (
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/paths"
	"github.com/kosatka-cms/kosatka/pkg/imagestyle"
)

// styleFormats - image style name → derivative format, for the URLs
// built here: a JPEG style's derivative ends in .jpg. Kept in sync by
// the image style service (startup and every admin change); a style not
// listed is WebP.
var (
	styleFormatsMu sync.RWMutex
	styleFormats   = map[string]imagestyle.Format{}
)

// SetStyleFormats replaces the whole style → format map.
func SetStyleFormats(formats map[string]imagestyle.Format) {
	styleFormatsMu.Lock()
	defer styleFormatsMu.Unlock()
	styleFormats = make(map[string]imagestyle.Format, len(formats))
	for name, f := range formats {
		styleFormats[name] = f
	}
}

// SetStyleFormat records one style's format (FormatWebP removes
// the entry - the default).
func SetStyleFormat(name string, f imagestyle.Format) {
	styleFormatsMu.Lock()
	defer styleFormatsMu.Unlock()
	if f == imagestyle.FormatWebP {
		delete(styleFormats, name)
		return
	}
	styleFormats[name] = f
}

// styleVersions - image style name → its settings' version, added to the
// derivative's URL (?v=…): derivatives are served for a year as
// immutable, so a style changed gets new URLs and browsers and CDNs fetch
// the new derivatives. Kept in sync by the image style service like
// styleFormats; a style not listed has a URL without the version.
var (
	styleVersionsMu sync.RWMutex
	styleVersions   = map[string]string{}
)

// SetStyleVersions replaces the whole style → version map.
func SetStyleVersions(versions map[string]string) {
	styleVersionsMu.Lock()
	defer styleVersionsMu.Unlock()
	styleVersions = make(map[string]string, len(versions))
	for name, v := range versions {
		styleVersions[name] = v
	}
}

// SetStyleVersion records one style's version ("" removes the entry).
func SetStyleVersion(name, version string) {
	styleVersionsMu.Lock()
	defer styleVersionsMu.Unlock()
	if version == "" {
		delete(styleVersions, name)
		return
	}
	styleVersions[name] = version
}

func styleVersion(name string) string {
	styleVersionsMu.RLock()
	defer styleVersionsMu.RUnlock()
	return styleVersions[name]
}

func styleFormat(name string) imagestyle.Format {
	styleFormatsMu.RLock()
	defer styleFormatsMu.RUnlock()
	if f, ok := styleFormats[name]; ok {
		return f
	}
	return imagestyle.FormatWebP
}

// Styled returns the web path of a styled image, e.g.
// "images/foo.jpg" + "thumbnail" →
// "/uploads/styles/thumbnail/images/foo.webp" (.jpg for a
// JPEG style), with the style's version when it has one
// ("…/foo.webp?v=1a2b3c4d", SetStyleVersions). No image - an empty File: the picture of an empty field is
// the field's own default image (entity.FieldInstanceDef.DefaultImageID),
// resolved by the caller. Always the site's address, also with a store
// serving its files itself: a derivative is made on its first request,
// and only the site can make it (it redirects to the store then).
func Styled(image entity.File, styleName string) entity.File {
	if image.URI == "" {
		return entity.File{}
	}
	newImage := image
	newImage.URI = withFocal(styledURL(image.URI, styleName, false), image, styleName)
	return newImage
}

// styledURL - the address of uri's derivative in styleName (Styled);
// double - its 2x one (imagestyle.DoublePath).
func styledURL(uri, styleName string, double bool) string {
	if key, ok := privateKey(uri); ok {
		return privateStyledURL(key, styleName, double)
	}
	relativePath := derivativeRelPath(RelPath(uri), styleFormat(styleName).Extension())
	if double {
		relativePath = imagestyle.DoublePath(relativePath)
	}
	return withStyleVersion(fmt.Sprintf("%sstyles/%s/%s", paths.PublicFilesURL(), styleName, relativePath), styleName)
}

// withStyleVersion - u with styleName's version (SetStyleVersions), if
// it has one.
func withStyleVersion(u, styleName string) string {
	if v := styleVersion(styleName); v != "" {
		return u + "?v=" + v
	}
	return u
}

// DerivativeRelPaths - every derivative file name a source may have
// under a style dir (.webp and .jpg), for cleanup on file delete.
func DerivativeRelPaths(relativePath string) []string {
	return []string{
		derivativeRelPath(relativePath, imagestyle.FormatWebP.Extension()),
		derivativeRelPath(relativePath, imagestyle.FormatJPEG.Extension()),
	}
}

func derivativeRelPath(relativePath, newExt string) string {
	ext := filepath.Ext(relativePath)
	if ext == "" {
		return relativePath + newExt
	}
	return strings.TrimSuffix(relativePath, ext) + newExt
}

// AvatarProfileStyle - the avatar style on the profile page.
const AvatarProfileStyle = "user_avatar_profile"

// Avatar builds the path to avatar's file in another style.
// Accepts a URI already passed through Styled
// (…/styles/{any}/…) as well as a raw public://… or sites/… one.
func Avatar(avatar entity.File, styleName string) entity.File {
	if styleName == "" {
		styleName = AvatarProfileStyle
	}
	if avatar.URI == "" {
		return Styled(avatar, styleName)
	}
	// A static asset (the engine's placeholder, entity.AvatarPlaceholderURL)
	// isn't an uploaded file: no image style applies.
	if strings.HasPrefix(avatar.URI, "/static/") {
		return avatar
	}
	path := strings.TrimPrefix(avatar.URI, "/")
	// A styled URI's version (Styled) isn't part of the file's path.
	path, _, _ = strings.Cut(path, "?")
	stylesPrefix := paths.PublicFilesRel() + "styles/"
	if strings.HasPrefix(path, stylesPrefix) {
		rest := strings.TrimPrefix(path, stylesPrefix)
		if i := strings.Index(rest, "/"); i > 0 && i+1 < len(rest) {
			rel := rest[i+1:]
			return Styled(entity.File{URI: "public://" + rel}, styleName)
		}
	}
	return Styled(avatar, styleName)
}

// RelPath - the file's path in the public files (its store key) from
// any form of its address: public://…, /uploads/… (with or
// without the leading slash) or the store's own address
// (paths.PublicStoreURL). Built addresses are fed back here (a file's
// original restyled, Original applied twice), so every form maps back.
func RelPath(uri string) string {
	if base := paths.PublicStoreURL(); base != "" {
		if rel, ok := strings.CutPrefix(uri, base); ok {
			return rel
		}
	}
	rel := strings.TrimPrefix(uri, "/")
	rel = strings.TrimPrefix(rel, "public://")
	return strings.TrimPrefix(rel, paths.PublicFilesRel())
}

// FileURL - the address a file at rel (its store key) is served at: the
// store's own (paths.PublicStoreURL, e.g. a bucket's CDN) when it serves
// its files itself, else the site's /uploads/<rel>.
func FileURL(rel string) string {
	if base := paths.PublicStoreURL(); base != "" {
		return base + rel
	}
	return paths.PublicFilesURL() + rel
}

// Original resolves image.URI into the address of the file itself
// (FileURL; a private:// file's PrivateFileURL). The URI may be
// public://… or an address already built (some callers apply this
// twice): RelPath reads every form, which makes the function idempotent.
func Original(image entity.File) entity.File {
	if image.URI == "" {
		return entity.File{}
	}

	newImage := image
	switch {
	case strings.HasPrefix(image.URI, entity.PrivateScheme):
		newImage.URI = PrivateFileURL(strings.TrimPrefix(image.URI, entity.PrivateScheme))
	case strings.HasPrefix(image.URI, paths.PrivateFilesURL):
	default:
		newImage.URI = FileURL(RelPath(image.URI))
	}

	return newImage
}

// PrivateFileURL - the site's address of the private file key: only the
// site serves it, after its access check (never the store itself).
func PrivateFileURL(key string) string {
	return paths.PrivateFilesURL + key
}

// privateStylesDir - the folder of a private image's style address
// (privateStyledURL), under PrivateFilesURL.
const privateStylesDir = "styles/"

// privateStyledURL - the site's address of the private image key in
// styleName: PrivateFilesURL + styles/<style>/<key> (the original's own
// key, extension included, which the site checks access by), with the
// style's version; double - the 2x derivative's: PrivateFilesURL +
// styles/<style>/@2x/<key>. The site makes the derivative on its first
// request.
func privateStyledURL(key, styleName string, double bool) string {
	if double {
		key = imagestyle.DoublePath(key)
	}
	return withStyleVersion(paths.PrivateFilesURL+privateStylesDir+styleName+"/"+key, styleName)
}

// ParsePrivateStyled - the style, whether it's the 2x derivative and
// the original's key of a private image's style address path (under
// PrivateFilesURL, privateStyledURL); false for any other path.
func ParsePrivateStyled(rel string) (style string, double bool, key string, ok bool) {
	rest, ok := strings.CutPrefix(rel, privateStylesDir)
	if !ok {
		return "", false, "", false
	}
	style, key, ok = strings.Cut(rest, "/")
	double, key = imagestyle.SplitDouble(key)
	return style, double, key, ok && style != "" && key != ""
}

// privateKey - the private file's key of uri: private://<key>, or an
// address already built (PrivateFileURL, privateStyledURL).
func privateKey(uri string) (string, bool) {
	if key, ok := strings.CutPrefix(uri, entity.PrivateScheme); ok {
		return key, true
	}
	rel, ok := strings.CutPrefix(uri, paths.PrivateFilesURL)
	if !ok {
		return "", false
	}
	rel, _, _ = strings.Cut(rel, "?")
	if _, _, key, styled := ParsePrivateStyled(rel); styled {
		return key, true
	}
	return rel, true
}

// PublicURI converts a stored File.URI into the file's address
// (FileURL). Unlike Original, only a public:// URI is converted;
// anything else (e.g. a seeded default under /static/..., see
// entity.SiteInformation.Logo) is returned unchanged. Empty stays empty.
func PublicURI(uri string) string {
	if uri == "" {
		return ""
	}
	if rel, ok := strings.CutPrefix(uri, "public://"); ok {
		return FileURL(rel)
	}
	if key, ok := strings.CutPrefix(uri, entity.PrivateScheme); ok {
		return PrivateFileURL(key)
	}
	return uri
}
