package service

import (
	"context"
	"fmt"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

// PrivateFileAccess decides who may download a private file of a file
// field (entity.PrivateScheme): its uploader, and whoever may see - or
// edit - a content item it is attached to, by the same checks as the
// item's page (AuthService.CanViewNode, CanEditNode). Nobody else, even
// with the link.
type PrivateFileAccess struct {
	// NodesWithFile - the content items holding the file
	// (database.UserContentRepository.NodesWithFile).
	NodesWithFile func(ctx context.Context, fileID int) ([]int, error)
	NodeByID      func(ctx context.Context, id int) (*entity.Node, error)
	Auth          AuthService
}

// CanDownload reports whether user (nil - a visitor) may download f.
func (a PrivateFileAccess) CanDownload(ctx context.Context, user *entity.User, f entity.File) (bool, error) {
	if user != nil && f.CreatedBy != nil && f.CreatedBy.ID == user.ID {
		return true, nil
	}
	ids, err := a.NodesWithFile(ctx, f.ID)
	if err != nil {
		return false, err
	}
	for _, id := range ids {
		n, err := a.NodeByID(ctx, id)
		if IsNotFound(err) {
			continue
		}
		if err != nil {
			return false, fmt.Errorf("loading node %d of file %d: %w", id, f.ID, err)
		}
		if a.Auth.CanViewNode(ctx, n, user) || a.Auth.CanEditNode(ctx, n, user) {
			return true, nil
		}
	}
	return false, nil
}
