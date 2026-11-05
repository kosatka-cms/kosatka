package database

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/database/dao"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/comment"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/node"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/predicate"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/user"
)

// NewCommentRepository - the comment repository on client.
func NewCommentRepository(client *generated.Client) *CommentRepository {
	return &CommentRepository{client: client}
}

// CommentRepository provides database access to comments.
type CommentRepository struct {
	client *generated.Client
}

// Find returns comments with filters
func (r CommentRepository) Find(ctx context.Context, filter entity.CommentsFilter) (entity.Comments, error) {
	client := GetClient(ctx, r.client)
	query := client.Comment.Query()

	query = query.
		Limit(filter.Limit).
		Offset(filter.Offset).
		Select(
			comment.FieldID,
			comment.FieldNid,
			comment.FieldParentID,
			comment.FieldCreatedBy,
			comment.FieldSubject,
			comment.FieldContent,
			comment.FieldCreatedAt,
			comment.FieldUpdatedAt,
			comment.FieldStatus,
		).
		Order(generated.Asc(comment.FieldCreatedAt)).
		WithNode(func(nq *generated.NodeQuery) {
			nq.Select(
				node.FieldID,
				node.FieldTitle,
				node.FieldSlug,
				node.FieldLangcode,
			)
		}).
		WithAuthor(func(uq *generated.UserQuery) {
			uq.Select(
				user.FieldID,
				user.FieldUsername,
				user.FieldSlug,
				user.FieldSignature,
			)
		})

	if filter.Status != nil {
		query = query.Where(comment.Status(*filter.Status))
	}

	if filter.Nid != nil {
		query = query.Where(comment.Nid(*filter.Nid))
	}

	if filter.CreatedBy != nil {
		query = query.Where(comment.CreatedBy(*filter.CreatedBy))
	}

	if filter.ParentID != nil {
		query = query.Where(comment.ParentID(*filter.ParentID))
	}

	res, err := query.All(ctx)
	if err != nil {
		return nil, fmt.Errorf("CommentRepository.Find: %w", err)
	}

	return dao.ToCommentsEntity(res), nil
}

// ListForAdmin returns all comments for admin list with pagination and configurable sort.
func (r CommentRepository) ListForAdmin(ctx context.Context, filter entity.AdminCommentsListFilter) (entity.Comments, int, error) {
	client := GetClient(ctx, r.client)

	query := client.Comment.Query().
		Where(comment.Status(filter.Status)).
		WithNode().
		WithAuthor()

	switch filter.SortBy {
	case entity.AdminCommentsSortUpdated:
		if filter.Desc {
			query = query.Order(generated.Desc(comment.FieldUpdatedAt), generated.Desc(comment.FieldID))
		} else {
			query = query.Order(generated.Asc(comment.FieldUpdatedAt), generated.Asc(comment.FieldID))
		}
	default:
		if filter.Desc {
			query = query.Order(generated.Desc(comment.FieldCreatedAt), generated.Desc(comment.FieldID))
		} else {
			query = query.Order(generated.Asc(comment.FieldCreatedAt), generated.Asc(comment.FieldID))
		}
	}

	totalCount, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("CommentRepository.ListForAdmin: %w", err)
	}

	if filter.Limit > 0 {
		query = query.Limit(filter.Limit)
	}
	if filter.Offset > 0 {
		query = query.Offset(filter.Offset)
	}

	res, err := query.All(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("CommentRepository.ListForAdmin: %w", err)
	}

	return dao.ToCommentsEntity(res), totalCount, nil
}

// CommentOnPublicNode - the comments on public content only (published,
// not on moderation): what the public comment lists show.
func CommentOnPublicNode() predicate.Comment {
	return comment.HasNodeWith(node.Status(1), node.OnModeration(0))
}

