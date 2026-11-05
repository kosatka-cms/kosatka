package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"slices"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/lib/apperrors"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// Guard errors returned by the content-type admin module.
// Callers should use errors.Is to detect them.
var (
	ErrContentTypeMachineNameReserved = errors.New("machine name collides with a built-in content type")
	ErrContentTypeHasNodes            = errors.New("content type still has nodes")
	ErrContentTypeDuplicate           = errors.New("content type machine_name already taken")
	// ErrContentTypeMachineName - CreateContentType got a machine name
	// that isn't contentTypeMachineNameRe.
	ErrContentTypeMachineName = errors.New("content type machine_name: lowercase latin letters, digits and _, starting with a letter")
	// ErrFieldStorageMachineNameTaken - CreateField got a machine_name
	// that already exists as a FieldStorageDef. Machine names are global
	// (shared by every content type that attaches the field), so the admin
	// should use AttachExistingField instead.
	ErrFieldStorageMachineNameTaken = errors.New("a field with this machine_name already exists - attach the existing field instead")
	// ErrFieldAlreadyAttached - AttachExistingField tried to attach a
	// FieldStorageDef to a content type that already has an instance of
	// it (UNIQUE(content_type_id, field_storage_id)).
	ErrFieldAlreadyAttached = errors.New("this field is already attached to this content type")
	// ErrFieldHasSeveralValues - SetFieldMultiple can't make a field
	// single-valued while a node holds several of its values.
	ErrFieldHasSeveralValues = errors.New("some content has several values in this field")
	// ErrFieldCardinalityFixed - SetFieldMultiple got a field type that
	// holds one value only (entity.ContentTypeFieldType.AllowsMultiple).
	ErrFieldCardinalityFixed = errors.New("this field type holds one value only")
	// ErrBlocksInFieldSet - a blocks field can't be put in a field set:
	// blocks aren't nested.
	ErrBlocksInFieldSet = errors.New("a field set can't have a blocks field")
)

// contentTypeMachineNameRe - a content type's machine name: it becomes
// node.Type and a part of addresses and permission names.
var contentTypeMachineNameRe = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)

