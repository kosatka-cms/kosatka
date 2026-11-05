package files

import (
	"context"
	"log/slog"
)

// StoredFileRemover - removal of stored files (implemented by
// FileUploadService).
type StoredFileRemover interface {
	RemoveStoredFile(ctx context.Context, uri string) error
	RemoveUserDir(ctx context.Context, userID int) error
}

// NodeFileCleaner - the file queries a content delete needs
// (implemented by database.UserContentRepository).
type NodeFileCleaner interface {
	// NodeFileIDs - files attached to these nodes (node files, the modules',
	// file/image fields); call before the nodes are deleted.
	NodeFileIDs(ctx context.Context, nodeIDs []int) ([]int, error)
	// DeleteUnreferencedFiles deletes the rows nothing references any
	// more and returns the URIs safe to remove from the store.
	DeleteUnreferencedFiles(ctx context.Context, fileIDs []int) ([]string, error)
}

// RemoveUnreferenced deletes the file rows (from fileIDs) nothing
// references any more and their stored files. Failures only log: the
// content they belonged to is already gone, a leftover file is not
// worth failing the deletion.
func RemoveUnreferenced(ctx context.Context, cleaner NodeFileCleaner, files StoredFileRemover, logger *slog.Logger, fileIDs []int) {
	if len(fileIDs) == 0 {
		return
	}
	uris, err := cleaner.DeleteUnreferencedFiles(ctx, fileIDs)
	if err != nil {
		logger.Warn("delete unreferenced files failed", "fileIDs", fileIDs, "error", err)
		return
	}
	for _, uri := range uris {
		if err := files.RemoveStoredFile(ctx, uri); err != nil {
			logger.Warn("remove stored file failed", "uri", uri, "error", err)
		}
	}
}