// ListRecentPublished returns published comments site-wide (including forum) on public content, except on nodes of excludeTypes, newest first, with total count.
func (r CommentRepository) ListRecentPublished(ctx context.Context, status int8, excludeTypes []string, limit int, offset int) (entity.Comments, int, error) {
	client := GetClient(ctx, r.client)

	known, err := knownNodeTypes(ctx, client)
	if err != nil {
		return nil, 0, fmt.Errorf("CommentRepository.ListRecentPublished: %w", err)
	}
	base := client.Comment.Query().Where(comment.Status(status), CommentOnPublicNode(), comment.HasNodeWith(known))
	if len(excludeTypes) > 0 {
		base = base.Where(comment.HasNodeWith(node.TypeNotIn(excludeTypes...)))
	}

	totalCount, err := base.Clone().Count(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("CommentRepository.ListRecentPublished: %w", err)
	}

	query := base.
		Order(generated.Desc(comment.FieldCreatedAt)).
		Limit(limit).
		Offset(offset).
		WithNode(func(nq *generated.NodeQuery) {
			nq.Select(
				node.FieldID,
				node.FieldTitle,
				node.FieldSlug,
				node.FieldLangcode,
			)
		}).
		WithAuthor(func(uq *generated.UserQuery) {
			uq.Select(
				user.FieldID,
				user.FieldUsername,
				user.FieldSlug,
				user.FieldAvatar,
				user.FieldSignature,
			)
		})

	res, err := query.All(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("CommentRepository.ListRecentPublished: %w", err)
	}

	return dao.ToCommentsEntity(res), totalCount, nil
}

// TotalCount returns total count of comments with filters
func (r CommentRepository) TotalCount(ctx context.Context, filter entity.CommentsFilter) (int, error) {
	client := GetClient(ctx, r.client)
	query := client.Comment.Query()

	if filter.Status != nil {
		query = query.Where(comment.Status(*filter.Status))
	}

	if filter.Nid != nil {
		query = query.Where(comment.Nid(*filter.Nid))
	}

	if filter.CreatedBy != nil {
		query = query.Where(comment.CreatedBy(*filter.CreatedBy))
	}

	if filter.ParentID != nil {
		query = query.Where(comment.ParentID(*filter.ParentID))
	}

	totalCount, err := query.Count(ctx)
	if err != nil {
		return 0, fmt.Errorf("CommentRepository.TotalCount: %w", err)
	}

	return totalCount, nil
}

// ByID returns comment by ID
func (r CommentRepository) ByID(ctx context.Context, id int) (*entity.Comment, error) {
	client := GetClient(ctx, r.client)
	res, err := client.Comment.Query().
		Where(comment.ID(id)).
		// The whole node: who may see the comment is who may see it
		// (its status, type, author, trash, roles - CanViewNode).
		WithNode().
		WithAuthor(func(uq *generated.UserQuery) {
			uq.Select(
				user.FieldID,
				user.FieldUsername,
				user.FieldSlug,
				user.FieldSignature,
			)
		}).
		WithParent().
		WithChildren().
		Only(ctx)

	if err != nil {
		return nil, err
	}

	return dao.ToCommentEntity(res), nil
}

// ByNodeID returns all comments for a specific node (with hierarchy)
func (r CommentRepository) ByNodeID(ctx context.Context, nid int, status int8) (entity.Comments, error) {
	client := GetClient(ctx, r.client)
	// Load all comments of the node; the limit is applied to root comments
	// after the tree is built.
	res, err := client.Comment.Query().
		Where(
			comment.Nid(nid),
			comment.Status(status),
		).
		Order(generated.Asc(comment.FieldCreatedAt)).
		WithAuthor(func(uq *generated.UserQuery) {
			uq.Select(
				user.FieldID,
				user.FieldUsername,
				user.FieldSlug,
				user.FieldAvatar,
				user.FieldSignature,
			)
		}).
		All(ctx)

	if err != nil {
		return nil, fmt.Errorf("error finding comments by node: %w", err)
	}

	return dao.ToCommentsEntity(res), nil
}

