package feeds

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"slices"
	"strings"
	"sync/atomic"
	"time"

	"github.com/kosatka-cms/kosatka/modulekit"
)

// Feed errors (the admin form shows each as its own message).
var (
	ErrFeedLabel        = errors.New("feed label is required")
	ErrFeedPath         = errors.New("feed path: latin letters, digits, - _ . /")
	ErrFeedPathTaken    = errors.New("feed path is taken")
	ErrFeedContentTypes = errors.New("feed needs at least one content type")
	ErrFeedFieldMap     = errors.New("feed field map refers to a missing or unsuitable field")
)

var feedPathRe = regexp.MustCompile(`^[a-z0-9][a-z0-9._/-]{0,254}$`)

const feedDefaultItemLimit = 50

// feedContentTypes - what FeedService needs about content types.
type feedContentTypes interface {
	ContentTypes(ctx context.Context) (modulekit.ContentTypeDefs, error)
	FieldsByContentTypeID(ctx context.Context, contentTypeID int) (modulekit.FieldInstanceDefs, error)
}

// FeedService - the feeds module: feed settings and the nodes each feed
// includes.
type FeedService struct {
	repo         FeedRepository
	nodes        modulekit.NodesRepository
	contentTypes feedContentTypes
	// pathOwner - what else on the site answers a path ("" - nothing;
	// modulekit.PathOwner): a feed doesn't take it.
	pathOwner func(ctx context.Context, path string) (string, error)
	// typeEnabled - whether a content type's content is available (not a
	// disabled module's): a feed leaves the others out.
	typeEnabled func(nodeType string) bool
	// paths - the enabled feeds' addresses → their IDs: FeedByPath is
	// asked for every address no route answers. Loaded on the first ask,
	// dropped by a save or a delete.
	paths  atomic.Pointer[map[string]int]
	logger *slog.Logger
}

// NewFeedService - the feeds of repo, their content from nodes and
// contentTypes; pathOwner tells what else answers an address.
func NewFeedService(repo FeedRepository, nodes modulekit.NodesRepository, contentTypes feedContentTypes, pathOwner func(ctx context.Context, path string) (string, error), logger *slog.Logger) *FeedService {
	return &FeedService{repo: repo, nodes: nodes, contentTypes: contentTypes, pathOwner: pathOwner, typeEnabled: modulekit.NodeTypeEnabled, logger: logger}
}

// Feeds - every feed, by label.
func (s *FeedService) Feeds(ctx context.Context) ([]Feed, error) { return s.repo.Feeds(ctx) }

// FeedByID - the feed, enabled or not (ErrEntityNotFound - none).
func (s *FeedService) FeedByID(ctx context.Context, id int) (*Feed, error) {
	return s.repo.FeedByID(ctx, id)
}

// FeedByPath - the enabled feed at path (nil - none, or the feeds can't
// be read: logged).
func (s *FeedService) FeedByPath(ctx context.Context, path string) *Feed {
	paths, err := s.enabledPaths(ctx)
	if err != nil {
		modulekit.LogFallback(s.logger, err, "loading the feeds' addresses (no feed answers)")
		return nil
	}
	id, ok := paths[strings.Trim(path, "/")]
	if !ok {
		return nil
	}
	f, err := s.repo.FeedByID(ctx, id)
	if err != nil || f == nil || f.Status != 1 {
		modulekit.LogFallback(s.logger, err, "loading the feed (none answers)", "feedID", id)
		return nil
	}
	return f
}

// enabledPaths - the enabled feeds' addresses → their IDs, kept until a
// save or a delete.
func (s *FeedService) enabledPaths(ctx context.Context) (map[string]int, error) {
	if p := s.paths.Load(); p != nil {
		return *p, nil
	}
	feeds, err := s.repo.Feeds(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing the feeds: %w", err)
	}
	paths := make(map[string]int, len(feeds))
	for _, f := range feeds {
		if f.Status == 1 {
			paths[f.Path] = f.ID
		}
	}
	s.paths.Store(&paths)
	return paths, nil
}

// Save validates and creates (ID == 0) or updates f; returns its ID.
func (s *FeedService) Save(ctx context.Context, f Feed) (int, error) {
	defer s.paths.Store(nil)
	if err := s.normalize(ctx, &f); err != nil {
		return 0, err
	}
	if f.ID == 0 {
		return s.repo.CreateFeed(ctx, f)
	}
	return f.ID, s.repo.UpdateFeed(ctx, f)
}

