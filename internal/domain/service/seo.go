package service

import (
	"context"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/imageurl"
	"github.com/kosatka-cms/kosatka/internal/lib/dateformat"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
	"github.com/kosatka-cms/kosatka/pkg/metadescription"
)

// seoTermNames - term names by ID (implemented by TaxonomyService).
type seoTermNames interface {
	TermsByIDs(ctx context.Context, ids []int) (map[int]entity.TaxonomyTerm, error)
}

// seoTaxonomy - what SEOService needs from TaxonomyService.
type seoTaxonomy interface {
	breadcrumbTermsLoader
	seoTermNames
}

// SEOService - node page title/description from token templates:
// the node's own override, else the first
// matching SEO rule, else its type's template, else the default template,
// else the built-in fallback (interfaces.PageSEO).
type SEOService struct {
	// uow - a reorder in one transaction (SetUnitOfWork; nil - none).
	uow       entity.UnitOfWork
	templates entity.SEOTemplateRepository
	rules     entity.SEORuleRepository
	taxonomy  seoTaxonomy
	siteName  func(ctx context.Context) string
	// files - an image field's file by ID, for [node:field:<image>]
	// (SetFiles); nil - image tokens render empty.
	files func(ctx context.Context, id int) (*entity.File, error)
	// dateFormats - [node:field:<date>:<format>] (SetDateFormats); nil -
	// only the default d.m.Y, :year and :iso.
	dateFormats *DateFormatService
	// site - the site's information for the JSON-LD (SetSite); nil - no
	// publisher or organization.
	site   func(ctx context.Context) (*entity.SiteInformation, error)
	logger *slog.Logger

	// mu/cache - templates and rules, loaded once and reset by every
	// write below (read on every node page).
	mu         sync.Mutex
	cacheTpl   entity.SEOTemplates
	cacheRules entity.SEORuleDefs
}

// NewSEOService - the SEO service on templates and rules (their storage),
// taxonomy (the terms of the tokens and rules), siteName (the [site:name]
// token) and logger.
func NewSEOService(templates entity.SEOTemplateRepository, rules entity.SEORuleRepository, taxonomy seoTaxonomy, siteName func(ctx context.Context) string, logger *slog.Logger) *SEOService {
	return &SEOService{templates: templates, rules: rules, taxonomy: taxonomy, siteName: siteName, logger: logger.With("component", "seo.service")}
}

// Templates - all templates by scope (cached).
func (s *SEOService) Templates(ctx context.Context) (entity.SEOTemplates, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cacheTpl != nil {
		return s.cacheTpl, nil
	}
	tpl, err := s.templates.SEOTemplates(ctx)
	if err != nil {
		return nil, err
	}
	s.cacheTpl = tpl
	return tpl, nil
}

// Rules - all rules by weight (cached).
func (s *SEOService) Rules(ctx context.Context) (entity.SEORuleDefs, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cacheRules != nil {
		return s.cacheRules, nil
	}
	rules, err := s.rules.SEORules(ctx)
	if err != nil {
		return nil, err
	}
	s.cacheRules = rules
	return rules, nil
}

func (s *SEOService) reset() {
	s.mu.Lock()
	s.cacheTpl, s.cacheRules = nil, nil
	s.mu.Unlock()
}

// SaveTemplate saves the templates of t's scope, trimmed (a property's
// template left empty is dropped).
func (s *SEOService) SaveTemplate(ctx context.Context, t entity.SEOTemplate) error {
	defer s.reset()
	t.TitleTemplate = strings.TrimSpace(t.TitleTemplate)
	t.DescriptionTemplate = strings.TrimSpace(t.DescriptionTemplate)
	t.ImageTemplate = strings.TrimSpace(t.ImageTemplate)
	t.SchemaType = strings.TrimSpace(t.SchemaType)
	for name, tpl := range t.SchemaProps {
		if tpl = strings.TrimSpace(tpl); tpl == "" {
			delete(t.SchemaProps, name)
		} else {
			t.SchemaProps[name] = tpl
		}
	}
	return s.templates.SaveSEOTemplate(ctx, t)
}

// RuleByID - the SEO rule id.
func (s *SEOService) RuleByID(ctx context.Context, id int) (*entity.SEORuleDef, error) {
	return s.rules.SEORuleByID(ctx, id)
}

