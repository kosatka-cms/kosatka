package service

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/pkg/sitetime"
	"github.com/kosatka-cms/kosatka/pkg/slug"
)

// DefaultNodePathPattern - the path pattern of a content type without its
// own (ContentTypeDef.PathPattern ""): the type's machine name and the
// title.
const DefaultNodePathPattern = "[node:type]/[node:title]"

// The errors ValidateNodePathPattern returns.
var (
	// ErrNodePathPatternChars - the pattern's text outside the tokens has
	// characters a URL alias can't (only a-z, 0-9, "-", "_", "." and "/").
	ErrNodePathPatternChars = errors.New("the path pattern may contain only lowercase Latin letters, digits, - _ . / and tokens")
	// ErrNodePathPatternToken - a token the pattern can't take: not one of
	// the node's, or not a taxonomy field of the type.
	ErrNodePathPatternToken = errors.New("unknown token in the path pattern")
)

// nodePathTerms - terms by ID with their vocabularies (implemented by
// TaxonomyService).
type nodePathTerms interface {
	TermsByIDs(ctx context.Context, ids []int) (map[int]entity.TaxonomyTerm, error)
}

// nodePathSlugs - whether another node has a slug (implemented by the
// nodes repository).
type nodePathSlugs interface {
	SlugExistsExcludingNode(ctx context.Context, slug string, excludeNodeID int) (bool, error)
}

// NodePathService builds a node's URL alias from its type's path pattern.
// Tokens, in the SEO templates' syntax ({…}
// optional groups and | alternatives too, renderSEOTemplate):
//   - [node:type] - the type's machine name;
//   - [node:title] - the title, transliterated;
//   - [node:created:Y], [node:created:m], [node:created:d] - the creation
//     date's year, month, day (the site's time zone);
//   - [node:field:<machine name>] - a taxonomy field's term: its path (its
//     URL without the vocabulary's prefix, so with its parents). Other
//     fields' values (texts, numbers…) make no addresses.
type NodePathService struct {
	terms nodePathTerms
	slugs nodePathSlugs
}

// NewNodePathService - the node path service on terms (the terms of the
// pattern's field tokens) and slugs (the other nodes' slugs, for a unique
// one).
func NewNodePathService(terms nodePathTerms, slugs nodePathSlugs) *NodePathService {
	return &NodePathService{terms: terms, slugs: slugs}
}

// Path - dc's URL alias by its type's pattern (def), unique among the
// other nodes (a taken one gets -1, -2…); dc.Node.ID is 0 for a new
// node, its CreatedAt zero - now.
func (s *NodePathService) Path(ctx context.Context, def entity.ContentTypeDef, dc entity.DynamicContent) (string, error) {
	pattern := def.PathPattern
	if strings.TrimSpace(pattern) == "" {
		pattern = DefaultNodePathPattern
	}
	created := dc.Node.CreatedAt
	if created.IsZero() {
		created = time.Now()
	}
	var tokenErr error
	noTitle := false
	rendered, _ := renderSEOTemplate(pattern, func(token string) string {
		value, err := s.token(ctx, token, def, dc, created)
		if err != nil && tokenErr == nil {
			tokenErr = err
		}
		if token == "node:title" && value == "" {
			noTitle = true
		}
		return value
	})
	if tokenErr != nil {
		return "", tokenErr
	}
	base := normalizeNodePath(rendered)
	if noTitle || base == "" {
		// Nothing to name the node by (a title of no Latin or Cyrillic
		// letters): the type and a timestamp.
		base = normalizeNodePath(def.MachineName + "/" + strconv.FormatInt(time.Now().Unix(), 10))
	}
	return s.unique(ctx, base, dc.Node.ID)
}

// token - one token's value for dc, ready for a URL ("" - none).
func (s *NodePathService) token(ctx context.Context, token string, def entity.ContentTypeDef, dc entity.DynamicContent, created time.Time) (string, error) {
	switch token {
	case "node:type":
		return def.MachineName, nil
	case "node:title":
		return slug.Generate(dc.Node.Title), nil
	case "node:created:Y":
		return sitetime.In(created).Format("2006"), nil
	case "node:created:m":
		return sitetime.In(created).Format("01"), nil
	case "node:created:d":
		return sitetime.In(created).Format("02"), nil
	}
	name, ok := strings.CutPrefix(token, "node:field:")
	if !ok {
		return "", nil
	}
	for _, f := range dc.Fields {
		if f.MachineName == name {
			return s.fieldValue(ctx, f, dc.Values[f.ID])
		}
	}
	return "", nil
}