// ContentTypeService - the content types and the field sets, their fields
// (the catalog of reusable fields, each type's instances of them) and the
// nodes' field values.
type ContentTypeService interface {
	// ContentTypes - the content types, not the field sets (FieldSets).
	ContentTypes(ctx context.Context) (entity.ContentTypeDefs, error)
	// FieldSets - the field sets (ContentTypeDef.Embedded): the types of
	// the blocks fields' items.
	FieldSets(ctx context.Context) (entity.ContentTypeDefs, error)
	ContentTypeByID(ctx context.Context, id int) (*entity.ContentTypeDef, error)
	ContentTypeByMachineName(ctx context.Context, machineName string) (*entity.ContentTypeDef, error)
	// NodeTypeLabels - every node type's label by machine name: the
	// content types' own and the modules' node types' (translated). When
	// the content types fail to load (logged), the modules' alone.
	NodeTypeLabels(ctx context.Context) map[string]string
	CreateContentType(ctx context.Context, ct entity.ContentTypeDef) (*entity.ContentTypeDef, error)
	UpdateContentType(ctx context.Context, ct entity.ContentTypeDef) (*entity.ContentTypeDef, error)
	DeleteContentType(ctx context.Context, id int) error

	FieldsByContentTypeID(ctx context.Context, contentTypeID int) (entity.FieldInstanceDefs, error)
	FieldByID(ctx context.Context, id int) (*entity.FieldInstanceDef, error)
	CreateField(ctx context.Context, f entity.FieldInstanceDef) (*entity.FieldInstanceDef, error)
	UpdateField(ctx context.Context, f entity.FieldInstanceDef) (*entity.FieldInstanceDef, error)
	// FieldStorages - the catalog of reusable fields, for the "add an
	// existing field" picker.
	FieldStorages(ctx context.Context) (entity.FieldStorageDefs, error)
	// FieldStorageByID - see entity.ContentTypeRepository.FieldStorageByID;
	// used by Views, which resolve fields by FieldStorageDef.ID rather
	// than through a per-content-type FieldInstanceDef.
	FieldStorageByID(ctx context.Context, id int) (*entity.FieldStorageDef, error)
	// FieldInstanceLabelsByFieldStorageIDs - see
	// entity.ContentTypeRepository.FieldInstanceLabelsByFieldStorageIDs.
	FieldInstanceLabelsByFieldStorageIDs(ctx context.Context, fieldStorageIDs []int) (map[int]string, error)
	// RichTextFieldStorageIDs - see
	// entity.ContentTypeRepository.RichTextFieldStorageIDs.
	RichTextFieldStorageIDs(ctx context.Context, fieldStorageIDs []int) (map[int]bool, error)
	// AttachExistingField - "add an existing field"; see
	// entity.ContentTypeRepository.AttachExistingField.
	AttachExistingField(ctx context.Context, contentTypeID, fieldStorageID int, label string, required bool, sortOrder int) (*entity.FieldInstanceDef, error)
	// UpdateFieldSortOrder/UpdateFieldDetailDisplay - the "Fields" and
	// "Detail display" tabs: ordering (plus that tab's visibility flags for
	// the latter), nothing structural.
	UpdateFieldSortOrder(ctx context.Context, fieldID int, sortOrder int, formGroupID *int) error
	UpdateFieldDetailDisplay(ctx context.Context, fieldID int, d entity.FieldDetailDisplay) error
	DeleteField(ctx context.Context, id int) error
	// FieldValueCount - how many values are stored for the field, for the
	// confirmation shown before deleting it.
	FieldValueCount(ctx context.Context, fieldID int) (int, error)
	// FieldStorageUsage - what holds the field's catalog entry, for the
	// same confirmation.
	FieldStorageUsage(ctx context.Context, storageID int) (entity.FieldStorageUsage, error)
	// NodesWithSeveralFieldValues - the nodes holding several values of a
	// multi-value field: they block making it single-valued.
	NodesWithSeveralFieldValues(ctx context.Context, storageID int) ([]int, error)
	// SetFieldMultiple switches a field between one and several values per
	// node, for every content type using it (see
	// entity.ContentTypeRepository.SetFieldMultiple).
	SetFieldMultiple(ctx context.Context, storageID int, multiple bool) error

	FieldValues(ctx context.Context, nodeID int, fields entity.FieldInstanceDefs) (map[int]any, error)
	FieldValuesForNodes(ctx context.Context, nodeIDs []int, fields entity.FieldInstanceDefs) (map[int]map[int]any, error)
	SetFieldValues(ctx context.Context, nodeID int, fields entity.FieldInstanceDefs, values map[int]any, textFormats map[int]int) error
	TextFormatIDsByNode(ctx context.Context, nodeID int, fields entity.FieldInstanceDefs) (map[int]int, error)
	FieldTextValuesForNodes(ctx context.Context, fieldStorageID int, nodeIDs []int) (map[int]string, error)
	FieldNumberValuesForNodes(ctx context.Context, fieldStorageID int, nodeIDs []int) (map[int]float64, error)
	FieldDateValuesForNodes(ctx context.Context, fieldStorageID int, nodeIDs []int) (map[int]time.Time, error)
	FieldTaxonomyValuesForNodes(ctx context.Context, fieldStorageID int, nodeIDs []int) (map[int]int, error)
	FieldTaxonomyMultiValuesForNodes(ctx context.Context, fieldStorageID int, nodeIDs []int) (map[int][]int, error)
}

// DefaultContentTypeService - the ContentTypeService on the content types'
// repository.
type DefaultContentTypeService struct {
	repository entity.ContentTypeRepository
	nodeRepo   entity.NodesRepository
	uow        entity.UnitOfWork
	logger     *slog.Logger
	// changed - called after a type is created or changed (SetChanged).
	changed func()
}

// SetChanged sets what runs after a content type is created or changed
// (wiring: the view access' cache).
func (s *DefaultContentTypeService) SetChanged(f func()) { s.changed = f }

func (s *DefaultContentTypeService) notifyChanged() {
	if s.changed != nil {
		s.changed()
	}
}

