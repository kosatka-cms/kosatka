package database

import (
	"context"
	"fmt"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/database/dao"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/breadcrumbruledef"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/fieldinstancedef"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/fieldstoragedef"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/fieldvalueblock"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/node"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/seorule"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/viewdisplaydef"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/viewfielddef"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/viewfilterdef"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/viewsortdef"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/schema"
)

// FieldsByContentTypeID - the type's fields with their storages, by sort
// order, then ID.
func (r ContentTypeRepository) FieldsByContentTypeID(ctx context.Context, contentTypeID int) (entity.FieldInstanceDefs, error) {
	client := GetClient(ctx, r.client)
	res, err := client.FieldInstanceDef.Query().
		Where(fieldinstancedef.ContentTypeID(contentTypeID)).
		WithFieldStorage().
		Order(generated.Asc(fieldinstancedef.FieldSortOrder), generated.Asc(fieldinstancedef.FieldID)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("ContentTypeRepository.FieldsByContentTypeID: %w", err)
	}

	fields := make(entity.FieldInstanceDefs, len(res))
	for i, f := range res {
		fields[i] = dao.ToFieldInstanceDefEntity(f)
	}
	return fields, nil
}

// FieldByID - the field instance with id, with its storage;
// ErrEntityNotFound if there is none.
func (r ContentTypeRepository) FieldByID(ctx context.Context, id int) (*entity.FieldInstanceDef, error) {
	client := GetClient(ctx, r.client)
	res, err := client.FieldInstanceDef.Query().
		Where(fieldinstancedef.ID(id)).
		WithFieldStorage().
		Only(ctx)
	if err != nil {
		return nil, notFoundAs(err, "content type field with id %d", id)
	}

	f := dao.ToFieldInstanceDefEntity(res)
	return &f, nil
}

// CreateField creates a new FieldStorageDef (the machine name must be
// globally unique; the service layer handles duplicate-key errors) and a
// FieldInstanceDef pointing at it. Both inserts share a transaction when
// the caller wraps them in entity.UnitOfWork.Do (GetClient picks up the tx
// client from ctx).
func (r ContentTypeRepository) CreateField(ctx context.Context, f entity.FieldInstanceDef) (*entity.FieldInstanceDef, error) {
	client := GetClient(ctx, r.client)
	now := time.Now()

	storage, err := client.FieldStorageDef.Create().
		SetMachineName(f.MachineName).
		SetFieldType(string(f.FieldType)).
		SetConfig(f.Config).
		SetCreatedAt(now).
		SetUpdatedAt(now).
		Save(ctx)
	if err != nil {
		return nil, err
	}

	v, err := client.FieldInstanceDef.Create().
		SetContentTypeID(f.ContentTypeID).
		SetFieldStorageID(storage.ID).
		SetConfig(f.WidgetConfig).
		SetLabel(f.Label).
		SetRequired(f.Required).
		SetHiddenInForm(f.HiddenInForm).
		SetShowOnDetail(f.ShowOnDetail).
		SetShowLabel(f.ShowLabel).
		SetRendersInBody(f.RendersInBody).
		SetSortOrder(f.SortOrder).
		SetDisplayOrder(f.DisplayOrder).
		SetNillableParentGroupID(f.ParentGroupID).
		SetNillableFormGroupID(f.FormGroupID).
		SetUsableInForm(f.UsableInForm).
		SetUsableInDetail(f.UsableInDetail).
		SetCreatedAt(now).
		SetUpdatedAt(now).
		Save(ctx)
	if err != nil {
		return nil, err
	}

	v.Edges.FieldStorage = storage
	created := dao.ToFieldInstanceDefEntity(v)
	return &created, nil
}

// AttachExistingField creates only a FieldInstanceDef for an existing
// FieldStorageDef. The new instance gets the same visibility defaults as
// CreateField (shown on the detail page with its label).
func (r ContentTypeRepository) AttachExistingField(ctx context.Context, contentTypeID, fieldStorageID int, label string, required bool, sortOrder int) (*entity.FieldInstanceDef, error) {
	client := GetClient(ctx, r.client)
	now := time.Now()

	v, err := client.FieldInstanceDef.Create().
		SetContentTypeID(contentTypeID).
		SetFieldStorageID(fieldStorageID).
		SetLabel(label).
		SetRequired(required).
		SetShowOnDetail(true).
		SetShowLabel(true).
		SetRendersInBody(false).
		SetSortOrder(sortOrder).
		SetDisplayOrder(sortOrder).
		SetCreatedAt(now).
		SetUpdatedAt(now).
		Save(ctx)
	if err != nil {
		return nil, err
	}

	created, err := client.FieldInstanceDef.Query().
		Where(fieldinstancedef.ID(v.ID)).
		WithFieldStorage().
		Only(ctx)
	if err != nil {
		return nil, fmt.Errorf("ContentTypeRepository.AttachExistingField: %w", err)
	}

	result := dao.ToFieldInstanceDefEntity(created)
	return &result, nil
}

