package database

import (
	"context"
	"fmt"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/database/dao"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/file"
)

// NewFileRepository - the file repository on fileClient.
func NewFileRepository(fileClient *generated.FileClient) *FileRepository {
	return &FileRepository{fileClient: fileClient}
}

// FileRepository is the ent-backed entity.FileRepository.
type FileRepository struct {
	fileClient *generated.FileClient
}

// Create creates a new File record.
func (r FileRepository) Create(
	ctx context.Context,
	uri, mimeType string,
	width, height, sizeBytes int,
	createdBy int,
) (*entity.File, error) {
	file, err := r.fileClient.Create().
		SetURI(uri).
		SetMimeType(mimeType).
		SetSizeBytes(sizeBytes).
		SetWidth(width).
		SetHeight(height).
		SetCreatedBy(createdBy).
		SetAlt("").
		SetCreatedAt(time.Now()).
		SetUpdatedAt(time.Now()).
		Save(ctx)

	if err != nil {
		return nil, err
	}

	fileEntity := dao.ToFileEntity(file)
	return &fileEntity, nil
}

// ByID resolves a file by its id.
func (r FileRepository) ByID(ctx context.Context, id int) (*entity.File, error) {
	file, err := r.fileClient.Get(ctx, id)
	return one(file, err, dao.ToFileEntity, "file with id %d", id)
}

// UpdateAlt sets a file's alt text (see entity.FileRepository.UpdateAlt).
func (r FileRepository) UpdateAlt(ctx context.Context, id int, alt string) error {
	err := r.fileClient.UpdateOneID(id).
		SetAlt(alt).
		SetUpdatedAt(time.Now()).
		Exec(ctx)
	if err != nil {
		return notFoundAs(err, "file with id %d", id)
	}
	return nil
}

// SetFocal sets a file's focal point (see entity.FileRepository.SetFocal).
func (r FileRepository) SetFocal(ctx context.Context, id int, focal *entity.FocalPoint) error {
	u := r.fileClient.UpdateOneID(id).SetUpdatedAt(time.Now())
	if focal == nil {
		u = u.ClearFocalX().ClearFocalY()
	} else {
		u = u.SetFocalX(focal.X).SetFocalY(focal.Y)
	}
	if err := u.Exec(ctx); err != nil {
		return notFoundAs(err, "file with id %d", id)
	}
	return nil
}

// SetOriginalFilename sets a file's OriginalFilename (see
// entity.FileRepository.SetOriginalFilename).
func (r FileRepository) SetOriginalFilename(ctx context.Context, id int, name string) error {
	err := r.fileClient.UpdateOneID(id).
		SetOriginalFilename(name).
		SetUpdatedAt(time.Now()).
		Exec(ctx)
	if err != nil {
		return notFoundAs(err, "file with id %d", id)
	}
	return nil
}

// ByURI resolves a file by its URI (the first row of several sharing it).
func (r FileRepository) ByURI(ctx context.Context, uri string) (*entity.File, error) {
	f, err := r.fileClient.Query().Where(file.URI(uri)).Order(file.ByID()).First(ctx)
	return one(f, err, dao.ToFileEntity, "file %q", uri)
}

// ByIDs resolves multiple files in one query, keyed by id.
func (r FileRepository) ByIDs(ctx context.Context, ids []int) (map[int]entity.File, error) {
	if len(ids) == 0 {
		return map[int]entity.File{}, nil
	}
	rows, err := r.fileClient.Query().Where(file.IDIn(ids...)).All(ctx)
	if err != nil {
		return nil, fmt.Errorf("FileRepository.ByIDs: %w", err)
	}
	result := make(map[int]entity.File, len(rows))
	for _, row := range rows {
		result[row.ID] = dao.ToFileEntity(row)
	}
	return result, nil
}