// NewContentTypeService - the content type service on r (the types, their
// fields and values), nodeRepo (a type's nodes, counted before it is
// deleted), uow (the transactions) and baseLogger.
func NewContentTypeService(
	r entity.ContentTypeRepository,
	nodeRepo entity.NodesRepository,
	uow entity.UnitOfWork,
	baseLogger *slog.Logger,
) ContentTypeService {
	return &DefaultContentTypeService{
		repository: r,
		nodeRepo:   nodeRepo,
		uow:        uow,
		logger:     baseLogger.With("component", "content_type.service"),
	}
}

// ContentTypes - see ContentTypeService.ContentTypes.
func (s *DefaultContentTypeService) ContentTypes(ctx context.Context) (entity.ContentTypeDefs, error) {
	cts, err := s.repository.ContentTypes(ctx)
	return entity.Wrapped(cts, err, "listing content types")
}

// FieldSets - see ContentTypeService.FieldSets.
func (s *DefaultContentTypeService) FieldSets(ctx context.Context) (entity.ContentTypeDefs, error) {
	sets, err := s.repository.FieldSets(ctx)
	return entity.Wrapped(sets, err, "listing field sets")
}

// NodeTypeLabels - see ContentTypeService.NodeTypeLabels.
func (s *DefaultContentTypeService) NodeTypeLabels(ctx context.Context) map[string]string {
	modules := entity.ModuleNodeTypeLabels()
	labels := make(map[string]string, len(modules))
	for t, l := range modules {
		labels[t] = i18n.T(ctx, l)
	}
	defs, err := s.ContentTypes(ctx)
	if err != nil {
		LogFallback(s.logger, err, "node type labels: the content types")
		return labels
	}
	for _, d := range defs {
		labels[d.MachineName] = d.Label
	}
	return labels
}

// ContentTypeByID - the content type (or field set) id.
func (s *DefaultContentTypeService) ContentTypeByID(ctx context.Context, id int) (*entity.ContentTypeDef, error) {
	ct, err := s.repository.ContentTypeByID(ctx, id)
	return entity.Found(ct, err, "finding content type by id (id %v)", id)
}

// ContentTypeByMachineName - the content type named machineName (the
// nodes' Type).
func (s *DefaultContentTypeService) ContentTypeByMachineName(ctx context.Context, machineName string) (*entity.ContentTypeDef, error) {
	ct, err := s.repository.ContentTypeByMachineName(ctx, machineName)
	return entity.Found(ct, err, "finding content type by machine name (machineName %v)", machineName)
}

// CreateContentType saves the new content type ct: its machine name must
// be valid (ErrContentTypeMachineName), not a built-in type's
// (ErrContentTypeMachineNameReserved) and free (ErrContentTypeDuplicate).
func (s *DefaultContentTypeService) CreateContentType(ctx context.Context, ct entity.ContentTypeDef) (*entity.ContentTypeDef, error) {
	if !contentTypeMachineNameRe.MatchString(ct.MachineName) {
		return nil, fmt.Errorf("%w: %q", ErrContentTypeMachineName, ct.MachineName)
	}
	if slices.Contains(entity.ReservedContentTypeMachineNames(), ct.MachineName) {
		return nil, fmt.Errorf("%w: %q", ErrContentTypeMachineNameReserved, ct.MachineName)
	}

	created, err := s.repository.CreateContentType(ctx, ct)
	if err != nil {
		if isDuplicateKeyErr(err) {
			return nil, fmt.Errorf("%w", ErrContentTypeDuplicate)
		}
		return nil, fmt.Errorf("creating content type (machineName %v): %w", ct.MachineName, err)
	}
	s.notifyChanged()
	return created, nil
}

// UpdateContentType saves content type ct; a machine name another type
// has - ErrContentTypeDuplicate.
func (s *DefaultContentTypeService) UpdateContentType(ctx context.Context, ct entity.ContentTypeDef) (*entity.ContentTypeDef, error) {
	updated, err := s.repository.UpdateContentType(ctx, ct)
	if err != nil {
		if isDuplicateKeyErr(err) {
			return nil, fmt.Errorf("%w", ErrContentTypeDuplicate)
		}
		return nil, fmt.Errorf("updating content type (id %v): %w", ct.ID, err)
	}
	s.notifyChanged()
	return updated, nil
}