// ByCreatedByPaginated returns user's comments ordered by CreatedAt desc
func (r CommentRepository) ByCreatedByPaginated(ctx context.Context, userID int, status int8, limit int, offset int) (entity.Comments, error) {
	client := GetClient(ctx, r.client)
	res, err := client.Comment.Query().
		Where(
			comment.CreatedBy(userID),
			comment.Status(status),
		).
		Order(generated.Desc(comment.FieldCreatedAt)).
		Limit(limit).
		Offset(offset).
		WithNode(func(nq *generated.NodeQuery) {
			nq.Select(
				node.FieldID,
				node.FieldTitle,
				node.FieldSlug,
				node.FieldLangcode,
			)
		}).
		WithAuthor(func(uq *generated.UserQuery) {
			uq.Select(
				user.FieldID,
				user.FieldUsername,
				user.FieldSlug,
				user.FieldAvatar,
				user.FieldSignature,
			)
		}).
		All(ctx)

	if err != nil {
		return nil, fmt.Errorf("error finding comments by user: %w", err)
	}

	return dao.ToCommentsEntity(res), nil
}

// ByNodeIDPaginated returns paginated comments in threaded order (materialized path).
// Single query with ORDER BY path - exactly limit comments per page.
func (r CommentRepository) ByNodeIDPaginated(ctx context.Context, nid int, status int8, limit int, offset int) (entity.Comments, error) {
	client := GetClient(ctx, r.client)
	res, err := client.Comment.Query().
		Where(
			comment.Nid(nid),
			comment.Status(status),
		).
		Order(generated.Asc(comment.FieldPath)).
		Limit(limit).
		Offset(offset).
		WithAuthor(func(uq *generated.UserQuery) {
			uq.Select(
				user.FieldID,
				user.FieldUsername,
				user.FieldSlug,
				user.FieldAvatar,
				user.FieldSignature,
			)
		}).
		All(ctx)

	if err != nil {
		return nil, fmt.Errorf("error finding paginated comments: %w", err)
	}

	return dao.ToCommentsEntity(res), nil
}

// Create creates a new comment and its materialized path (the parent's
// path plus its own ID) in one transaction (the caller's when ctx carries
// one): a reply never ends up a root.
func (r CommentRepository) Create(ctx context.Context, c *entity.Comment) (*entity.Comment, error) {
	var created *entity.Comment
	err := NewUnitOfWork(r.client).Do(ctx, func(ctx context.Context) error {
		client := GetClient(ctx, r.client)
		parentPath := ""
		if c.ParentID != nil && *c.ParentID != 0 {
			parent, err := client.Comment.Get(ctx, *c.ParentID)
			if err != nil {
				return fmt.Errorf("loading the parent comment (id %d): %w", *c.ParentID, err)
			}
			if parent.Path != nil {
				parentPath = *parent.Path
			}
		}
		query := client.Comment.Create().
			SetNid(c.Nid).
			SetCreatedBy(c.CreatedBy).
			SetContent(c.Content).
			SetStatus(c.Status).
			SetHoldReason(c.HoldReason)
		if c.ParentID != nil {
			query = query.SetParentID(*c.ParentID)
		}
		if c.Subject != nil {
			query = query.SetSubject(*c.Subject)
		}
		res, err := query.Save(ctx)
		if err != nil {
			return fmt.Errorf("error creating comment: %w", err)
		}
		// The path after the insert: it holds the comment's own ID.
		id := res.ID
		if res, err = client.Comment.UpdateOneID(id).SetPath(parentPath + "/" + commentPathSegment(id)).Save(ctx); err != nil {
			return fmt.Errorf("setting the path of comment %d: %w", id, err)
		}
		created = dao.ToCommentEntity(res)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("CommentRepository.Create: %w", err)
	}
	return created, nil
}

// commentPathSegment returns zero-padded ID for materialized path (10 digits)
func commentPathSegment(id int) string {
	return fmt.Sprintf("%010d", id)
}

// Update updates an existing comment
func (r CommentRepository) Update(ctx context.Context, id int, c *entity.Comment) (*entity.Comment, error) {
	client := GetClient(ctx, r.client)
	query := client.Comment.UpdateOneID(id)

	if c.Subject != nil {
		query = query.SetSubject(*c.Subject)
	}

	if c.Content != "" {
		query = query.SetContent(c.Content)
	}

	res, err := query.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("error updating comment: %w", err)
	}

	return dao.ToCommentEntity(res), nil
}