// Delete deletes the feed id.
func (s *FeedService) Delete(ctx context.Context, id int) error {
	defer s.paths.Store(nil)
	return s.repo.DeleteFeed(ctx, id)
}

func (s *FeedService) normalize(ctx context.Context, f *Feed) error {
	f.Label = strings.TrimSpace(f.Label)
	if f.Label == "" {
		return ErrFeedLabel
	}
	f.Path = strings.ToLower(strings.Trim(strings.TrimSpace(f.Path), "/"))
	if !feedPathRe.MatchString(f.Path) || strings.Contains(f.Path, "//") || strings.Contains(f.Path, "..") {
		return ErrFeedPath
	}
	if other, err := s.repo.FeedByPath(ctx, f.Path); err == nil && other != nil && other.ID != f.ID {
		return ErrFeedPathTaken
	}
	// A new address only: a feed already there keeps it (saving its other
	// settings isn't refused).
	if s.pathOwner != nil && !s.keepsPath(ctx, *f) {
		owner, err := s.pathOwner(ctx, f.Path)
		if err != nil {
			return err
		}
		if owner != "" {
			return fmt.Errorf("%w (%s)", ErrFeedPathTaken, owner)
		}
	}
	f.Format = ParseFeedFormat(string(f.Format))
	if f.ItemLimit <= 0 {
		f.ItemLimit = feedDefaultItemLimit
	}
	f.ItemLimit = min(f.ItemLimit, FeedMaxItems)
	f.Days = max(f.Days, 0)
	f.Title, f.Description = strings.TrimSpace(f.Title), strings.TrimSpace(f.Description)
	f.Language, f.ImageStyle = strings.TrimSpace(f.Language), strings.TrimSpace(f.ImageStyle)
	if f.Status != 1 {
		f.Status = 0
	}

	defs, err := s.contentTypes.ContentTypes(ctx)
	if err != nil {
		return err
	}
	byName := make(map[string]modulekit.ContentTypeDef, len(defs))
	for _, d := range defs {
		byName[d.MachineName] = d
	}
	var types []string
	for _, t := range f.ContentTypes {
		if _, ok := byName[t]; ok && !slices.Contains(types, t) {
			types = append(types, t)
		}
	}
	if len(types) == 0 {
		return ErrFeedContentTypes
	}
	f.ContentTypes = types

	var maps FeedFieldMaps
	for _, t := range types {
		m := f.FieldMaps.For(t)
		fields, err := s.contentTypes.FieldsByContentTypeID(ctx, byName[t].ID)
		if err != nil {
			return err
		}
		typeOf := make(map[string]modulekit.ContentTypeFieldType, len(fields))
		for _, fld := range fields {
			typeOf[fld.MachineName] = fld.FieldType
		}
		for _, check := range []struct {
			name string
			want modulekit.ContentTypeFieldType
		}{{m.BodyField, modulekit.FieldTypeText}, {m.ImageField, modulekit.FieldTypeImage}, {m.CategoryField, modulekit.FieldTypeTaxonomy}} {
			if check.name != "" && typeOf[check.name] != check.want {
				return ErrFeedFieldMap
			}
		}
		maps = append(maps, m)
	}
	f.FieldMaps = maps
	return nil
}

// keepsPath - whether f is a saved feed whose address doesn't change.
func (s *FeedService) keepsPath(ctx context.Context, f Feed) bool {
	if f.ID == 0 {
		return false
	}
	current, err := s.repo.FeedByID(ctx, f.ID)
	return err == nil && current != nil && current.Path == f.Path
}

// Nodes - the nodes feed f emits now: its types and terms, within
// its day window (or the format's), newest first, at most ItemLimit.
func (s *FeedService) Nodes(ctx context.Context, f Feed, now time.Time) (modulekit.Nodes, error) {
	days := f.Days
	if days <= 0 {
		days = f.Format.DefaultDays()
	}
	// A disabled module's content doesn't open: the feed leaves it out.
	types := slices.DeleteFunc(slices.Clone(f.ContentTypes), func(t string) bool { return !s.typeEnabled(t) })
	if len(types) == 0 {
		return nil, nil
	}
	filter := modulekit.FeedNodeFilter{Types: types, TermIDs: f.TermIDs, Limit: f.ItemLimit}
	if days > 0 {
		filter.Since = now.Add(-time.Duration(days) * 24 * time.Hour)
	}
	return s.nodes.ListForFeed(ctx, filter)
}
