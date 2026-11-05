package service

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

// fakeCardinalityRepo - a field storage, the nodes with several of its
// values and the SetFieldMultiple calls it got.
type fakeCardinalityRepo struct {
	entity.ContentTypeRepository
	storage entity.FieldStorageDef
	several []int
	set     []bool
}

func (f *fakeCardinalityRepo) FieldStorageByID(context.Context, int) (*entity.FieldStorageDef, error) {
	return &f.storage, nil
}

func (f *fakeCardinalityRepo) NodesWithSeveralFieldValues(context.Context, entity.FieldStorageDef) ([]int, error) {
	return f.several, nil
}

func (f *fakeCardinalityRepo) SetFieldMultiple(_ context.Context, _ int, multiple bool) error {
	f.set = append(f.set, multiple)
	return nil
}

type passUnitOfWork struct{}

func (passUnitOfWork) Do(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }

// TestSetFieldMultiple - a list/taxonomy/reference field changes either
// way; going to one value is refused while a node has several; a type
// holding one value only can't change.
func TestSetFieldMultiple(t *testing.T) {
	ctx := context.Background()
	newService := func(repo *fakeCardinalityRepo) ContentTypeService {
		return NewContentTypeService(repo, nil, passUnitOfWork{}, slog.Default())
	}

	repo := &fakeCardinalityRepo{storage: entity.FieldStorageDef{ID: 7, FieldType: entity.FieldTypeList}}
	if err := newService(repo).SetFieldMultiple(ctx, 7, true); err != nil || len(repo.set) != 1 || !repo.set[0] {
		t.Errorf("to multiple: %v %v", err, repo.set)
	}

	repo = &fakeCardinalityRepo{storage: entity.FieldStorageDef{ID: 7, FieldType: entity.FieldTypeTaxonomy, Config: map[string]any{"multiple": true}}, several: []int{3, 5}}
	if err := newService(repo).SetFieldMultiple(ctx, 7, false); !errors.Is(err, ErrFieldHasSeveralValues) || len(repo.set) != 0 {
		t.Errorf("to single with several values: %v %v", err, repo.set)
	}
	repo.several = nil
	if err := newService(repo).SetFieldMultiple(ctx, 7, false); err != nil || len(repo.set) != 1 || repo.set[0] {
		t.Errorf("to single: %v %v", err, repo.set)
	}

	repo = &fakeCardinalityRepo{storage: entity.FieldStorageDef{ID: 7, FieldType: entity.FieldTypeText}}
	if err := newService(repo).SetFieldMultiple(ctx, 7, true); !errors.Is(err, ErrFieldCardinalityFixed) || len(repo.set) != 0 {
		t.Errorf("text field: %v %v", err, repo.set)
	}
	if ids, err := newService(repo).NodesWithSeveralFieldValues(ctx, 7); err != nil || ids != nil {
		t.Errorf("several values of a text field: %v %v", ids, err)
	}
}

// fakeTypeRepo - one content type and what CreateContentType/
// DeleteContentType got.
type fakeTypeRepo struct {
	entity.ContentTypeRepository
	def     entity.ContentTypeDef
	defs    entity.ContentTypeDefs
	listErr error
	created []string
	deleted []int
}

func (f *fakeTypeRepo) ContentTypeByID(context.Context, int) (*entity.ContentTypeDef, error) {
	return &f.def, nil
}

func (f *fakeTypeRepo) ContentTypes(context.Context) (entity.ContentTypeDefs, error) {
	return f.defs, f.listErr
}

func (f *fakeTypeRepo) CreateContentType(_ context.Context, ct entity.ContentTypeDef) (*entity.ContentTypeDef, error) {
	f.created = append(f.created, ct.MachineName)
	return &ct, nil
}

func (f *fakeTypeRepo) DeleteContentType(_ context.Context, id int) error {
	f.deleted = append(f.deleted, id)
	return nil
}

// typeNodeCount - n nodes of every type.
type typeNodeCount struct {
	entity.NodesRepository
	n int
}