// HasDuplicate - see entity.CommentsRepository.HasDuplicate.
func (r CommentRepository) HasDuplicate(ctx context.Context, authorID int, content string, since time.Time) (bool, error) {
	ok, err := GetClient(ctx, r.client).Comment.Query().
		Where(comment.CreatedBy(authorID), comment.CreatedAtGTE(since), comment.Content(content)).Exist(ctx)
	if err != nil {
		return false, fmt.Errorf("looking for a duplicate comment of user %d: %w", authorID, err)
	}
	return ok, nil
}

// SpamBefore - see entity.CommentsRepository.SpamBefore.
func (r CommentRepository) SpamBefore(ctx context.Context, before time.Time) ([]int, error) {
	ids, err := GetClient(ctx, r.client).Comment.Query().
		Where(comment.Status(entity.CommentSpam), comment.UpdatedAtLT(before)).IDs(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing old spam comments: %w", err)
	}
	return ids, nil
}

// SetStatus - see entity.CommentsRepository.SetStatus.
func (r CommentRepository) SetStatus(ctx context.Context, id int, status int8, reason string) error {
	if err := GetClient(ctx, r.client).Comment.UpdateOneID(id).SetStatus(status).SetHoldReason(reason).Exec(ctx); err != nil {
		return fmt.Errorf("setting the status of comment %d: %w", id, err)
	}
	return nil
}

// Delete deletes a comment by ID
// Delete removes the comment and all its replies in one statement.
func (r CommentRepository) Delete(ctx context.Context, id int) error {
	client := GetClient(ctx, r.client)
	ids, err := r.DescendantIDs(ctx, id)
	if err != nil {
		return err
	}
	ids = append(ids, id)
	if _, err := client.Comment.Delete().Where(comment.IDIn(ids...)).Exec(ctx); err != nil {
		return fmt.Errorf("error deleting comment: %w", err)
	}
	return nil
}

// DescendantIDs walks the reply tree by parent_id, level by level (the
// materialized path may be missing on imported comments).
func (r CommentRepository) DescendantIDs(ctx context.Context, id int) ([]int, error) {
	client := GetClient(ctx, r.client)
	var all []int
	seen := map[int]bool{id: true}
	level := []int{id}
	for len(level) > 0 {
		children, err := client.Comment.Query().Where(comment.ParentIDIn(level...)).IDs(ctx)
		if err != nil {
			return nil, fmt.Errorf("error loading comment replies: %w", err)
		}
		level = level[:0:0]
		for _, c := range children {
			if !seen[c] {
				seen[c] = true
				all = append(all, c)
				level = append(level, c)
			}
		}
	}
	return all, nil
}

// DeleteByNodeID deletes every comment on the node.
func (r CommentRepository) DeleteByNodeID(ctx context.Context, nodeID int) error {
	client := GetClient(ctx, r.client)
	_, err := client.Comment.Delete().
		Where(comment.NidEQ(nodeID)).
		Exec(ctx)

	if err != nil {
		return fmt.Errorf("error deleting comments by nid: %w", err)
	}

	return nil
}

// GetCommentPath returns the materialized path for a comment
func (r CommentRepository) GetCommentPath(ctx context.Context, commentID int) (string, error) {
	client := GetClient(ctx, r.client)
	c, err := client.Comment.Query().
		Where(comment.ID(commentID)).
		Select(comment.FieldPath).
		Only(ctx)
	if err != nil {
		return "", fmt.Errorf("error getting comment path: %w", err)
	}
	if c.Path == nil || *c.Path == "" {
		return "", fmt.Errorf("comment %d has no path", commentID)
	}
	return *c.Path, nil
}

