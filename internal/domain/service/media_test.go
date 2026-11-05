package service

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

// fakeMedia - files 1..n, the used ones in used.
type fakeMedia struct {
	entity.MediaRepository
	n    int
	used map[int]bool
}

func (f fakeMedia) MediaUseCounts(_ context.Context, ids []int) (map[int]int, error) {
	out := map[int]int{}
	for _, id := range ids {
		if f.used[id] {
			out[id] = 1
		}
	}
	return out, nil
}

func (f fakeMedia) MediaFiles(_ context.Context, filter entity.MediaFilter) ([]entity.File, int, error) {
	var all []entity.File
	for id := 1; id <= f.n; id++ {
		if !filter.Unused || !f.used[id] {
			all = append(all, entity.File{ID: id})
		}
	}
	end := min(filter.Offset+filter.Limit, len(all))
	if filter.Offset >= end {
		return nil, len(all), nil
	}
	return all[filter.Offset:end], len(all), nil
}

// fakeCleaner deletes the rows it's asked to, each one's file removable.
type fakeCleaner struct{ deleted []int }

func (c *fakeCleaner) NodeFileIDs(context.Context, []int) ([]int, error) { return nil, nil }
func (c *fakeCleaner) DeleteUnreferencedFiles(_ context.Context, ids []int) ([]string, error) {
	c.deleted = append(c.deleted, ids...)
	out := make([]string, len(ids))
	for i := range ids {
		out[i] = "public://f.jpg"
	}
	return out, nil
}

type fakeRemover struct{ removed int }

func (r *fakeRemover) RemoveStoredFile(context.Context, string) error { r.removed++; return nil }
func (r *fakeRemover) RemoveUserDir(context.Context, int) error       { return nil }

// TestMediaLibraryDelete - a file in use isn't deleted; one used nowhere
// is, with its stored file; DeleteUnused deletes every unused one, past
// a batch.
func TestMediaLibraryDelete(t *testing.T) {
	ctx := context.Background()
	cleaner, remover := &fakeCleaner{}, &fakeRemover{}
	m := &MediaLibrary{Repo: fakeMedia{n: 2*mediaDeleteBatch + 5, used: map[int]bool{1: true, 7: true}}, Cleaner: cleaner, Remover: remover}
	if err := m.Delete(ctx, 1); !errors.Is(err, ErrMediaInUse) || len(cleaner.deleted) != 0 {
		t.Fatalf("a file in use: %v, deleted %v", err, cleaner.deleted)
	}
	if err := m.Delete(ctx, 2); err != nil || !slices.Equal(cleaner.deleted, []int{2}) || remover.removed != 1 {
		t.Fatalf("an unused file: %v, deleted %v, removed %d", err, cleaner.deleted, remover.removed)
	}
	cleaner.deleted = nil
	n, err := m.DeleteUnused(ctx)
	if err != nil || n != 2*mediaDeleteBatch+3 || len(cleaner.deleted) != n || slices.Contains(cleaner.deleted, 7) {
		t.Fatalf("DeleteUnused = %d, %v; deleted %d", n, err, len(cleaner.deleted))
	}
}

// fakeFocalFiles - one file, its focal point set by SetFocal.
type fakeFocalFiles struct {
	entity.FileRepository
	file entity.File
	sets int
}

func (f *fakeFocalFiles) ByID(context.Context, int) (*entity.File, error) {
	file := f.file
	return &file, nil
}

func (f *fakeFocalFiles) SetFocal(_ context.Context, _ int, p *entity.FocalPoint) error {
	f.sets++
	f.file.Focal = p
	return nil
}

type fakeCopies struct{ dropped []string }

func (c *fakeCopies) RemoveDerivatives(_ context.Context, uri string) error {
	c.dropped = append(c.dropped, uri)
	return nil
}

// TestMediaLibrarySetFocal - a new point is saved and drops the file's
// derivatives; the same one changes nothing; one outside the image is
// refused.
func TestMediaLibrarySetFocal(t *testing.T) {
	ctx := context.Background()
	files := &fakeFocalFiles{file: entity.File{ID: 1, URI: "public://a.jpg"}}
	copies := &fakeCopies{}
	m := &MediaLibrary{Files: files, Copies: copies}
	if err := m.SetFocal(ctx, 1, &entity.FocalPoint{X: 20, Y: 40}); err != nil || files.sets != 1 || len(copies.dropped) != 1 {
		t.Fatalf("a new point: %v, sets %d, dropped %v", err, files.sets, copies.dropped)
	}
	if err := m.SetFocal(ctx, 1, &entity.FocalPoint{X: 20, Y: 40}); err != nil || files.sets != 1 {
		t.Fatalf("the same point: %v, sets %d", err, files.sets)
	}
	if err := m.SetFocal(ctx, 1, &entity.FocalPoint{X: 120, Y: 40}); err == nil || files.sets != 1 {
		t.Fatalf("a point outside: %v, sets %d", err, files.sets)
	}
	if err := m.SetFocal(ctx, 1, nil); err != nil || files.file.Focal != nil || len(copies.dropped) != 2 {
		t.Fatalf("removing the point: %v, %v", err, copies.dropped)
	}
}

type fakeUploads struct {
	entity.UploadSettingsRepository
}

func (fakeUploads) Get(context.Context) (*entity.UploadSettings, error) {
	return &entity.UploadSettings{MaxFileSizeBytes: 10 << 20, MaxImageDimension: 4000}, nil
}

// TestMediaPickerFilter - what a field's picker offers: an image field's
// images, public, its least resolution, the site's size; a file field's
// default extensions; a private field's private files - only the user's
// own unless they administer content.
func TestMediaPickerFilter(t *testing.T) {
	ctx := context.Background()
	m := &MediaLibrary{Uploads: fakeUploads{}}
	image := entity.FieldInstanceDef{FieldType: entity.FieldTypeImage, WidgetConfig: map[string]any{"min_resolution_width": float64(800), "min_resolution_height": float64(600)}}
	f, err := m.PickerFilter(ctx, image, 7, false)
	if err != nil || f.Kind != entity.MediaImages || f.Scheme != entity.PublicScheme || f.MinWidth != 800 || f.MinHeight != 600 || f.MaxBytes != 10<<20 || f.CreatedBy != 0 {
		t.Errorf("an image field: %+v %v", f, err)
	}
	file := entity.FieldInstanceDef{FieldType: entity.FieldTypeFile, WidgetConfig: map[string]any{"private_files": true}}
	f, _ = m.PickerFilter(ctx, file, 7, false)
	if f.Kind != entity.MediaAll || f.Scheme != entity.PrivateScheme || f.CreatedBy != 7 || !slices.Contains(f.Extensions, "pdf") {
		t.Errorf("a private file field: %+v", f)
	}
	if f, _ = m.PickerFilter(ctx, file, 7, true); f.CreatedBy != 0 {
		t.Errorf("a private file field, an administrator: %+v", f)
	}
}