// CreateRule adds r at the end of the list (weight = max + 10).
func (s *SEOService) CreateRule(ctx context.Context, r entity.SEORuleDef) (*entity.SEORuleDef, error) {
	rules, err := s.Rules(ctx)
	if err != nil {
		return nil, err
	}
	r.Weight = lastWeight(rules, func(x entity.SEORuleDef) int { return x.Weight })
	defer s.reset()
	return s.rules.CreateSEORule(ctx, r)
}

// UpdateRule saves rule r (and drops the cached rules).
func (s *SEOService) UpdateRule(ctx context.Context, r entity.SEORuleDef) error {
	defer s.reset()
	return s.rules.UpdateSEORule(ctx, r)
}

// ReorderRules sets weights 0, 10, 20… in the order of ids.
func (s *SEOService) ReorderRules(ctx context.Context, ids []int) error {
	defer s.reset()
	return reorder(ctx, s.uow, ids, s.rules.UpdateSEORuleWeight, "the SEO rules")
}

// SetUnitOfWork sets the transactions a reorder runs in.
func (s *SEOService) SetUnitOfWork(uow entity.UnitOfWork) { s.uow = uow }

// DeleteRule deletes rule id (and drops the cached rules).
func (s *SEOService) DeleteRule(ctx context.Context, id int) error {
	defer s.reset()
	return s.rules.DeleteSEORule(ctx, id)
}

// Apply puts the templated title and description of dc's page onto p
// (built by the built-in fallback), unless the node has its own; a
// template rendering empty leaves p's value.
func (s *SEOService) Apply(ctx context.Context, dc entity.DynamicContent, p entity.PageSEO) entity.PageSEO {
	titleTpl, descTpl := s.templatesFor(ctx, dc)
	if strings.TrimSpace(dc.Node.PageSEO.Title) == "" && titleTpl != "" {
		if t := strings.Join(strings.Fields(s.Render(ctx, titleTpl, dc)), " "); t != "" {
			p.Title = t
		}
	}
	if strings.TrimSpace(dc.Node.PageSEO.Description) == "" && descTpl != "" {
		if d := metadescription.Generate(s.Render(ctx, descTpl, dc), entity.MaxMetaDesc); d != "" {
			p.Description = d
		}
	}
	if imageTpl := s.imageTemplateFor(ctx, dc); imageTpl != "" {
		if uri := strings.TrimSpace(s.Render(ctx, imageTpl, dc)); uri != "" {
			if strings.HasPrefix(uri, "/") {
				uri = strings.TrimSuffix(entity.NewSystemSettings().Domain, "/") + uri
			}
			p.OGImage = uri
		}
	}
	return p
}

// SetFiles gives image tokens their files (wiring).
func (s *SEOService) SetFiles(files func(ctx context.Context, id int) (*entity.File, error)) {
	s.files = files
}

// imageTemplateFor - the og:image template for dc: its type's, else the
// default one.
func (s *SEOService) imageTemplateFor(ctx context.Context, dc entity.DynamicContent) string {
	tpl, err := s.Templates(ctx)
	if err != nil {
		LogFallback(s.logger, err, "error loading seo templates")
		return ""
	}
	if t := tpl[entity.SEOTemplateTypeScope(string(dc.Node.Type))].ImageTemplate; t != "" {
		return t
	}
	return tpl[entity.SEOTemplateDefaultScope].ImageTemplate
}

// ApplyTerm puts the templated title and description of a taxonomy term
// page onto p: the vocabulary's template, else the term pages' default
// (per part); a template rendering empty leaves p's value. Tokens:
// [term:name], [term:title] (PageTitle, else Name), [term:description],
// [site:name].
func (s *SEOService) ApplyTerm(ctx context.Context, term entity.TaxonomyTerm, p entity.PageSEO) entity.PageSEO {
	tpl, err := s.Templates(ctx)
	if err != nil {
		LogFallback(s.logger, err, "error loading seo templates")
		return p
	}
	var titleTpl, descTpl string
	for _, l := range []entity.SEOTemplate{tpl[entity.SEOTemplateVocabularyScope(term.Vocabulary.ID)], tpl[entity.SEOTemplateTermScope]} {
		if titleTpl == "" {
			titleTpl = l.TitleTemplate
		}
		if descTpl == "" {
			descTpl = l.DescriptionTemplate
		}
	}
	resolve := func(token string) string { return s.termToken(ctx, token, term) }
	if titleTpl != "" {
		if out, _ := renderSEOTemplate(titleTpl, resolve); strings.TrimSpace(out) != "" {
			p.Title = strings.Join(strings.Fields(out), " ")
		}
	}
	if descTpl != "" {
		out, _ := renderSEOTemplate(descTpl, resolve)
		if d := metadescription.Generate(out, entity.MaxMetaDesc); d != "" {
			p.Description = d
		}
	}
	return p
}

