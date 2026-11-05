package entity

import (
	"context"
	"time"
)

// PreviewLink - a link showing a content item to whoever has it, even
// unpublished, until ExpiresAt (service.PreviewLinks): a draft shown to
// a reviewer without an account.
type PreviewLink struct {
	NodeID    int
	ExpiresAt time.Time
	CreatedBy int
	CreatedAt time.Time
}

// PreviewLinkRepository - the preview links, by the SHA-256 of their
// tokens (the token itself is shown once); a node has one at most.
type PreviewLinkRepository interface {
	// ReplacePreviewLink makes nodeID's link (an earlier one gone).
	ReplacePreviewLink(ctx context.Context, link PreviewLink, tokenHash string) error
	// PreviewLinkOf - nodeID's link; ErrEntityNotFound - none.
	PreviewLinkOf(ctx context.Context, nodeID int) (*PreviewLink, error)
	// PreviewLinkByHash - the link of a token's hash; ErrEntityNotFound -
	// none.
	PreviewLinkByHash(ctx context.Context, tokenHash string) (*PreviewLink, error)
	// DeletePreviewLinks removes nodeID's link.
	DeletePreviewLinks(ctx context.Context, nodeID int) error
}