// CountCommentsWithPathLEQ returns count of comments with path <= given path (for same nid, status)
func (r CommentRepository) CountCommentsWithPathLEQ(ctx context.Context, nid int, status int8, path string) (int, error) {
	client := GetClient(ctx, r.client)
	count, err := client.Comment.Query().
		Where(
			comment.Nid(nid),
			comment.Status(status),
			comment.Or(comment.PathLT(path), comment.PathEQ(path)),
		).
		Count(ctx)
	if err != nil {
		return 0, fmt.Errorf("error counting comments: %w", err)
	}
	return count, nil
}

// CountByNodeID returns count of comments for a node
// If rootOnly is true, counts only root comments (without parent)
func (r CommentRepository) CountByNodeID(ctx context.Context, nid int, status int8, rootOnly bool) (int, error) {
	client := GetClient(ctx, r.client)
	query := client.Comment.Query().
		Where(
			comment.Nid(nid),
			comment.Status(status),
		)

	if rootOnly {
		query = query.Where(comment.ParentIDIsNil())
	}

	count, err := query.Count(ctx)
	if err != nil {
		return 0, fmt.Errorf("error counting comments: %w", err)
	}

	return count, nil
}

// CountByNodeIDs returns comment counts for multiple nodes in a single query.
func (r CommentRepository) CountByNodeIDs(ctx context.Context, nids []int, status int8, rootOnly bool) (map[int]int, error) {
	if len(nids) == 0 {
		return map[int]int{}, nil
	}
	query := GetClient(ctx, r.client).Comment.Query().Where(comment.Status(status), comment.NidIn(nids...))
	if rootOnly {
		query = query.Where(comment.ParentIDIsNil())
	}
	var rows []struct {
		Nid   int `json:"nid"`
		Count int `json:"count"`
	}
	if err := query.GroupBy(comment.FieldNid).Aggregate(generated.Count()).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("error counting comments by node ids: %w", err)
	}
	counts := make(map[int]int, len(rows))
	for _, row := range rows {
		counts[row.Nid] = row.Count
	}
	return counts, nil
}

// GetLatestComments returns the latest comments of every node type except
// those whose comments are a discussion of their own (a forum topic's,
// entity.OwnDiscussionNodeTypes), on public content only. Only the
// essential fields, no user relations.
func (r CommentRepository) GetLatestComments(ctx context.Context, limit int, status int8, excludeTypes []string) (entity.Comments, error) {
	client := GetClient(ctx, r.client)
	known, err := knownNodeTypes(ctx, client)
	if err != nil {
		return nil, fmt.Errorf("CommentRepository.GetLatestComments: %w", err)
	}
	q := client.Comment.Query().Where(comment.Status(status), CommentOnPublicNode(), comment.HasNodeWith(known))
	if exclude := slices.Concat(entity.OwnDiscussionNodeTypes(), excludeTypes); len(exclude) > 0 {
		q = q.Where(comment.HasNodeWith(node.TypeNotIn(exclude...)))
	}
	res, err := q.
		Order(generated.Desc(comment.FieldCreatedAt)).
		Limit(limit).
		Select(
			comment.FieldID,
			comment.FieldNid,
			comment.FieldSubject,
			comment.FieldContent,
			comment.FieldCreatedAt,
		).
		WithNode(func(nq *generated.NodeQuery) {
			nq.Select(
				node.FieldID,
				node.FieldType,
				node.FieldTitle,
				node.FieldSlug,
				node.FieldLangcode,
			)
		}).
		All(ctx)

	if err != nil {
		return nil, fmt.Errorf("error getting latest comments: %w", err)
	}

	return dao.ToCommentsEntity(res), nil
}

// NodeIDsByAuthor - see entity.CommentsRepository.NodeIDsByAuthor.
func (r CommentRepository) NodeIDsByAuthor(ctx context.Context, userID int) ([]int, error) {
	return GetClient(ctx, r.client).Comment.Query().
		Where(comment.CreatedBy(userID)).
		Unique(true).
		Select(comment.FieldNid).
		Ints(ctx)
}