// DeleteContentType deletes a type that has no nodes (ErrContentTypeHasNodes)
// together with its fields, in one transaction.
func (s *DefaultContentTypeService) DeleteContentType(ctx context.Context, id int) error {
	return s.uow.Do(ctx, func(ctx context.Context) error {
		ct, err := s.repository.ContentTypeByID(ctx, id)
		if err != nil {
			return fmt.Errorf("loading content type before delete (id %v): %w", id, err)
		}

		count, err := s.nodeRepo.CountByType(ctx, ct.MachineName)
		if err != nil {
			return fmt.Errorf("checking content type usage before delete (id %v): %w", id, err)
		}
		if count > 0 {
			return fmt.Errorf("%w: %d nodes of type %q", ErrContentTypeHasNodes, count, ct.MachineName)
		}

		if err := s.repository.DeleteContentType(ctx, id); err != nil {
			return fmt.Errorf("deleting content type (id %v): %w", id, err)
		}
		return nil
	})
}

// FieldsByContentTypeID - the fields of the content type, by their sort
// order.
func (s *DefaultContentTypeService) FieldsByContentTypeID(ctx context.Context, contentTypeID int) (entity.FieldInstanceDefs, error) {
	fields, err := s.repository.FieldsByContentTypeID(ctx, contentTypeID)
	return entity.Wrapped(fields, err, "listing content type fields (contentTypeID %v)", contentTypeID)
}

// FieldByID - the field instance id (a field of one content type).
func (s *DefaultContentTypeService) FieldByID(ctx context.Context, id int) (*entity.FieldInstanceDef, error) {
	f, err := s.repository.FieldByID(ctx, id)
	return entity.Found(f, err, "finding content type field by id (id %v)", id)
}

// CreateField atomically creates a new FieldStorageDef + FieldInstanceDef.
// A machine_name collision is ErrFieldStorageMachineNameTaken (not
// ErrContentTypeDuplicate), so the admin can attach the existing field
// instead.
func (s *DefaultContentTypeService) CreateField(ctx context.Context, f entity.FieldInstanceDef) (*entity.FieldInstanceDef, error) {
	if err := s.checkBlocksPlace(ctx, f.ContentTypeID, f.FieldType); err != nil {
		return nil, err
	}
	var created *entity.FieldInstanceDef
	err := s.uow.Do(ctx, func(ctx context.Context) error {
		var err error
		created, err = s.repository.CreateField(ctx, f)
		return err
	})
	if err != nil {
		if isDuplicateKeyErr(err) {
			return nil, fmt.Errorf("%w", ErrFieldStorageMachineNameTaken)
		}
		return nil, fmt.Errorf("creating content type field (contentTypeID %v, machineName %v): %w", f.ContentTypeID, f.MachineName, err)
	}
	return created, nil
}

// AttachExistingField attaches an existing field storage to a content type
// (single insert). Attaching it to the same type twice is
// ErrFieldAlreadyAttached.
func (s *DefaultContentTypeService) AttachExistingField(ctx context.Context, contentTypeID, fieldStorageID int, label string, required bool, sortOrder int) (*entity.FieldInstanceDef, error) {
	storage, err := s.repository.FieldStorageByID(ctx, fieldStorageID)
	if err != nil {
		return nil, fmt.Errorf("loading the field to attach (id %v): %w", fieldStorageID, err)
	}
	if err := s.checkBlocksPlace(ctx, contentTypeID, storage.FieldType); err != nil {
		return nil, err
	}
	created, err := s.repository.AttachExistingField(ctx, contentTypeID, fieldStorageID, label, required, sortOrder)
	if err != nil {
		if isDuplicateKeyErr(err) {
			return nil, fmt.Errorf("%w", ErrFieldAlreadyAttached)
		}
		return nil, fmt.Errorf("attaching existing field (contentTypeID %v, fieldStorageID %v): %w", contentTypeID, fieldStorageID, err)
	}
	return created, nil
}

