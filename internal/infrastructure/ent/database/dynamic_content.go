package database

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/database/dao"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/node"
)

// DynamicContentRepository - the nodes of one content type: the node row
// and its field values (entity.ContentTypeRepository's generic
// SetFieldValues/FieldValues, not a per-type ent entity). Built per
// request (NewDynamicContentRepositoryFactory): admin-created types are
// dynamic, and the struct is cheap.
type DynamicContentRepository struct {
	client   *generated.Client
	types    entity.ContentTypeRepository
	taxonomy entity.TaxonomyRepository
	def      entity.ContentTypeDef
	fields   entity.FieldInstanceDefs
}

// NewDynamicContentRepository - the repository of the nodes of the content
// type def with its fields: their values go through contentTypeRepo, the
// terms a node is loaded with come from taxonomyRepo.
func NewDynamicContentRepository(
	client *generated.Client,
	contentTypeRepo entity.ContentTypeRepository,
	taxonomyRepo entity.TaxonomyRepository,
	def entity.ContentTypeDef,
	fields entity.FieldInstanceDefs,
) *DynamicContentRepository {
	return &DynamicContentRepository{client: client, types: contentTypeRepo, taxonomy: taxonomyRepo, def: def, fields: fields}
}

// Create persists a new node row plus its field values, in one
// transaction (the caller's unit of work when ctx carries one): a failed
// value leaves no node.
func (r DynamicContentRepository) Create(ctx context.Context, e *entity.DynamicContent) (*entity.DynamicContent, error) {
	var created *entity.DynamicContent
	err := NewUnitOfWork(r.client).Do(ctx, func(ctx context.Context) error {
		client := GetClient(ctx, r.client)
		n := e.Node
		now := time.Now()
		row, err := client.Node.Create().
			SetType(r.def.MachineName).
			SetTitle(n.Title).
			SetNillableSlug(stringOrNil(n.Slug)).
			SetStatus(n.Status).
			SetOnModeration(n.OnModeration).
			SetCommentMode(string(n.CommentMode.OrDefault())).
			SetPathAuto(n.PathAuto).
			SetPromote(n.Promote).
			SetSticky(n.Sticky).
			SetViewRoles(strings.Join(n.ViewRoles, ",")).
			SetCreatedBy(n.CreatedBy.ID).
			SetCreatedAt(now).
			SetUpdatedAt(now).
			SetNillablePublishOn(timeOrNil(n.PublishOn)).
			SetNillableUnpublishOn(timeOrNil(n.UnpublishOn)).
			Save(ctx)
		if err != nil {
			return fmt.Errorf("creating the node: %w", err)
		}
		n.ID = row.ID
		values, err := r.saveBlockItems(ctx, client, n, e.Values)
		if err != nil {
			return err
		}
		if err := r.types.SetFieldValues(ctx, row.ID, r.fields, values, e.TextFormats); err != nil {
			return fmt.Errorf("creating the %s values (node %d): %w", r.def.MachineName, row.ID, err)
		}
		if err := indexTitle(ctx, client, row.ID); err != nil {
			return err
		}
		created, err = r.load(ctx, client, row.ID)
		return err
	})
	if err != nil {
		return nil, err
	}
	return created, nil
}

