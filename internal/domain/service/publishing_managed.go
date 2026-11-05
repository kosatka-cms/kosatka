package service

import "context"

// publishingManaged - whether a module publishes a content type's items
// itself (an editorial workflow, SetPublishingManaged; nil - no module
// does).
var publishingManaged func(ctx context.Context, contentType string) bool

// SetPublishingManaged sets which content types a module publishes
// itself, at startup (nil - none): the core's own ways of publishing -
// the bulk actions, the publish buttons' routes, the schedule - leave
// their items alone, their state would no longer match.
func SetPublishingManaged(fn func(ctx context.Context, contentType string) bool) {
	publishingManaged = fn
}

// PublishingManaged - contentType's items are published by a module's
// own rules (SetPublishingManaged), not by the core's.
func PublishingManaged(ctx context.Context, contentType string) bool {
	return publishingManaged != nil && publishingManaged(ctx, contentType)
}
