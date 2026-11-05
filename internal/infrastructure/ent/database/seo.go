package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/seorule"
)

// SEORepository - seo_templates and seo_rules (entity.SEOTemplateRepository,
// entity.SEORuleRepository).
type SEORepository struct {
	client *generated.Client
}

// NewSEORepository - the SEO templates and rules repository on db.
func NewSEORepository(db *sql.DB) *SEORepository {
	return &SEORepository{client: clientFor(db)}
}

// SEOTemplates - every saved scope's templates, by scope.
func (r *SEORepository) SEOTemplates(ctx context.Context) (entity.SEOTemplates, error) {
	rows, err := GetClient(ctx, r.client).SeoTemplate.Query().All(ctx)
	if err != nil {
		return nil, fmt.Errorf("SEORepository.SEOTemplates: %w", err)
	}
	out := entity.SEOTemplates{}
	for _, t := range rows {
		out[t.ID] = entity.SEOTemplate{Scope: t.ID, TitleTemplate: t.TitleTemplate, DescriptionTemplate: t.DescriptionTemplate, ImageTemplate: t.ImageTemplate,
			SchemaType: t.SchemaType, SchemaProps: t.SchemaProps}
	}
	return out, nil
}

// SaveSEOTemplate saves a scope's templates, creating the row if it isn't
// there.
func (r *SEORepository) SaveSEOTemplate(ctx context.Context, t entity.SEOTemplate) error {
	client := GetClient(ctx, r.client)
	err := client.SeoTemplate.UpdateOneID(t.Scope).
		SetTitleTemplate(t.TitleTemplate).SetDescriptionTemplate(t.DescriptionTemplate).SetImageTemplate(t.ImageTemplate).
		SetSchemaType(t.SchemaType).SetSchemaProps(t.SchemaProps).
		Exec(ctx)
	if !generated.IsNotFound(err) {
		return err
	}
	return client.SeoTemplate.Create().SetID(t.Scope).
		SetTitleTemplate(t.TitleTemplate).SetDescriptionTemplate(t.DescriptionTemplate).SetImageTemplate(t.ImageTemplate).
		SetSchemaType(t.SchemaType).SetSchemaProps(t.SchemaProps).
		Exec(ctx)
}

func toSEORule(s *generated.SeoRule) entity.SEORuleDef {
	return entity.SEORuleDef{
		ID: s.ID, ContentType: s.ContentType, FieldStorageID: s.FieldStorageID, TermIDs: s.TermIds,
		TitleTemplate: s.TitleTemplate, DescriptionTemplate: s.DescriptionTemplate, Weight: s.Weight,
	}
}

// SEORules - see entity.SEORuleRepository.SEORules.
func (r *SEORepository) SEORules(ctx context.Context) (entity.SEORuleDefs, error) {
	rows, err := GetClient(ctx, r.client).SeoRule.Query().
		Order(generated.Asc(seorule.FieldWeight), generated.Asc(seorule.FieldID)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("SEORepository.SEORules: %w", err)
	}
	out := entity.SEORuleDefs{}
	for _, s := range rows {
		out = append(out, toSEORule(s))
	}
	return out, nil
}

// SEORuleByID - the rule with id; ErrEntityNotFound if there is none.
func (r *SEORepository) SEORuleByID(ctx context.Context, id int) (*entity.SEORuleDef, error) {
	s, err := GetClient(ctx, r.client).SeoRule.Get(ctx, id)
	if err != nil {
		return nil, notFoundAs(err, "seo rule with id %d", id)
	}
	rule := toSEORule(s)
	return &rule, nil
}

// CreateSEORule adds the rule and returns it with its ID.
func (r *SEORepository) CreateSEORule(ctx context.Context, rule entity.SEORuleDef) (*entity.SEORuleDef, error) {
	now := time.Now()
	s, err := GetClient(ctx, r.client).SeoRule.Create().
		SetContentType(rule.ContentType).SetNillableFieldStorageID(rule.FieldStorageID).SetTermIds(rule.TermIDs).
		SetTitleTemplate(rule.TitleTemplate).SetDescriptionTemplate(rule.DescriptionTemplate).SetWeight(rule.Weight).
		SetCreatedAt(now).SetUpdatedAt(now).
		Save(ctx)
	if err != nil {
		return nil, err
	}
	rule.ID = s.ID
	return &rule, nil
}

// UpdateSEORule saves rule over the rule with its ID.
func (r *SEORepository) UpdateSEORule(ctx context.Context, rule entity.SEORuleDef) error {
	u := GetClient(ctx, r.client).SeoRule.Update().Where(seorule.ID(rule.ID)).
		SetContentType(rule.ContentType).SetTermIds(rule.TermIDs).
		SetTitleTemplate(rule.TitleTemplate).SetDescriptionTemplate(rule.DescriptionTemplate).SetWeight(rule.Weight).
		SetUpdatedAt(time.Now())
	if rule.FieldStorageID != nil {
		u.SetFieldStorageID(*rule.FieldStorageID)
	} else {
		u.ClearFieldStorageID()
	}
	_, err := u.Save(ctx)
	return err
}

// UpdateSEORuleWeight sets only the rule's weight (its place in the
// order).
func (r *SEORepository) UpdateSEORuleWeight(ctx context.Context, id, weight int) error {
	_, err := GetClient(ctx, r.client).SeoRule.Update().Where(seorule.ID(id)).SetWeight(weight).SetUpdatedAt(time.Now()).Save(ctx)
	return err
}

// DeleteSEORule deletes the rule with id.
func (r *SEORepository) DeleteSEORule(ctx context.Context, id int) error {
	_, err := GetClient(ctx, r.client).SeoRule.Delete().Where(seorule.ID(id)).Exec(ctx)
	return err
}