// Update saves the node's field values, then its own fields (title, slug,
// publishing, comment mode, flags, the schedule; the author and the date
// when set - an administrator's override, zero leaves them) - in one transaction (the
// caller's when ctx carries one): a failure halfway leaves the node as it
// was, its values not deleted.
func (r DynamicContentRepository) Update(ctx context.Context, e *entity.DynamicContent) (*entity.DynamicContent, error) {
	var updated *entity.DynamicContent
	err := NewUnitOfWork(r.client).Do(ctx, func(ctx context.Context) error {
		client := GetClient(ctx, r.client)
		n := e.Node
		values, err := r.saveBlockItems(ctx, client, n, e.Values)
		if err != nil {
			return err
		}
		if err := r.types.SetFieldValues(ctx, n.ID, r.fields, values, e.TextFormats); err != nil {
			return fmt.Errorf("updating the %s values (node %d): %w", r.def.MachineName, n.ID, err)
		}
		u := client.Node.UpdateOneID(n.ID).
			SetTitle(n.Title).
			SetNillableSlug(stringOrNil(n.Slug)).
			SetStatus(n.Status).
			SetOnModeration(n.OnModeration).
			SetCommentMode(string(n.CommentMode.OrDefault())).
			SetPathAuto(n.PathAuto).
			SetPromote(n.Promote).
			SetSticky(n.Sticky).
			SetViewRoles(strings.Join(n.ViewRoles, ",")).
			SetUpdatedAt(time.Now())
		if n.CreatedBy.ID > 0 {
			u = u.SetCreatedBy(n.CreatedBy.ID)
		}
		if !n.CreatedAt.IsZero() {
			u = u.SetCreatedAt(n.CreatedAt)
		}
		// The schedule: set, or cleared.
		if n.PublishOn.IsZero() {
			u = u.ClearPublishOn()
		} else {
			u = u.SetPublishOn(n.PublishOn)
		}
		if n.UnpublishOn.IsZero() {
			u = u.ClearUnpublishOn()
		} else {
			u = u.SetUnpublishOn(n.UnpublishOn)
		}
		if _, err := u.Save(ctx); err != nil {
			return fmt.Errorf("updating the node (id %d): %w", n.ID, err)
		}
		if err := indexTitle(ctx, client, n.ID); err != nil {
			return err
		}
		updated, err = r.load(ctx, client, n.ID)
		return err
	})
	if err != nil {
		return nil, err
	}
	return updated, nil
}

// ByNodeID - the node with its field values, terms and text formats.
func (r DynamicContentRepository) ByNodeID(ctx context.Context, nodeID int) (*entity.DynamicContent, error) {
	return r.load(ctx, GetClient(ctx, r.client), nodeID)
}

// load - the node nodeID read with client: its row (author, counter,
// SEO), field values, terms (the edit form's tag widget would render
// empty without them, and a save would wipe them) and the text formats
// of its values (the edit form's format dropdown).
func (r DynamicContentRepository) load(ctx context.Context, client *generated.Client, nodeID int) (*entity.DynamicContent, error) {
	n, err := client.Node.Query().
		Where(node.ID(nodeID)).
		WithUsers().
		WithNodeCounter().
		WithPageSeo().
		Only(ctx)
	if err != nil {
		return nil, fmt.Errorf("DynamicContentRepository.load: %w", err)
	}
	values, err := r.types.FieldValues(ctx, nodeID, r.fields)
	if err != nil {
		return nil, err
	}
	if err := r.loadBlockItems(ctx, client, values); err != nil {
		return nil, err
	}
	tags, err := r.taxonomy.NodeTermsByID(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	textFormats, err := r.types.TextFormatIDsByNode(ctx, nodeID, r.fields)
	if err != nil {
		return nil, err
	}
	return &entity.DynamicContent{
		Node:        dao.ToNodeEntity(n),
		ContentType: r.def,
		Fields:      r.fields,
		Values:      values,
		Tags:        tags,
		TextFormats: textFormats,
	}, nil
}

// NewDynamicContentRepositoryFactory returns a factory the interfaces layer
// can hold without importing ent: it captures the ent client and content
// type repository at wiring time and returns an
// entity.DynamicContentRepository per resolved content type.
func NewDynamicContentRepositoryFactory(
	client *generated.Client,
	contentTypeRepo entity.ContentTypeRepository,
	taxonomyRepo entity.TaxonomyRepository,
) func(entity.ContentTypeDef, entity.FieldInstanceDefs) entity.DynamicContentRepository {
	return func(def entity.ContentTypeDef, fields entity.FieldInstanceDefs) entity.DynamicContentRepository {
		return NewDynamicContentRepository(client, contentTypeRepo, taxonomyRepo, def, fields)
	}
}

// stringOrNil - s, nil for "" (a NULL column: a block item has no slug).
func stringOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// timeOrNil - t, nil for the zero time (a NULL column).
func timeOrNil(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}
