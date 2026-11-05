package service

import (
	"context"
	"fmt"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/pkg/randtoken"
)

// PreviewLinkTTL - how long a preview link works.
const PreviewLinkTTL = 7 * 24 * time.Hour

// PreviewLinkPath - where a preview link leads: PreviewLinkPath + token.
const PreviewLinkPath = "/preview/"

// PreviewLinks - the links showing a content item to whoever has them,
// even unpublished (entity.PreviewLink): a node has one at most, made
// anew to replace it, revoked by its editors, working for PreviewLinkTTL.
type PreviewLinks struct {
	Repo entity.PreviewLinkRepository
	// now - the clock (tests set it); nil - time.Now.
	now func() time.Time
}

func (p *PreviewLinks) clock() time.Time {
	if p.now != nil {
		return p.now()
	}
	return time.Now()
}

// Create makes nodeID's link by user by, replacing an earlier one: its
// token (shown once: only its hash is kept) and when it expires.
func (p *PreviewLinks) Create(ctx context.Context, nodeID, by int) (token string, expires time.Time, err error) {
	token, err = randtoken.Generate(24)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("making a preview token: %w", err)
	}
	now := p.clock()
	link := entity.PreviewLink{NodeID: nodeID, ExpiresAt: now.Add(PreviewLinkTTL), CreatedBy: by, CreatedAt: now}
	if err := p.Repo.ReplacePreviewLink(ctx, link, randtoken.Hash(token)); err != nil {
		return "", time.Time{}, err
	}
	return token, link.ExpiresAt, nil
}

// Active - nodeID's link while it works; ok=false - none, or expired.
func (p *PreviewLinks) Active(ctx context.Context, nodeID int) (link *entity.PreviewLink, ok bool, err error) {
	link, err = p.Repo.PreviewLinkOf(ctx, nodeID)
	if entity.IsNotFound(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("loading node %d's preview link: %w", nodeID, err)
	}
	return link, p.clock().Before(link.ExpiresAt), nil
}

// Revoke ends nodeID's link.
func (p *PreviewLinks) Revoke(ctx context.Context, nodeID int) error {
	return p.Repo.DeletePreviewLinks(ctx, nodeID)
}

// Resolve - the node token's link shows; ok=false - no such link, or it
// expired.
func (p *PreviewLinks) Resolve(ctx context.Context, token string) (nodeID int, ok bool, err error) {
	if token == "" {
		return 0, false, nil
	}
	link, err := p.Repo.PreviewLinkByHash(ctx, randtoken.Hash(token))
	if entity.IsNotFound(err) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("resolving a preview link: %w", err)
	}
	if !p.clock().Before(link.ExpiresAt) {
		return 0, false, nil
	}
	return link.NodeID, true, nil
}
