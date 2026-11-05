package database

import (
	"context"
	"fmt"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/previewlink"
)

// PreviewLinkRepository - see entity.PreviewLinkRepository.
type PreviewLinkRepository struct {
	client *generated.Client
}

// NewPreviewLinkRepository - the preview link repository on client.
func NewPreviewLinkRepository(client *generated.Client) *PreviewLinkRepository {
	return &PreviewLinkRepository{client: client}
}

func toPreviewLink(row *generated.PreviewLink) entity.PreviewLink {
	return entity.PreviewLink{NodeID: row.Nid, ExpiresAt: row.ExpiresAt, CreatedBy: row.CreatedBy, CreatedAt: row.CreatedAt}
}

// ReplacePreviewLink - see entity.PreviewLinkRepository.ReplacePreviewLink.
func (r *PreviewLinkRepository) ReplacePreviewLink(ctx context.Context, link entity.PreviewLink, tokenHash string) error {
	return NewUnitOfWork(r.client).Do(ctx, func(ctx context.Context) error {
		client := GetClient(ctx, r.client)
		if _, err := client.PreviewLink.Delete().Where(previewlink.Nid(link.NodeID)).Exec(ctx); err != nil {
			return fmt.Errorf("removing node %d's preview link: %w", link.NodeID, err)
		}
		c := client.PreviewLink.Create().SetNid(link.NodeID).SetTokenHash(tokenHash).
			SetExpiresAt(link.ExpiresAt).SetCreatedAt(link.CreatedAt)
		if link.CreatedBy > 0 {
			c = c.SetCreatedBy(link.CreatedBy)
		}
		if err := c.Exec(ctx); err != nil {
			return fmt.Errorf("saving node %d's preview link: %w", link.NodeID, err)
		}
		return nil
	})
}

// PreviewLinkOf - see entity.PreviewLinkRepository.PreviewLinkOf.
func (r *PreviewLinkRepository) PreviewLinkOf(ctx context.Context, nodeID int) (*entity.PreviewLink, error) {
	row, err := GetClient(ctx, r.client).PreviewLink.Query().Where(previewlink.Nid(nodeID)).First(ctx)
	return one(row, err, toPreviewLink, "preview link of node %d", nodeID)
}

// PreviewLinkByHash - see entity.PreviewLinkRepository.PreviewLinkByHash.
func (r *PreviewLinkRepository) PreviewLinkByHash(ctx context.Context, tokenHash string) (*entity.PreviewLink, error) {
	row, err := GetClient(ctx, r.client).PreviewLink.Query().Where(previewlink.TokenHash(tokenHash)).Only(ctx)
	return one(row, err, toPreviewLink, "preview link")
}

// DeletePreviewLinks - see entity.PreviewLinkRepository.DeletePreviewLinks.
func (r *PreviewLinkRepository) DeletePreviewLinks(ctx context.Context, nodeID int) error {
	if _, err := GetClient(ctx, r.client).PreviewLink.Delete().Where(previewlink.Nid(nodeID)).Exec(ctx); err != nil {
		return fmt.Errorf("removing node %d's preview link: %w", nodeID, err)
	}
	return nil
}
