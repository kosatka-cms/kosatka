package database

import (
	"context"
	"fmt"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/database/dao"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/breadcrumbruledef"
)

// NewBreadcrumbRuleRepository - the breadcrumb rule repository on client.
func NewBreadcrumbRuleRepository(client *generated.Client) *BreadcrumbRuleRepository {
	return &BreadcrumbRuleRepository{client: client}
}

// BreadcrumbRuleRepository is the ent adapter for entity.BreadcrumbRuleRepository.
type BreadcrumbRuleRepository struct {
	client *generated.Client
}

// BreadcrumbRules - see entity.BreadcrumbRuleRepository.BreadcrumbRules.
func (r BreadcrumbRuleRepository) BreadcrumbRules(ctx context.Context) (entity.BreadcrumbRuleDefs, error) {
	res, err := GetClient(ctx, r.client).BreadcrumbRuleDef.Query().
		Order(generated.Asc(breadcrumbruledef.FieldWeight), generated.Asc(breadcrumbruledef.FieldID)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("BreadcrumbRuleRepository.BreadcrumbRules: %w", err)
	}
	out := make(entity.BreadcrumbRuleDefs, 0, len(res))
	for _, v := range res {
		out = append(out, dao.ToBreadcrumbRuleDefEntity(v))
	}
	return out, nil
}

// BreadcrumbRuleByID - the rule with id; ErrEntityNotFound if there is
// none.
func (r BreadcrumbRuleRepository) BreadcrumbRuleByID(ctx context.Context, id int) (*entity.BreadcrumbRuleDef, error) {
	res, err := GetClient(ctx, r.client).BreadcrumbRuleDef.Get(ctx, id)
	return one(res, err, dao.ToBreadcrumbRuleDefEntity, "breadcrumb rule with id %d", id)
}

// CreateBreadcrumbRule adds the rule d and returns it with its ID.
func (r BreadcrumbRuleRepository) CreateBreadcrumbRule(ctx context.Context, d entity.BreadcrumbRuleDef) (*entity.BreadcrumbRuleDef, error) {
	now := time.Now()
	res, err := GetClient(ctx, r.client).BreadcrumbRuleDef.Create().
		SetContentType(d.ContentType).
		SetNillableFieldStorageID(d.FieldStorageID).
		SetTermIds(d.TermIDs).
		SetMenuLinkID(d.MenuLinkID).
		SetWeight(d.Weight).
		SetCreatedAt(now).
		SetUpdatedAt(now).
		Save(ctx)
	if err != nil {
		return nil, err
	}
	created := dao.ToBreadcrumbRuleDefEntity(res)
	return &created, nil
}

// UpdateBreadcrumbRule saves d over the rule d.ID and returns it.
func (r BreadcrumbRuleRepository) UpdateBreadcrumbRule(ctx context.Context, d entity.BreadcrumbRuleDef) (*entity.BreadcrumbRuleDef, error) {
	upd := GetClient(ctx, r.client).BreadcrumbRuleDef.UpdateOneID(d.ID).
		SetContentType(d.ContentType).
		SetTermIds(d.TermIDs).
		SetMenuLinkID(d.MenuLinkID).
		SetWeight(d.Weight).
		SetUpdatedAt(time.Now())
	if d.FieldStorageID != nil {
		upd = upd.SetFieldStorageID(*d.FieldStorageID)
	} else {
		upd = upd.ClearFieldStorageID()
	}
	res, err := upd.Save(ctx)
	return one(res, err, dao.ToBreadcrumbRuleDefEntity, "breadcrumb rule with id %d", d.ID)
}

// UpdateBreadcrumbRuleWeight sets only the rule's weight (its place in
// the order).
func (r BreadcrumbRuleRepository) UpdateBreadcrumbRuleWeight(ctx context.Context, id, weight int) error {
	return GetClient(ctx, r.client).BreadcrumbRuleDef.UpdateOneID(id).
		SetWeight(weight).
		SetUpdatedAt(time.Now()).
		Exec(ctx)
}

// DeleteBreadcrumbRule deletes the rule with id.
func (r BreadcrumbRuleRepository) DeleteBreadcrumbRule(ctx context.Context, id int) error {
	return GetClient(ctx, r.client).BreadcrumbRuleDef.DeleteOneID(id).Exec(ctx)
}
