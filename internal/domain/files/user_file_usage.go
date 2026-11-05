package files

import (
	"context"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/kosatka-cms/kosatka/internal/paths"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// userFileLinkRe finds links to users' files in HTML: the path after the
// public files URL (paths.PublicFilesURL) that starts with a user folder
// (u<id>/...).
func userFileLinkRe() *regexp.Regexp {
	return regexp.MustCompile(regexp.QuoteMeta(paths.PublicFilesURL()) + `(u[0-9]+/[^"'\s<>?#()\\]+)`)
}

// extractUserFilePaths - the users' files a text links to, each once
// ("u42/docs/a.pdf"), URL-decoded.
func extractUserFilePaths(text string) []string {
	if !strings.Contains(text, paths.PublicFilesURL()+"u") {
		return nil
	}
	seen := map[string]bool{}
	var out []string
	for _, m := range userFileLinkRe().FindAllStringSubmatch(text, -1) {
		p := m[1]
		if dec, err := url.PathUnescape(p); err == nil {
			p = dec
		}
		p = strings.TrimRight(p, ".,;:")
		if len(p) > 400 || !utf8.ValidString(p) || seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	return out
}

// UserFileUsageService finds where users' files are linked from (texts
// of content, comments, forum, messages, signatures, blocks, Views,
// quotes). It reads the texts on demand - before a file is deleted or
// renamed - and keeps no index.
type UserFileUsageService struct {
	repo entity.UserFileUsageRepository
}

// NewUserFileUsageService - the users' files' usage on repo (the texts
// that may link them).
func NewUserFileUsageService(repo entity.UserFileUsageRepository) *UserFileUsageService {
	return &UserFileUsageService{repo: repo}
}

// Refs - where a user's file (rel, relative to the user's folder) is used
// right now, read from the texts themselves.
func (s *UserFileUsageService) Refs(ctx context.Context, userID int, rel string) ([]entity.UserFileRef, error) {
	path := UserDirName(userID) + "/" + strings.Trim(rel, "/")
	var usages []entity.UserFileUsage
	err := s.repo.ScanTextsFor(ctx, path, func(source string, id int, text string) {
		for _, p := range extractUserFilePaths(text) {
			if p == path {
				usages = append(usages, entity.UserFileUsage{Path: p, Source: source, EntityID: id})
				return
			}
		}
	})
	if err != nil || len(usages) == 0 {
		return nil, err
	}
	refs, err := s.repo.Describe(ctx, usages)
	if err != nil {
		return nil, err
	}
	for i := range refs {
		refs[i].Title = userFileRefLabel(ctx, refs[i])
	}
	sort.SliceStable(refs, func(i, j int) bool { return refs[i].Source < refs[j].Source })
	return refs, nil
}

// userFileRefLabel - how a place of use reads for people.
func userFileRefLabel(ctx context.Context, ref entity.UserFileRef) string {
	title := ref.Title
	if title == "" {
		title = "#" + strconv.Itoa(ref.EntityID)
	}
	switch ref.Source {
	case entity.UserFileSourceComment:
		return i18n.T(ctx, "Comment on \"@title\"", "@title", title)
	case entity.UserFileSourceSignature:
		return i18n.T(ctx, "Signature of @name", "@name", title)
	case entity.UserFileSourceUserField:
		return i18n.T(ctx, "Profile of @name", "@name", title)
	case entity.UserFileSourceBlock:
		return i18n.T(ctx, "Block \"@title\"", "@title", title)
	case entity.UserFileSourceView:
		return i18n.T(ctx, "View \"@title\"", "@title", title)
	default:
		// A module's text: its label (Module.FileSources).
		if s, ok := entity.ModuleFileSourceByKey(ref.Source); ok {
			return i18n.T(ctx, s.Label, "@title", title)
		}
		return i18n.T(ctx, "Content \"@title\"", "@title", title)
	}
}