// termToken - one token's value for a term page.
func (s *SEOService) termToken(ctx context.Context, token string, term entity.TaxonomyTerm) string {
	switch token {
	case "term:name":
		return strings.TrimSpace(term.Name)
	case "term:title":
		if t := strings.TrimSpace(term.PageTitle); t != "" {
			return t
		}
		return strings.TrimSpace(term.Name)
	case "term:description":
		return strings.TrimSpace(term.Desc)
	case "site:name":
		if s.siteName != nil {
			return s.siteName(ctx)
		}
	}
	return ""
}

// templatesFor - the title and description templates for dc: per part,
// the first non-empty of the first matching rule, the type's template,
// the default template.
func (s *SEOService) templatesFor(ctx context.Context, dc entity.DynamicContent) (string, string) {
	var levels []entity.SEOTemplate
	if rule := s.matchingRule(ctx, dc); rule != nil {
		levels = append(levels, entity.SEOTemplate{TitleTemplate: rule.TitleTemplate, DescriptionTemplate: rule.DescriptionTemplate})
	}
	if tpl, err := s.Templates(ctx); err != nil {
		LogFallback(s.logger, err, "error loading seo templates")
	} else {
		levels = append(levels, tpl[entity.SEOTemplateTypeScope(string(dc.Node.Type))], tpl[entity.SEOTemplateDefaultScope])
	}
	var title, desc string
	for _, l := range levels {
		if title == "" {
			title = l.TitleTemplate
		}
		if desc == "" {
			desc = l.DescriptionTemplate
		}
	}
	return title, desc
}

func (s *SEOService) matchingRule(ctx context.Context, dc entity.DynamicContent) *entity.SEORuleDef {
	rules, err := s.Rules(ctx)
	if err != nil {
		LogFallback(s.logger, err, "error loading seo rules")
		return nil
	}
	of := func(r entity.SEORuleDef) (string, *int, func(int) bool) {
		return r.ContentType, r.FieldStorageID, r.HasTerm
	}
	fieldTerms := func(storageID int) (int, []int) { return dcFieldTerms(dc, storageID) }
	if i := matchRule(ctx, rules, of, string(dc.Node.Type), fieldTerms, s.taxonomy, s.logger); i >= 0 {
		return &rules[i]
	}
	return nil
}

// Render - tpl with dc's tokens: [token] or [token|token…] (the first
// non-empty); {…} - dropped when a token inside renders empty, {A|B|…} -
// the first variant whose tokens all render. Unknown tokens render
// empty.
func (s *SEOService) Render(ctx context.Context, tpl string, dc entity.DynamicContent) string {
	out, _ := renderSEOTemplate(tpl, func(token string) string { return s.token(ctx, token, dc) })
	return strings.TrimSpace(out)
}

// renderSEOTemplate - tpl rendered with resolve; ok is false when a token
// rendered empty.
func renderSEOTemplate(tpl string, resolve func(string) string) (string, bool) {
	var b strings.Builder
	ok := true
	for i := 0; i < len(tpl); {
		switch tpl[i] {
		case '{':
			if end := strings.IndexByte(tpl[i+1:], '}'); end >= 0 {
				for _, variant := range splitSEOVariants(tpl[i+1 : i+1+end]) {
					if inner, innerOK := renderSEOTemplate(variant, resolve); innerOK {
						b.WriteString(inner)
						break
					}
				}
				i += end + 2
				continue
			}
		case '[':
			if end := strings.IndexByte(tpl[i+1:], ']'); end >= 0 {
				value := ""
				for _, alt := range strings.Split(tpl[i+1:i+1+end], "|") {
					if value = resolve(strings.TrimSpace(alt)); value != "" {
						break
					}
				}
				if value == "" {
					ok = false
				}
				b.WriteString(value)
				i += end + 2
				continue
			}
		}
		b.WriteByte(tpl[i])
		i++
	}
	return b.String(), ok
}