// checkBlocksPlace - ErrBlocksInFieldSet for a blocks field in a field
// set (the content type contentTypeID is Embedded).
func (s *DefaultContentTypeService) checkBlocksPlace(ctx context.Context, contentTypeID int, ft entity.ContentTypeFieldType) error {
	if ft != entity.FieldTypeBlocks {
		return nil
	}
	ct, err := s.repository.ContentTypeByID(ctx, contentTypeID)
	if err != nil {
		return fmt.Errorf("loading the content type (id %v): %w", contentTypeID, err)
	}
	if ct.Embedded {
		return ErrBlocksInFieldSet
	}
	return nil
}

// FieldStorages - see ContentTypeService.FieldStorages.
func (s *DefaultContentTypeService) FieldStorages(ctx context.Context) (entity.FieldStorageDefs, error) {
	storages, err := s.repository.FieldStorages(ctx)
	return entity.Wrapped(storages, err, "listing field storages")
}

// FieldStorageByID - see ContentTypeService.FieldStorageByID.
func (s *DefaultContentTypeService) FieldStorageByID(ctx context.Context, id int) (*entity.FieldStorageDef, error) {
	fs, err := s.repository.FieldStorageByID(ctx, id)
	return entity.Found(fs, err, "finding field storage by id (id %v)", id)
}

// FieldInstanceLabelsByFieldStorageIDs - see
// ContentTypeService.FieldInstanceLabelsByFieldStorageIDs.
func (s *DefaultContentTypeService) FieldInstanceLabelsByFieldStorageIDs(ctx context.Context, fieldStorageIDs []int) (map[int]string, error) {
	labels, err := s.repository.FieldInstanceLabelsByFieldStorageIDs(ctx, fieldStorageIDs)
	return entity.Wrapped(labels, err, "loading field instance labels")
}

// RichTextFieldStorageIDs - see ContentTypeService.RichTextFieldStorageIDs.
func (s *DefaultContentTypeService) RichTextFieldStorageIDs(ctx context.Context, fieldStorageIDs []int) (map[int]bool, error) {
	rich, err := s.repository.RichTextFieldStorageIDs(ctx, fieldStorageIDs)
	return entity.Wrapped(rich, err, "loading rich text field storages")
}

