package service

import (
	"context"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

// DynamicContentService wraps one content-type-scoped
// entity.DynamicContentRepository, its errors wrapped and a missing node
// as entity.ErrEntityNotFound. Built per request by
// DynamicContentController, like the repository it wraps.
type DynamicContentService interface {
	ByNodeID(ctx context.Context, nodeID int) (*entity.DynamicContent, error)
	Create(ctx context.Context, e *entity.DynamicContent) (*entity.DynamicContent, error)
	Update(ctx context.Context, e *entity.DynamicContent) (*entity.DynamicContent, error)
}

// DefaultDynamicContentService - a content type's nodes, read and saved
// by its repository (built per type: DynamicContentController.dynamicService).
type DefaultDynamicContentService struct {
	repository entity.DynamicContentRepository
}

// NewDynamicContentService - the dynamic content service on repo (one
// content type's nodes).
func NewDynamicContentService(repo entity.DynamicContentRepository) DynamicContentService {
	return &DefaultDynamicContentService{repository: repo}
}

// ByNodeID - the node nodeID with its field values; none -
// entity.ErrEntityNotFound.
func (s *DefaultDynamicContentService) ByNodeID(ctx context.Context, nodeID int) (*entity.DynamicContent, error) {
	dc, err := s.repository.ByNodeID(ctx, nodeID)
	return entity.Found(dc, err, "finding dynamic content by node id (nodeID %v)", nodeID)
}

// Create saves the new node e with its field values.
func (s *DefaultDynamicContentService) Create(ctx context.Context, e *entity.DynamicContent) (*entity.DynamicContent, error) {
	created, err := s.repository.Create(ctx, e)
	return entity.Wrapped(created, err, "creating dynamic content")
}

// Update saves node e and its field values.
func (s *DefaultDynamicContentService) Update(ctx context.Context, e *entity.DynamicContent) (*entity.DynamicContent, error) {
	updated, err := s.repository.Update(ctx, e)
	return entity.Wrapped(updated, err, "updating dynamic content (nodeID %v)", e.Node.ID)
}