// splitSEOVariants - a {…} group's variants, split on | outside […].
func splitSEOVariants(group string) []string {
	var variants []string
	depth, start := 0, 0
	for i := 0; i < len(group); i++ {
		switch group[i] {
		case '[':
			depth++
		case ']':
			if depth > 0 {
				depth--
			}
		case '|':
			if depth == 0 {
				variants = append(variants, group[start:i])
				start = i + 1
			}
		}
	}
	return append(variants, group[start:])
}

// token - one token's value for dc.
func (s *SEOService) token(ctx context.Context, token string, dc entity.DynamicContent) string {
	switch token {
	case "node:title":
		return strings.TrimSpace(dc.Node.Title)
	case "node:summary":
		return metadescription.Generate(dc.GetContent(), entity.MaxMetaDesc)
	case "site:name":
		if s.siteName != nil {
			return s.siteName(ctx)
		}
		return ""
	}
	if rest, found := strings.CutPrefix(token, "node:field:"); found {
		name, modifier, _ := strings.Cut(rest, ":")
		return s.fieldToken(ctx, dc, name, modifier)
	}
	return ""
}

// SetDateFormats lets date tokens take a configured date format as the
// modifier (wiring).
func (s *SEOService) SetDateFormats(d *DateFormatService) {
	s.dateFormats = d
}

// fieldToken - a field's value by machine name: text as is, taxonomy
// terms' names joined with ", ", a date as 02.01.2006 (modifier "year":
// its year; "iso": ISO 8601 in the site's time zone, for the JSON-LD; a
// date format's machine name: the date in that format, e.g.
// [node:field:<date field>:medium]), a number as is.
func (s *SEOService) fieldToken(ctx context.Context, dc entity.DynamicContent, machineName, modifier string) string {
	for _, f := range dc.Fields {
		if f.MachineName != machineName {
			continue
		}
		value := dc.Values[f.ID]
		if f.FieldType == entity.FieldTypeTaxonomy {
			return s.termNames(ctx, nodeTermIDs(value))
		}
		if f.FieldType == entity.FieldTypeImage {
			return s.imageURI(ctx, value)
		}
		switch v := value.(type) {
		case string:
			return strings.TrimSpace(v)
		case time.Time:
			if v.IsZero() {
				return ""
			}
			if modifier == "year" {
				return strconv.Itoa(v.Year())
			}
			if modifier == "iso" {
				return schemaTime(v)
			}
			if modifier != "" && s.dateFormats != nil {
				if pattern, ok := s.dateFormats.Lookup(ctx, modifier); ok {
					return dateformat.Format(v, pattern, i18n.FromContext(ctx))
				}
			}
			return v.Format("02.01.2006")
		case float64:
			return strconv.FormatFloat(v, 'f', -1, 64)
		case int:
			return strconv.Itoa(v)
		}
		return ""
	}
	return ""
}

// imageURI - an image field value's original file path ("" - none).
func (s *SEOService) imageURI(ctx context.Context, value any) string {
	// A field with several images: its first.
	id := entity.FirstFileID(value)
	if id <= 0 || s.files == nil {
		return ""
	}
	file, err := s.files(ctx, id)
	if err != nil || file == nil {
		return ""
	}
	return imageurl.Original(*file).URI
}

func (s *SEOService) termNames(ctx context.Context, ids []int) string {
	if len(ids) == 0 || s.taxonomy == nil {
		return ""
	}
	terms, err := s.taxonomy.TermsByIDs(ctx, ids)
	if err != nil {
		LogFallback(s.logger, err, "error loading terms for a seo token")
		return ""
	}
	names := make([]string, 0, len(ids))
	for _, id := range ids {
		if name := strings.TrimSpace(terms[id].Name); name != "" {
			names = append(names, name)
		}
	}
	return strings.Join(names, ", ")
}