// UpdateField updates FieldStorageDef.Config (shared) and
// FieldInstanceDef.Label/Required (per type) in one transaction.
func (s *DefaultContentTypeService) UpdateField(ctx context.Context, f entity.FieldInstanceDef) (*entity.FieldInstanceDef, error) {
	var updated *entity.FieldInstanceDef
	err := s.uow.Do(ctx, func(ctx context.Context) error {
		var err error
		updated, err = s.repository.UpdateField(ctx, f)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("updating content type field (id %v): %w", f.ID, err)
	}
	return updated, nil
}

// UpdateFieldSortOrder - see ContentTypeService.UpdateFieldSortOrder.
func (s *DefaultContentTypeService) UpdateFieldSortOrder(ctx context.Context, fieldID int, sortOrder int, formGroupID *int) error {
	if err := s.repository.UpdateFieldSortOrder(ctx, fieldID, sortOrder, formGroupID); err != nil {
		return fmt.Errorf("updating content type field sort order (fieldID %v): %w", fieldID, err)
	}
	return nil
}

// UpdateFieldDetailDisplay saves the field's "Detail display" settings d,
// nothing else of it.
func (s *DefaultContentTypeService) UpdateFieldDetailDisplay(ctx context.Context, fieldID int, d entity.FieldDetailDisplay) error {
	if err := s.repository.UpdateFieldDetailDisplay(ctx, fieldID, d); err != nil {
		return fmt.Errorf("updating content type field detail display settings (fieldID %v): %w", fieldID, err)
	}
	return nil
}

// DeleteField deletes the field instance and its values on that content
// type's nodes in one transaction.
func (s *DefaultContentTypeService) DeleteField(ctx context.Context, id int) error {
	err := s.uow.Do(ctx, func(ctx context.Context) error {
		return s.repository.DeleteField(ctx, id)
	})
	if err != nil {
		return fmt.Errorf("deleting content type field (id %v): %w", id, err)
	}
	return nil
}

// NodesWithSeveralFieldValues - see ContentTypeService.
func (s *DefaultContentTypeService) NodesWithSeveralFieldValues(ctx context.Context, storageID int) ([]int, error) {
	storage, err := s.repository.FieldStorageByID(ctx, storageID)
	if err != nil {
		return nil, fmt.Errorf("loading field storage (id %v): %w", storageID, err)
	}
	if !storage.FieldType.AllowsMultiple() {
		return nil, nil
	}
	ids, err := s.repository.NodesWithSeveralFieldValues(ctx, *storage)
	return entity.Wrapped(ids, err, "finding nodes with several values (field storage %v)", storageID)
}

// SetFieldMultiple - a field storage's cardinality: a field of a
// type that allows it (entity.ContentTypeFieldType.AllowsMultiple) can go
// from one value per node to several and back. Its values move to the
// other value table in one transaction; making it single-valued is refused
// with ErrFieldHasSeveralValues while a node has several values (the
// admin edits them first). It applies to every content type using the
// field.
func (s *DefaultContentTypeService) SetFieldMultiple(ctx context.Context, storageID int, multiple bool) error {
	err := s.uow.Do(ctx, func(ctx context.Context) error {
		storage, err := s.repository.FieldStorageByID(ctx, storageID)
		if err != nil {
			return err
		}
		if !storage.FieldType.AllowsMultiple() {
			return ErrFieldCardinalityFixed
		}
		if !multiple {
			several, err := s.repository.NodesWithSeveralFieldValues(ctx, *storage)
			if err != nil {
				return err
			}
			if len(several) > 0 {
				return fmt.Errorf("%w: %d nodes", ErrFieldHasSeveralValues, len(several))
			}
		}
		return s.repository.SetFieldMultiple(ctx, storageID, multiple)
	})
	if err != nil {
		return fmt.Errorf("setting field cardinality (field storage %v, multiple %v): %w", storageID, multiple, err)
	}
	return nil
}

// FieldStorageUsage - what holds a catalog field (entity.FieldStorageUsage).
func (s *DefaultContentTypeService) FieldStorageUsage(ctx context.Context, storageID int) (entity.FieldStorageUsage, error) {
	return s.repository.FieldStorageUsage(ctx, storageID)
}

// FieldValueCount - the number of the field's values stored on nodes of
// the instance's content type. Scoping by type is required because the
// storage is shared by every type that attaches the field.
func (s *DefaultContentTypeService) FieldValueCount(ctx context.Context, fieldID int) (int, error) {
	f, err := s.repository.FieldByID(ctx, fieldID)
	if err != nil {
		if apperrors.IsNotFound(err) {
			return 0, err
		}
		return 0, fmt.Errorf("loading field before counting values (fieldID %v): %w", fieldID, err)
	}

	count, err := s.repository.CountFieldValues(ctx, f.ContentTypeID, f.FieldStorageID, f.FieldType, f.Multiple())
	if err != nil {
		return 0, fmt.Errorf("counting field values (fieldID %v): %w", fieldID, err)
	}
	return count, nil
}

// FieldValues - the node's values of fields, by FieldInstanceDef.ID.
func (s *DefaultContentTypeService) FieldValues(ctx context.Context, nodeID int, fields entity.FieldInstanceDefs) (map[int]any, error) {
	values, err := s.repository.FieldValues(ctx, nodeID, fields)
	return entity.Wrapped(values, err, "loading field values (nodeID %v)", nodeID)
}

// FieldValuesForNodes - FieldValues of each of the nodes nodeIDs, by node
// ID (one query a field).
func (s *DefaultContentTypeService) FieldValuesForNodes(ctx context.Context, nodeIDs []int, fields entity.FieldInstanceDefs) (map[int]map[int]any, error) {
	values, err := s.repository.FieldValuesForNodes(ctx, nodeIDs, fields)
	return entity.Wrapped(values, err, "loading batched field values (nodeCount %v)", len(nodeIDs))
}

// SetFieldValues saves the node's values of fields and the text formats
// they were written in (values, textFormats: by FieldInstanceDef.ID).
func (s *DefaultContentTypeService) SetFieldValues(ctx context.Context, nodeID int, fields entity.FieldInstanceDefs, values map[int]any, textFormats map[int]int) error {
	if err := s.repository.SetFieldValues(ctx, nodeID, fields, values, textFormats); err != nil {
		return fmt.Errorf("saving field values (nodeID %v): %w", nodeID, err)
	}
	return nil
}

// TextFormatIDsByNode - the text format (TextFormatDef.ID) each of the
// node's values of fields was saved with, by FieldInstanceDef.ID.
func (s *DefaultContentTypeService) TextFormatIDsByNode(ctx context.Context, nodeID int, fields entity.FieldInstanceDefs) (map[int]int, error) {
	formats, err := s.repository.TextFormatIDsByNode(ctx, nodeID, fields)
	return entity.Wrapped(formats, err, "loading field text format IDs (nodeID %v)", nodeID)
}

// FieldTextValuesForNodes - the text value of field fieldStorageID on each
// of the nodes nodeIDs that has one, by node ID.
func (s *DefaultContentTypeService) FieldTextValuesForNodes(ctx context.Context, fieldStorageID int, nodeIDs []int) (map[int]string, error) {
	values, err := s.repository.FieldTextValuesForNodes(ctx, fieldStorageID, nodeIDs)
	return entity.Wrapped(values, err, "batch loading field text values (fieldStorageID %v)", fieldStorageID)
}

// FieldNumberValuesForNodes - the number value of field fieldStorageID on
// each of the nodes nodeIDs that has one, by node ID.
func (s *DefaultContentTypeService) FieldNumberValuesForNodes(ctx context.Context, fieldStorageID int, nodeIDs []int) (map[int]float64, error) {
	values, err := s.repository.FieldNumberValuesForNodes(ctx, fieldStorageID, nodeIDs)
	return entity.Wrapped(values, err, "batch loading field number values (fieldStorageID %v)", fieldStorageID)
}

// FieldTaxonomyValuesForNodes - the term of taxonomy field fieldStorageID
// on each of the nodes nodeIDs that has one, by node ID.
func (s *DefaultContentTypeService) FieldTaxonomyValuesForNodes(ctx context.Context, fieldStorageID int, nodeIDs []int) (map[int]int, error) {
	values, err := s.repository.FieldTaxonomyValuesForNodes(ctx, fieldStorageID, nodeIDs)
	return entity.Wrapped(values, err, "batch loading field taxonomy values (fieldStorageID %v)", fieldStorageID)
}

// FieldTaxonomyMultiValuesForNodes - the terms of multi-value taxonomy
// field fieldStorageID on each of the nodes nodeIDs, by node ID.
func (s *DefaultContentTypeService) FieldTaxonomyMultiValuesForNodes(ctx context.Context, fieldStorageID int, nodeIDs []int) (map[int][]int, error) {
	values, err := s.repository.FieldTaxonomyMultiValuesForNodes(ctx, fieldStorageID, nodeIDs)
	return entity.Wrapped(values, err, "batch loading multi-value field taxonomy values (fieldStorageID %v)", fieldStorageID)
}

// FieldDateValuesForNodes - the date value of field fieldStorageID on each
// of the nodes nodeIDs that has one, by node ID.
func (s *DefaultContentTypeService) FieldDateValuesForNodes(ctx context.Context, fieldStorageID int, nodeIDs []int) (map[int]time.Time, error) {
	values, err := s.repository.FieldDateValuesForNodes(ctx, fieldStorageID, nodeIDs)
	return entity.Wrapped(values, err, "batch loading field date values (fieldStorageID %v)", fieldStorageID)
}

// isDuplicateKeyErr - see entity.IsDuplicateKey.
func isDuplicateKeyErr(err error) bool {
	return entity.IsDuplicateKey(err)
}