// fieldValue - a taxonomy field's first term's path ("" - no term, or
// not a taxonomy field).
func (s *NodePathService) fieldValue(ctx context.Context, f entity.FieldInstanceDef, value any) (string, error) {
	if f.FieldType != entity.FieldTypeTaxonomy {
		return "", nil
	}
	ids := nodeTermIDs(value)
	if len(ids) == 0 || s.terms == nil {
		return "", nil
	}
	terms, err := s.terms.TermsByIDs(ctx, ids[:1])
	if err != nil {
		return "", fmt.Errorf("loading the term %d of [node:field:%s]: %w", ids[0], f.MachineName, err)
	}
	term, ok := terms[ids[0]]
	if !ok {
		return "", nil
	}
	return termRelPath(term), nil
}

// termRelPath - a term's URL without its vocabulary's prefix: the term
// and its parents as the term's own URL has them.
func termRelPath(term entity.TaxonomyTerm) string {
	prefix := NormalizeTermPathPrefix(term.Vocabulary.TermPathPrefix)
	if prefix == "" {
		prefix = NormalizeTermPathPrefix(term.Vocabulary.Name)
	}
	path := strings.Trim(term.Slug, "/")
	if prefix != "" {
		path = strings.TrimPrefix(strings.TrimPrefix(path, prefix), "/")
	}
	if path == "" {
		return slug.Generate(term.Name)
	}
	return path
}

// normalizeNodePath - a rendered pattern as a slug: lowercase, without
// empty segments (a token with no value) or a segment's edge hyphens.
func normalizeNodePath(path string) string {
	var segments []string
	for _, seg := range strings.Split(strings.ToLower(path), "/") {
		if seg = strings.Trim(seg, "-. "); seg != "" {
			segments = append(segments, seg)
		}
	}
	return strings.Join(segments, "/")
}

// unique - base, or base-1, base-2… the first no node but excludeNodeID
// has.
func (s *NodePathService) unique(ctx context.Context, base string, excludeNodeID int) (string, error) {
	for i := 0; i < 1000; i++ {
		candidate := base
		if i > 0 {
			candidate = fmt.Sprintf("%s-%d", base, i)
		}
		taken, err := s.slugs.SlugExistsExcludingNode(ctx, candidate, excludeNodeID)
		if err != nil {
			return "", fmt.Errorf("checking the slug %q: %w", candidate, err)
		}
		if !taken {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("no free slug for %q", base)
}

// nodePathToken - a token in a pattern: […] (its | alternatives split by
// the caller).
var nodePathToken = regexp.MustCompile(`\[([^\]]*)\]`)

// nodePathLiteral - what a pattern may have outside its tokens (the
// {…} groups' braces and the | between their variants included).
var nodePathLiteral = regexp.MustCompile(`^[a-z0-9/_.{}|-]*$`)

// ValidateNodePathPattern checks a type's path pattern: its tokens are the
// node's (NodePathService) or its taxonomy fields' (fields - the type's),
// the rest fits a URL. "" is valid (DefaultNodePathPattern).
func ValidateNodePathPattern(pattern string, fields entity.FieldInstanceDefs) error {
	known := map[string]bool{"node:type": true, "node:title": true, "node:created:Y": true, "node:created:m": true, "node:created:d": true}
	for _, f := range fields {
		if f.FieldType == entity.FieldTypeTaxonomy {
			known["node:field:"+f.MachineName] = true
		}
	}
	for _, m := range nodePathToken.FindAllStringSubmatch(pattern, -1) {
		for _, alt := range strings.Split(m[1], "|") {
			if alt = strings.TrimSpace(alt); !known[alt] {
				return fmt.Errorf("%w: [%s]", ErrNodePathPatternToken, alt)
			}
		}
	}
	if !nodePathLiteral.MatchString(nodePathToken.ReplaceAllString(pattern, "")) {
		return ErrNodePathPatternChars
	}
	return nil
}
