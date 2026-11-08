package viewquery

import (
	"context"
	"fmt"
	"slices"
	"strconv"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/database"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/database/dao"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/comment"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/node"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/predicate"
)

// RunCommentView implements entity.ViewQuery.RunCommentView.
// Comments have no generic fields, so there is no field_value_*
// intersection and sorting is always a single builtin criterion
// (plain SQL ORDER BY + LIMIT/OFFSET). Only the comments on public
// content (database.CommentOnPublicNode). Eager-loads Node
// (id/title/slug) and Author.
func (e *Engine) RunCommentView(ctx context.Context, params entity.ViewRunParams) (entity.Comments, int, error) {
	client := database.GetClient(ctx, e.client)

	predicates, impossible, err := buildCommentFilterPredicates(params.Filters, params.QueryParams, params.ContextCreatedByID)
	if err != nil {
		return nil, 0, err
	}
	if impossible {
		return entity.Comments{}, 0, nil
	}
	predicates = append(predicates, database.CommentOnPublicNode())
	// Without a status filter, the published comments only: not those
	// waiting for approval nor spam.
	if !slices.ContainsFunc(params.Filters, func(f entity.ViewFilterDef) bool { return f.BuiltinField == entity.ViewFilterBuiltinCommentStatus }) {
		predicates = append(predicates, comment.Status(entity.CommentPublished))
	}

	skip, offset, pageSize := pageWindow(params)

	total, err := client.Comment.Query().Where(predicates...).Count(ctx)
	total = max(total-skip, 0)
	if err != nil || total == 0 {
		return entity.Comments{}, total, err
	}

	// Empty means "created_at desc", as in RunView. Only the first sort
	// criterion is honored - the admin UI offers just one for comment views.
	sortCriterion := entity.ViewRunSort{Builtin: entity.ViewSortBuiltinCreatedAt, Direction: "desc"}
	if len(params.Sorts) > 0 {
		sortCriterion = params.Sorts[0]
	}
	desc := sortCriterion.Direction != "asc"
	var order comment.OrderOption
	switch {
	case sortCriterion.Builtin == entity.ViewSortBuiltinCommentSubject && desc:
		order = generated.Desc(comment.FieldSubject)
	case sortCriterion.Builtin == entity.ViewSortBuiltinCommentSubject:
		order = generated.Asc(comment.FieldSubject)
	case desc:
		order = generated.Desc(comment.FieldCreatedAt)
	default:
		order = generated.Asc(comment.FieldCreatedAt)
	}

	res, err := client.Comment.Query().
		Where(predicates...).
		Order(order).
		WithNode(func(nq *generated.NodeQuery) {
			nq.Select(node.FieldID, node.FieldTitle, node.FieldSlug, node.FieldLangcode)
		}).
		WithAuthor().
		Limit(pageSize).
		Offset(offset).
		All(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("running the comment view: %w", err)
	}
	return dao.ToCommentsEntity(res), total, nil
}

// buildCommentFilterPredicates is buildFilterPredicates' counterpart for
// base_entity "comment": all filters are builtin fields, so each one maps
// to a single AND-combined predicate.Comment.
func buildCommentFilterPredicates(filters entity.ViewFilterDefs, queryParams map[string]string, contextCreatedByID int) (predicates []predicate.Comment, impossible bool, err error) {
	for _, f := range filters {
		// Contextual comment_created_by: the user from the page (e.g. a profile
		// tab). No context yields an empty result, not an unfiltered one.
		if f.Contextual && f.BuiltinField == entity.ViewFilterBuiltinCommentCreatedBy {
			if contextCreatedByID <= 0 {
				return nil, true, nil
			}
			predicates = append(predicates, comment.CreatedByEQ(contextCreatedByID))
			continue
		}
		isPresenceOperator := f.Operator == entity.ViewFilterOperatorEmpty || f.Operator == entity.ViewFilterOperatorNotEmpty
		var value string
		if !isPresenceOperator {
			v, ok := resolveViewFilterValue(f, queryParams)
			if !ok {
				continue
			}
			value = v
		}
		pred, err := builtinCommentFilterPredicate(f.BuiltinField, f.Operator, value)
		if err != nil {
			return nil, false, err
		}
		predicates = append(predicates, pred)
	}
	return predicates, false, nil
}

func builtinCommentFilterPredicate(builtinField entity.ViewFilterBuiltinField, operator entity.ViewFilterOperator, value string) (predicate.Comment, error) {
	switch builtinField {
	case entity.ViewFilterBuiltinCommentStatus:
		n, err := strconv.Atoi(value)
		if err != nil {
			return nil, fmt.Errorf("RunCommentView: invalid comment_status filter value %q: %w", value, err)
		}
		return comment.StatusEQ(int8(n)), nil
	case entity.ViewFilterBuiltinCommentCreatedBy:
		if operator == entity.ViewFilterOperatorIn {
			ids, err := parseIntList(value)
			if err != nil {
				return nil, fmt.Errorf("RunCommentView: invalid comment_created_by filter value %q: %w", value, err)
			}
			return comment.CreatedByIn(ids...), nil
		}
		n, err := strconv.Atoi(value)
		if err != nil {
			return nil, fmt.Errorf("RunCommentView: invalid comment_created_by filter value %q: %w", value, err)
		}
		return comment.CreatedByEQ(n), nil
	case entity.ViewFilterBuiltinCommentSubject:
		if operator == entity.ViewFilterOperatorContains {
			return comment.SubjectContainsFold(value), nil
		}
		return comment.SubjectEqualFold(value), nil
	case entity.ViewFilterBuiltinCommentContent:
		if operator == entity.ViewFilterOperatorContains {
			return comment.ContentContainsFold(value), nil
		}
		return comment.ContentEQ(value), nil
	case entity.ViewFilterBuiltinCommentNodeType:
		// Comment counterpart of ViewFilterBuiltinContentType.
		if operator == entity.ViewFilterOperatorIn {
			return comment.HasNodeWith(node.TypeIn(parseStringList(value)...)), nil
		}
		return comment.HasNodeWith(node.TypeEQ(value)), nil
	case entity.ViewFilterBuiltinCommentIsReply:
		if value == "1" {
			return comment.ParentIDNotNil(), nil
		}
		return comment.ParentIDIsNil(), nil
	default:
		return nil, fmt.Errorf("RunCommentView: unknown builtin field %q", builtinField)
	}
}