// FieldStorages returns the whole catalog of reusable fields, for the
// "add existing field" picker.
func (r ContentTypeRepository) FieldStorages(ctx context.Context) (entity.FieldStorageDefs, error) {
	client := GetClient(ctx, r.client)
	res, err := client.FieldStorageDef.Query().
		Order(generated.Asc(fieldstoragedef.FieldMachineName)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("ContentTypeRepository.FieldStorages: %w", err)
	}

	storages := make(entity.FieldStorageDefs, len(res))
	for i, v := range res {
		storages[i] = dao.ToFieldStorageDefEntity(v)
	}
	return storages, nil
}

// FieldStorageByID - see entity.ContentTypeRepository.FieldStorageByID.
func (r ContentTypeRepository) FieldStorageByID(ctx context.Context, id int) (*entity.FieldStorageDef, error) {
	client := GetClient(ctx, r.client)
	res, err := client.FieldStorageDef.Get(ctx, id)
	return one(res, err, dao.ToFieldStorageDefEntity, "field storage with id %d", id)
}

// FieldInstanceLabelsByFieldStorageIDs - see entity.ContentTypeRepository
// for why field_storages has no label. First instance (by id) wins per
// field_storage_id, which is deterministic and favors no content type.
func (r ContentTypeRepository) FieldInstanceLabelsByFieldStorageIDs(ctx context.Context, fieldStorageIDs []int) (map[int]string, error) {
	labels := make(map[int]string, len(fieldStorageIDs))
	if len(fieldStorageIDs) == 0 {
		return labels, nil
	}
	client := GetClient(ctx, r.client)
	res, err := client.FieldInstanceDef.Query().
		Where(fieldinstancedef.FieldStorageIDIn(fieldStorageIDs...)).
		Order(generated.Asc(fieldinstancedef.FieldID)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("ContentTypeRepository.FieldInstanceLabelsByFieldStorageIDs: %w", err)
	}
	for _, f := range res {
		if _, ok := labels[f.FieldStorageID]; !ok {
			labels[f.FieldStorageID] = f.Label
		}
	}
	return labels, nil
}

// RichTextFieldStorageIDs - see entity.ContentTypeRepository.
func (r ContentTypeRepository) RichTextFieldStorageIDs(ctx context.Context, fieldStorageIDs []int) (map[int]bool, error) {
	rich := make(map[int]bool, len(fieldStorageIDs))
	if len(fieldStorageIDs) == 0 {
		return rich, nil
	}
	client := GetClient(ctx, r.client)
	res, err := client.FieldInstanceDef.Query().
		Where(fieldinstancedef.FieldStorageIDIn(fieldStorageIDs...)).
		WithFieldStorage().
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("ContentTypeRepository.RichTextFieldStorageIDs: %w", err)
	}
	for _, f := range res {
		isRich := dao.ToFieldInstanceDefEntity(f).RichText()
		if prev, seen := rich[f.FieldStorageID]; seen {
			isRich = prev && isRich
		}
		rich[f.FieldStorageID] = isRich
	}
	return rich, nil
}

// UpdateField updates FieldStorageDef.Config (shared by every content type
// using the field) and the instance's Label/Required/HiddenInForm/
// DefaultImageID/UsableInForm/UsableInDetail/Config (the per-type
// WidgetConfig). It never touches SortOrder/FormGroupID
// (UpdateFieldSortOrder) or ShowOnDetail/ShowLabel/DisplayOrder/
// DetailImageStyle/ParentGroupID (UpdateFieldDetailDisplay): disjoint write
// paths keep one admin tab from clobbering another's settings.
func (r ContentTypeRepository) UpdateField(ctx context.Context, f entity.FieldInstanceDef) (*entity.FieldInstanceDef, error) {
	client := GetClient(ctx, r.client)
	now := time.Now()

	storage, err := client.FieldStorageDef.UpdateOneID(f.FieldStorageID).
		SetConfig(f.Config).
		SetUpdatedAt(now).
		Save(ctx)
	if err != nil {
		return nil, notFoundAs(err, "field storage with id %d", f.FieldStorageID)
	}

	// ParentGroupID/FormGroupID are written by UpdateFieldDetailDisplay and
	// UpdateFieldSortOrder respectively.
	update := client.FieldInstanceDef.UpdateOneID(f.ID).
		SetConfig(f.WidgetConfig).
		SetLabel(f.Label).
		SetRequired(f.Required).
		SetHiddenInForm(f.HiddenInForm).
		SetUsableInForm(f.UsableInForm).
		SetUsableInDetail(f.UsableInDetail).
		SetDateFormat(f.DateFormat).
		SetUpdatedAt(now)
	if f.DefaultImageID != nil {
		update = update.SetDefaultImageID(*f.DefaultImageID)
	} else {
		update = update.ClearDefaultImageID()
	}
	v, err := update.Save(ctx)
	if err != nil {
		return nil, notFoundAs(err, "content type field with id %d", f.ID)
	}

	v.Edges.FieldStorage = storage
	updated := dao.ToFieldInstanceDefEntity(v)
	return &updated, nil
}

