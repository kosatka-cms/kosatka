package database

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/database/dao"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/contenttypedef"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/fieldinstancedef"
)

// NewContentTypeRepository - the content type repository on client.
func NewContentTypeRepository(client *generated.Client) *ContentTypeRepository {
	return &ContentTypeRepository{client: client}
}

// ContentTypeRepository is the ent adapter for entity.ContentTypeRepository
// (runtime content type builder; for the field storage/instance split see
// entity/field_storage.go).
type ContentTypeRepository struct {
	client *generated.Client
}

// ContentTypes - see entity.ContentTypeRepository.ContentTypes; by label.
func (r ContentTypeRepository) ContentTypes(ctx context.Context) (entity.ContentTypeDefs, error) {
	return r.contentTypes(ctx, false)
}

// FieldSets - see entity.ContentTypeRepository.FieldSets.
func (r ContentTypeRepository) FieldSets(ctx context.Context) (entity.ContentTypeDefs, error) {
	return r.contentTypes(ctx, true)
}

// contentTypes - the content types (embedded false) or the field sets
// (true), by label.
func (r ContentTypeRepository) contentTypes(ctx context.Context, embedded bool) (entity.ContentTypeDefs, error) {
	client := GetClient(ctx, r.client)
	res, err := client.ContentTypeDef.Query().
		Where(contenttypedef.Embedded(embedded)).
		Order(generated.Asc(contenttypedef.FieldLabel)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing the content types (embedded %v): %w", embedded, err)
	}

	cts := make(entity.ContentTypeDefs, len(res))
	for i, v := range res {
		cts[i] = dao.ToContentTypeDefEntity(v)
	}
	return cts, nil
}

// ContentTypeByID - the content type with id; ErrEntityNotFound if there
// is none.
func (r ContentTypeRepository) ContentTypeByID(ctx context.Context, id int) (*entity.ContentTypeDef, error) {
	client := GetClient(ctx, r.client)
	res, err := client.ContentTypeDef.Get(ctx, id)
	return one(res, err, dao.ToContentTypeDefEntity, "content type with id %d", id)
}

// ContentTypeByMachineName - the content type named machineName;
// ErrEntityNotFound if there is none.
func (r ContentTypeRepository) ContentTypeByMachineName(ctx context.Context, machineName string) (*entity.ContentTypeDef, error) {
	client := GetClient(ctx, r.client)
	res, err := client.ContentTypeDef.Query().Where(contenttypedef.MachineName(machineName)).Only(ctx)
	return one(res, err, dao.ToContentTypeDefEntity, "content type with machine_name %q", machineName)
}

// CreateContentType adds the content type ct (settings the create form
// doesn't send keep their column defaults) and returns it with its ID.
func (r ContentTypeRepository) CreateContentType(ctx context.Context, ct entity.ContentTypeDef) (*entity.ContentTypeDef, error) {
	client := GetClient(ctx, r.client)
	now := time.Now()
	create := client.ContentTypeDef.Create().
		SetMachineName(ct.MachineName).
		SetLabel(ct.Label).
		SetPathPattern(ct.PathPattern).
		SetDescription(ct.Description).
		SetStatus(ct.Status).
		SetCreatedBy(ct.CreatedBy).
		SetCreatedAt(now).
		SetUpdatedAt(now)
	// DefaultCommentMode isn't submitted by the create form - keep the
	// schema default ("open") instead of writing "" over it.
	if ct.DefaultCommentMode != "" {
		create = create.SetDefaultCommentMode(string(ct.DefaultCommentMode))
	}
	create = create.SetViewRoles(strings.Join(ct.ViewRoles, ",")).
		SetHideEmptyComments(ct.HideEmptyComments).
		SetDefaultPromote(ct.DefaultPromote).
		SetDefaultSticky(ct.DefaultSticky)
	// Nor is the publishing - the column default (moderation).
	if ct.DefaultPublishing.Valid() {
		create = create.SetDefaultPublishing(string(ct.DefaultPublishing))
	}
	// Display settings aren't submitted by the create form - a zero value
	// keeps the column defaults.
	if ct.TitleDisplay != "" {
		create = create.SetTitleDisplay(string(ct.TitleDisplay)).
			SetShowSubmitted(ct.ShowSubmitted).
			SetShowViewCount(ct.ShowViewCount)
	}
	// The history: on when the type asks (the starter's types), with the
	// column's keep unless set.
	if ct.Revisions {
		create = create.SetRevisions(true)
	}
	if ct.RevisionsKeep > 0 {
		create = create.SetRevisionsKeep(ct.RevisionsKeep)
	}
	if ct.Embedded {
		create = create.SetEmbedded(true)
	}
	v, err := create.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("ContentTypeRepository.CreateContentType: %w", err)
	}

	created := dao.ToContentTypeDefEntity(v)
	return &created, nil
}

