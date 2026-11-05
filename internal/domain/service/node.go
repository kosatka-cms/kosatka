package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/pkg/slug"
)

// NodesService - the nodes (content items) of every type: loading one,
// publishing it, its slug and its page's SEO.
type NodesService interface {
	ByID(ctx context.Context, nodeID int) (*entity.Node, error)
	BySlug(ctx context.Context, slug string) (*entity.Node, error)
	Publish(ctx context.Context, nodeID int) error
	Unpublish(ctx context.Context, nodeID int) error
	GenerateUniqueSlug(ctx context.Context, title string, contentType entity.ContentType) (string, error)
	UpsertPageSEO(ctx context.Context, nodeID int, metaTitle, metaDescription string) error
}

// DefaultNodesService - the NodesService on the nodes' repository.
type DefaultNodesService struct {
	repository entity.NodesRepository
	logger     *slog.Logger
}

// NewNodesService - the nodes service on r (the nodes) and baseLogger.
func NewNodesService(
	r entity.NodesRepository,
	baseLogger *slog.Logger,
) NodesService {
	return &DefaultNodesService{
		repository: r,
		logger:     baseLogger.With("component", "nodes.service"),
	}
}

// ByID - the node nodeID.
func (s *DefaultNodesService) ByID(ctx context.Context, nodeID int) (*entity.Node, error) {
	node, err := s.repository.ByID(ctx, nodeID)
	return entity.Found(node, err, "getting node by ID (nodeID %v)", nodeID)
}

// BySlug - the node whose slug is slug.
func (s *DefaultNodesService) BySlug(ctx context.Context, slug string) (*entity.Node, error) {
	node, err := s.repository.BySlug(ctx, slug)
	return entity.Found(node, err, "getting node by slug (slug %v)", slug)
}

// Publish publishes the node nodeID.
func (s *DefaultNodesService) Publish(ctx context.Context, nodeID int) error {
	err := s.repository.UpdateStatus(ctx, nodeID, 1)
	if err != nil {
		return fmt.Errorf("publishing node (nodeID %v): %w", nodeID, err)
	}

	return nil
}

// Unpublish unpublishes the node nodeID.
func (s *DefaultNodesService) Unpublish(ctx context.Context, nodeID int) error {
	err := s.repository.UpdateStatus(ctx, nodeID, 0)
	if err != nil {
		return fmt.Errorf("unpublishing node (nodeID %v): %w", nodeID, err)
	}

	return nil
}

// GenerateUniqueSlug - a slug for a node of contentType titled title:
// "<type>/<title's slug>", a number added when it is taken (a time
// suffix when that can't be checked).
func (s *DefaultNodesService) GenerateUniqueSlug(ctx context.Context, title string, contentType entity.ContentType) (string, error) {
	baseSlug := slug.Generate(title)
	if baseSlug == "" {
		baseSlug = fmt.Sprintf("%d", time.Now().Unix())
	}

	// "Pollution" of the poll type is poll/pollution, not pollution.
	if !strings.HasPrefix(baseSlug, contentType.ToString()+"/") {
		baseSlug = fmt.Sprintf("%s/%s", contentType, baseSlug)
	}

	// Check uniqueness and append a number if needed.
	uniqueSlug, err := s.ensureUniqueSlug(ctx, baseSlug)
	if err != nil {
		// The slug gets a time suffix instead (unique enough); the node is saved.
		LogFallback(s.logger, err, "ensuring slug uniqueness", "baseSlug", baseSlug)
		return fmt.Sprintf("%s-%d", baseSlug, time.Now().Unix()), nil
	}

	return uniqueSlug, nil
}

// ensureUniqueSlug makes slug unique by checking the database.
func (s *DefaultNodesService) ensureUniqueSlug(ctx context.Context, baseSlug string) (string, error) {
	// Check if the base slug is available
	exists, err := s.repository.SlugExists(ctx, baseSlug)
	if err != nil {
		return "", err
	}

	if !exists {
		return baseSlug, nil
	}

	// Try appending numbers until we find a unique slug
	for i := 1; i < slugAttempts; i++ {
		candidate := fmt.Sprintf("%s-%d", baseSlug, i)
		exists, err := s.repository.SlugExists(ctx, candidate)
		if err != nil {
			return "", err
		}
		if !exists {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("could not generate unique slug after %d attempts", slugAttempts-1)
}

// slugAttempts bounds the numbered candidates ensureUniqueSlug tries.
const slugAttempts = 100

// UpsertPageSEO saves the node's own meta title and meta description.
func (s *DefaultNodesService) UpsertPageSEO(ctx context.Context, nodeID int, metaTitle, metaDescription string) error {
	if err := s.repository.UpsertPageSEO(ctx, nodeID, metaTitle, metaDescription); err != nil {
		return fmt.Errorf("saving the page SEO of node %v: %w", nodeID, err)
	}
	return nil
}
