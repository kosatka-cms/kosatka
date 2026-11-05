package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/files"
)

// ErrMediaInUse - a file something still uses can't be deleted.
var ErrMediaInUse = errors.New("the file is in use")

// MediaLibrary - the media library: the files uploaded to the site
// (Repo), where each is used, its alt text and focal point, and deleting
// the ones nothing uses (Cleaner deletes the rows, Remover the stored
// files; Copies - a file's style derivatives, made again at a new focal
// point).
type MediaLibrary struct {
	Repo    entity.MediaRepository
	Files   entity.FileRepository
	Cleaner files.NodeFileCleaner
	Remover files.StoredFileRemover
	Copies  interface {
		RemoveDerivatives(ctx context.Context, uri string) error
	}
	// Uploads - the site-wide upload policy, beside a field's own
	// (PickerFilter).
	Uploads entity.UploadSettingsRepository
}

// PickerFilter - the media library's files field's form may take: its
// kind, its store (public or private files), the extensions, size and
// least resolution of its upload policy (entity.ResolveFieldUploadPolicy);
// of the private files only userID's own unless they administer content.
func (m *MediaLibrary) PickerFilter(ctx context.Context, field entity.FieldInstanceDef, userID int, administers bool) (entity.MediaFilter, error) {
	sitewide := entity.UploadSettings{}
	if m.Uploads != nil {
		s, err := m.Uploads.Get(ctx)
		if err != nil {
			return entity.MediaFilter{}, fmt.Errorf("the upload policy: %w", err)
		}
		sitewide = *s
	}
	policy := entity.ResolveFieldUploadPolicy(field, sitewide)
	f := entity.MediaFilter{Scheme: entity.PublicScheme, Extensions: policy.AllowedExtensions, MaxBytes: policy.MaxFileSizeBytes}
	if field.FieldType == entity.FieldTypeImage {
		f.Kind = entity.MediaImages
		f.MinWidth, f.MinHeight = policy.MinWidth, policy.MinHeight
	}
	if field.PrivateFiles() {
		f.Scheme = entity.PrivateScheme
		if !administers {
			f.CreatedBy = userID
		}
	}
	return f, nil
}

// Pickable - those of ids filter (PickerFilter) finds.
func (m *MediaLibrary) Pickable(ctx context.Context, filter entity.MediaFilter, ids []int) (map[int]bool, error) {
	out := map[int]bool{}
	if len(ids) == 0 {
		return out, nil
	}
	filter.IDs, filter.Offset, filter.Limit = ids, 0, len(ids)
	files, _, err := m.Repo.MediaFiles(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("checking the picked files: %w", err)
	}
	for _, f := range files {
		out[f.ID] = true
	}
	return out, nil
}

// SetFocal sets the file's focal point (nil - none); a new one drops the
// file's style derivatives, made again around it on request.
func (m *MediaLibrary) SetFocal(ctx context.Context, id int, focal *entity.FocalPoint) error {
	f, err := m.Files.ByID(ctx, id)
	if err != nil {
		return fmt.Errorf("setting the focal point of file %d: %w", id, err)
	}
	if focal != nil && !focal.Valid() {
		return fmt.Errorf("setting the focal point of file %d: %v is outside the image", id, *focal)
	}
	if (f.Focal == nil && focal == nil) || (f.Focal != nil && focal != nil && *f.Focal == *focal) {
		return nil
	}
	if err := m.Files.SetFocal(ctx, id, focal); err != nil {
		return fmt.Errorf("setting the focal point of file %d: %w", id, err)
	}
	if m.Copies != nil {
		if err := m.Copies.RemoveDerivatives(ctx, f.URI); err != nil {
			return fmt.Errorf("dropping the derivatives of file %d: %w", id, err)
		}
	}
	return nil
}

// mediaDeleteBatch - how many unused files DeleteUnused deletes at once.
const mediaDeleteBatch = 200

// Delete deletes the file and its stored copies; ErrMediaInUse if
// something uses it.
func (m *MediaLibrary) Delete(ctx context.Context, id int) error {
	counts, err := m.Repo.MediaUseCounts(ctx, []int{id})
	if err != nil {
		return fmt.Errorf("deleting file %d: %w", id, err)
	}
	if counts[id] > 0 {
		return ErrMediaInUse
	}
	return m.remove(ctx, []int{id})
}

// DeleteUnused deletes every file nothing uses; how many it deleted.
func (m *MediaLibrary) DeleteUnused(ctx context.Context) (int, error) {
	var ids []int
	for offset := 0; ; offset += mediaDeleteBatch {
		batch, _, err := m.Repo.MediaFiles(ctx, entity.MediaFilter{Unused: true, Offset: offset, Limit: mediaDeleteBatch})
		if err != nil {
			return 0, fmt.Errorf("finding the unused files: %w", err)
		}
		for _, f := range batch {
			ids = append(ids, f.ID)
		}
		if len(batch) < mediaDeleteBatch {
			break
		}
	}
	for start := 0; start < len(ids); start += mediaDeleteBatch {
		if err := m.remove(ctx, ids[start:min(start+mediaDeleteBatch, len(ids))]); err != nil {
			return 0, err
		}
	}
	return len(ids), nil
}

// remove deletes the rows of ids nothing uses and the stored files no
// other row shares.
func (m *MediaLibrary) remove(ctx context.Context, ids []int) error {
	uris, err := m.Cleaner.DeleteUnreferencedFiles(ctx, ids)
	if err != nil {
		return fmt.Errorf("deleting files %v: %w", ids, err)
	}
	var errs []error
	for _, uri := range uris {
		if err := m.Remover.RemoveStoredFile(ctx, uri); err != nil {
			errs = append(errs, fmt.Errorf("removing %s: %w", uri, err))
		}
	}
	return errors.Join(errs...)
}