// UpdateContentType saves ct's settings over the type ct.ID (its machine
// name stays) and returns it.
func (r ContentTypeRepository) UpdateContentType(ctx context.Context, ct entity.ContentTypeDef) (*entity.ContentTypeDef, error) {
	client := GetClient(ctx, r.client)
	v, err := client.ContentTypeDef.UpdateOneID(ct.ID).
		SetLabel(ct.Label).
		SetPathPattern(ct.PathPattern).
		SetDescription(ct.Description).
		SetStatus(ct.Status).
		SetDefaultCommentMode(string(ct.DefaultCommentMode)).
		SetHideEmptyComments(ct.HideEmptyComments).
		SetViewRoles(strings.Join(ct.ViewRoles, ",")).
		SetDefaultPromote(ct.DefaultPromote).
		SetDefaultSticky(ct.DefaultSticky).
		SetDefaultPublishing(string(ct.DefaultPublishing)).
		SetTitleDisplay(string(ct.TitleDisplay)).
		SetAltTitleField(ct.AltTitleField).
		SetRedirectField(ct.RedirectField).
		SetShowSubmitted(ct.ShowSubmitted).
		SetShowViewCount(ct.ShowViewCount).
		SetRevisions(ct.Revisions).
		SetRevisionsKeep(max(ct.RevisionsKeep, 0)).
		SetUpdatedAt(time.Now()).
		Save(ctx)
	if err != nil {
		return nil, notFoundAs(err, "content type with id %d", ct.ID)
	}

	updated := dao.ToContentTypeDefEntity(v)
	return &updated, nil
}

// DeleteContentType deletes the type and its field instances (groups among
// them, and their form and display settings) - field_instances has no
// cascade on content_type_id. Each field goes as DeleteField does, so a
// storage nothing else uses leaves the catalog. Run it in a transaction:
// outside one, a failure halfway leaves some fields gone and the type in
// place.
func (r ContentTypeRepository) DeleteContentType(ctx context.Context, id int) error {
	client := GetClient(ctx, r.client)
	// A group is an instance its members reference (parent_group_id,
	// form_group_id, no cascade): out of their groups first.
	if err := client.FieldInstanceDef.Update().
		Where(fieldinstancedef.ContentTypeID(id)).
		ClearParentGroupID().
		ClearFormGroupID().
		Exec(ctx); err != nil {
		return fmt.Errorf("ungrouping the content type's fields: %w", err)
	}
	fieldIDs, err := client.FieldInstanceDef.Query().Where(fieldinstancedef.ContentTypeID(id)).IDs(ctx)
	if err != nil {
		return fmt.Errorf("listing the content type's fields: %w", err)
	}
	for _, fieldID := range fieldIDs {
		if err := r.DeleteField(ctx, fieldID); err != nil {
			return fmt.Errorf("deleting the content type's field (id %v): %w", fieldID, err)
		}
	}
	return notFoundAs(client.ContentTypeDef.DeleteOneID(id).Exec(ctx), "content type with id %d", id)
}