// UpdateFieldSortOrder updates the form field order and FormGroupID from
// the fields tab drag-and-drop table (see UpdateField for why it is
// separate). FormGroupID is also set per field via UpdateField; this path
// updates it in bulk when an admin drags a field into a form group.
func (r ContentTypeRepository) UpdateFieldSortOrder(ctx context.Context, fieldID int, sortOrder int, formGroupID *int) error {
	client := GetClient(ctx, r.client)
	update := client.FieldInstanceDef.UpdateOneID(fieldID).
		SetSortOrder(sortOrder).
		SetUpdatedAt(time.Now())
	if formGroupID != nil {
		update = update.SetFormGroupID(*formGroupID)
	} else {
		update = update.ClearFormGroupID()
	}
	return notFoundAs(update.Exec(ctx), "content type field with id %d", fieldID)
}

// UpdateFieldDetailDisplay updates the detail-page display settings,
// including ParentGroupID (see UpdateField for why it is separate).
func (r ContentTypeRepository) UpdateFieldDetailDisplay(ctx context.Context, fieldID int, d entity.FieldDetailDisplay) error {
	client := GetClient(ctx, r.client)
	update := client.FieldInstanceDef.UpdateOneID(fieldID).
		SetDisplayOrder(d.DisplayOrder).
		SetShowOnDetail(d.ShowOnDetail).
		SetShowLabel(d.ShowLabel).
		SetRendersInBody(d.RendersInBody).
		SetDetailImageStyle(d.ImageStyle).
		SetListImageStyle(d.ListImageStyle).
		SetDetailFormatter(d.Formatter).
		SetDetailLayout(string(d.Layout)).
		SetUpdatedAt(time.Now())
	// SetNillableParentGroupID would leave the column untouched on nil, but
	// here nil means "not in a group" and must clear the existing value.
	if parentGroupID := d.ParentGroupID; parentGroupID != nil {
		update = update.SetParentGroupID(*parentGroupID)
	} else {
		update = update.ClearParentGroupID()
	}
	err := update.Exec(ctx)
	if err != nil {
		return notFoundAs(err, "content type field with id %d", fieldID)
	}
	return nil
}

// DeleteField removes one content type's instance of a field. The shared
// FieldStorageDef stays in the reusable catalog while another instance or
// anything else holds it (FieldStorageUsage); with the last instance of an
// unused storage it goes too. field_value_*.field_id is the shared storage ID, so value rows
// are deleted only for nodes of this field's content type - otherwise the
// values of every other content type using the field would be wiped too.
func (r ContentTypeRepository) DeleteField(ctx context.Context, id int) error {
	client := GetClient(ctx, r.client)

	instance, err := client.FieldInstanceDef.Query().
		Where(fieldinstancedef.ID(id)).
		WithFieldStorage().
		WithContentType().
		Only(ctx)
	if err != nil {
		return notFoundAs(err, "content type field with id %d", id)
	}

	storageID := instance.Edges.FieldStorage.ID
	fieldType := entity.ContentTypeFieldType(instance.Edges.FieldStorage.FieldType)
	multiple, _ := instance.Edges.FieldStorage.Config["multiple"].(bool)
	machineName := instance.Edges.ContentType.MachineName

	nodeIDs, err := client.Node.Query().Where(node.Type(machineName)).IDs(everyNode(ctx))
	if err != nil {
		return fmt.Errorf("ContentTypeRepository.DeleteField: %w", err)
	}

	// A blocks field's blocks go with it (their rows in the field too).
	if fieldType == entity.FieldTypeBlocks && len(nodeIDs) > 0 {
		items, err := client.FieldValueBlock.Query().
			Where(fieldvalueblock.FieldIDEQ(storageID), fieldvalueblock.NodeIDIn(nodeIDs...)).
			Select(fieldvalueblock.FieldItemID).Ints(ctx)
		if err != nil {
			return fmt.Errorf("loading the blocks of field %d: %w", storageID, err)
		}
		if len(items) > 0 {
			if _, err := client.Node.Delete().Where(node.IDIn(items...)).Exec(ctx); err != nil {
				return fmt.Errorf("deleting the blocks of field %d: %w", storageID, err)
			}
		}
	}
	if t, ok := valueTableOf(fieldType, multiple); ok && len(nodeIDs) > 0 {
		if _, err := t.delete(ctx, client, valueOfField(storageID), valueOfNodes(nodeIDs...)); err != nil {
			return err
		}
	}

	if err := client.FieldInstanceDef.DeleteOneID(id).Exec(ctx); err != nil {
		if generated.IsNotFound(err) {
			return fmt.Errorf("%w: content type field with id %d", entity.ErrEntityNotFound, id)
		}
		return fmt.Errorf("error deleting content type field: %w", err)
	}

	// The last instance of an unreferenced storage: the field leaves the
	// catalog (values of other nodes, if any, go with it - ON DELETE
	// CASCADE).
	usage, err := r.FieldStorageUsage(ctx, storageID)
	if err != nil {
		return err
	}
	if usage.Removable() {
		if err := client.FieldStorageDef.DeleteOneID(storageID).Exec(ctx); err != nil {
			return fmt.Errorf("error deleting field storage: %w", err)
		}
	}
	return nil
}

