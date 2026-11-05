package entity

import "context"

// UserContentTypeCount - number of nodes of one type created by a user.
type UserContentTypeCount struct {
	Type  string
	Count int
}

// UserModuleContentCount - how much of a module's content a user has:
// Label - its translation source with @count ("private messages: @count").
type UserModuleContentCount struct {
	Label string
	Count int
}

// UserContentSummary - what will be deleted along with a user's content;
// shown on the confirmation page before the deletion.
type UserContentSummary struct {
	NodesByType []UserContentTypeCount
	Nodes       int
	Comments    int
	// Modules - the modules' content (UserDeletionService.AddModuleContent).
	Modules []UserModuleContentCount
	Files   int
}

// UserContentRepository - queries and deletes for everything a user owns
// that per-entity repositories don't cover. Nodes themselves are deleted
// via AdminBulkService.DeleteNode so all its cascades apply.
type UserContentRepository interface {
	Summary(ctx context.Context, userID int) (UserContentSummary, error)
	// NodeIDs - IDs of the user's nodes (any type/status), at most limit.
	NodeIDs(ctx context.Context, userID int, limit int) ([]int, error)
	// NodeFileIDs - files attached to these nodes (node files, the modules',
	// file fields). Must be called before the nodes are deleted, while the
	// links still exist.
	NodeFileIDs(ctx context.Context, nodeIDs []int) ([]int, error)
	// FileIDsByCreator - files uploaded by the user.
	FileIDsByCreator(ctx context.Context, userID int) ([]int, error)
	// DeleteUnreferencedFiles deletes the file rows among fileIDs that
	// nothing references any more and returns the URIs safe to remove from
	// disk (no remaining file row points to the same URI).
	DeleteUnreferencedFiles(ctx context.Context, fileIDs []int) ([]string, error)
	// ModuleInstalled - whether the module was installed: a module never
	// installed has no tables, so no content of the user.
	ModuleInstalled(ctx context.Context, module string) (bool, error)
}