func (c typeNodeCount) CountByType(context.Context, string) (int, error) { return c.n, nil }

// TestCreateContentTypeMachineName - a machine name is lowercase Latin
// letters, digits and _, starting with a letter; a module's type name
// stays reserved.
func TestCreateContentTypeMachineName(t *testing.T) {
	ctx := context.Background()
	for _, name := range []string{"event", "event_2", "e"} {
		repo := &fakeTypeRepo{}
		if _, err := NewContentTypeService(repo, nil, passUnitOfWork{}, slog.Default()).CreateContentType(ctx, entity.ContentTypeDef{MachineName: name}); err != nil || len(repo.created) != 1 {
			t.Errorf("%q: %v %v", name, err, repo.created)
		}
	}
	for _, name := range []string{"", "my type", "a/b", "Event", "2event", "_event", "event-2", "событие", strings.Repeat("a", 65)} {
		repo := &fakeTypeRepo{}
		if _, err := NewContentTypeService(repo, nil, passUnitOfWork{}, slog.Default()).CreateContentType(ctx, entity.ContentTypeDef{MachineName: name}); !errors.Is(err, ErrContentTypeMachineName) || len(repo.created) != 0 {
			t.Errorf("%q: %v %v", name, err, repo.created)
		}
	}
	if reserved := entity.ReservedContentTypeMachineNames(); len(reserved) > 0 {
		repo := &fakeTypeRepo{}
		if _, err := NewContentTypeService(repo, nil, passUnitOfWork{}, slog.Default()).CreateContentType(ctx, entity.ContentTypeDef{MachineName: reserved[0]}); !errors.Is(err, ErrContentTypeMachineNameReserved) {
			t.Errorf("reserved %q: %v", reserved[0], err)
		}
	}
}

// TestDeleteContentType - a type with content isn't deleted; one without
// is, in one transaction (the repository takes its fields with it).
func TestDeleteContentType(t *testing.T) {
	ctx := context.Background()
	repo := &fakeTypeRepo{def: entity.ContentTypeDef{ID: 3, MachineName: "event"}}
	uow := &countingUOW{}
	if err := NewContentTypeService(repo, typeNodeCount{n: 2}, uow, slog.Default()).DeleteContentType(ctx, 3); !errors.Is(err, ErrContentTypeHasNodes) || len(repo.deleted) != 0 {
		t.Errorf("with content: %v %v", err, repo.deleted)
	}
	uow = &countingUOW{}
	if err := NewContentTypeService(repo, typeNodeCount{}, uow, slog.Default()).DeleteContentType(ctx, 3); err != nil || len(repo.deleted) != 1 || uow.calls != 1 {
		t.Errorf("without content: %v %v, transactions %d", err, repo.deleted, uow.calls)
	}
}

// TestNodeTypeLabels - the content types' labels plus the modules' node
// types'; without the content types (a failure), the modules' alone.
func TestNodeTypeLabels(t *testing.T) {
	ctx := context.Background()
	repo := &fakeTypeRepo{defs: entity.ContentTypeDefs{{MachineName: "news", Label: "News"}, {MachineName: "video", Label: "Video"}}}
	got := NewContentTypeService(repo, nil, passUnitOfWork{}, slog.Default()).NodeTypeLabels(ctx)
	for typ, label := range map[string]string{"news": "News", "video": "Video", "poll": "Poll", "form": "Form"} {
		if got[typ] != label {
			t.Errorf("%s: got %q, want %q (all: %v)", typ, got[typ], label, got)
		}
	}

	repo = &fakeTypeRepo{listErr: errors.New("db down")}
	got = NewContentTypeService(repo, nil, passUnitOfWork{}, slog.New(slog.NewTextHandler(io.Discard, nil))).NodeTypeLabels(ctx)
	if _, ok := got["news"]; ok || got["poll"] != "Poll" {
		t.Errorf("on a failure: got %v, want the modules' types alone", got)
	}
}