// FieldStorageUsage counts what still holds the storage: its instances,
// Views (fields, filters, sorts, carousel link and glossary displays),
// breadcrumb rules and SEO rules.
func (r ContentTypeRepository) FieldStorageUsage(ctx context.Context, storageID int) (entity.FieldStorageUsage, error) {
	client := GetClient(ctx, r.client)
	var u entity.FieldStorageUsage
	var err error
	if u.Instances, err = client.FieldInstanceDef.Query().Where(fieldinstancedef.FieldStorageID(storageID)).Count(ctx); err != nil {
		return u, err
	}
	counts := []func() (int, error){
		func() (int, error) {
			return client.ViewFieldDef.Query().Where(viewfielddef.FieldStorageID(storageID)).Count(ctx)
		},
		func() (int, error) {
			return client.ViewFilterDef.Query().Where(viewfilterdef.Or(
				viewfilterdef.FieldStorageID(storageID),
				viewfilterdef.ViaFieldStorageID(storageID),
			)).Count(ctx)
		},
		func() (int, error) {
			return client.ViewSortDef.Query().Where(viewsortdef.FieldStorageID(storageID)).Count(ctx)
		},
		func() (int, error) {
			return client.ViewDisplayDef.Query().Where(viewdisplaydef.Or(
				viewdisplaydef.CarouselLinkFieldStorageID(storageID),
				viewdisplaydef.GlossaryFieldStorageID(storageID),
			)).Count(ctx)
		},
	}
	for _, count := range counts {
		n, err := count()
		if err != nil {
			return u, err
		}
		u.Views += n
	}
	if u.BreadcrumbRules, err = client.BreadcrumbRuleDef.Query().Where(breadcrumbruledef.FieldStorageID(storageID)).Count(ctx); err != nil {
		return u, err
	}
	u.SEORules, err = client.SeoRule.Query().Where(seorule.FieldStorageID(storageID)).Count(ctx)
	return u, err
}

// CountFieldValues counts stored values of the field on nodes of this
// content type only (for the confirmation shown before detaching it).
// fieldID is the shared FieldStorageDef.ID, so contentTypeID is required to
// exclude values of other content types using the same field.
func (r ContentTypeRepository) CountFieldValues(ctx context.Context, contentTypeID, fieldID int, fieldType entity.ContentTypeFieldType, multiple bool) (int, error) {
	client := GetClient(ctx, r.client)

	ct, err := client.ContentTypeDef.Get(ctx, contentTypeID)
	if err != nil {
		return 0, notFoundAs(err, "content type with id %d", contentTypeID)
	}
	nodeIDs, err := client.Node.Query().Where(node.Type(ct.MachineName)).IDs(schema.WithBlocks(ctx))
	if err != nil {
		return 0, fmt.Errorf("ContentTypeRepository.CountFieldValues: %w", err)
	}
	t, ok := valueTableOf(fieldType, multiple)
	if !ok || len(nodeIDs) == 0 {
		return 0, nil
	}
	return t.count(ctx, client, valueOfField(fieldID), valueOfNodes(nodeIDs...))
}
