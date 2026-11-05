package viewquery

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	entsql "entgo.io/ent/dialect/sql"
	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/database"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/fieldvalueblock"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/fieldvaluebool"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/fieldvaluedate"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/fieldvaluelist"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/fieldvaluelistmulti"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/fieldvaluenumber"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/fieldvaluereference"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/fieldvaluereferencemulti"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/fieldvaluetaxonomy"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/fieldvaluetaxonomymulti"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/fieldvaluetext"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/fieldvalueuser"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/node"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/predicate"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/taxonomyindex"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/taxonomytermdata"
	"github.com/kosatka-cms/kosatka/pkg/sitetime"
)

// currentNodeTermIDs returns nodeID's taxonomy terms from taxonomy_index,
// restricted to vocabulary vid when vid > 0.
func currentNodeTermIDs(ctx context.Context, client *generated.Client, nodeID, vid int) ([]int, error) {
	q := client.TaxonomyIndex.Query().Where(taxonomyindex.Nid(nodeID))
	if vid > 0 {
		q = q.Where(taxonomyindex.HasTaxonomyTermDataWith(taxonomytermdata.Vid(vid)))
	}
	rows, err := q.All(ctx)
	if err != nil {
		return nil, fmt.Errorf("currentNodeTermIDs: %w", err)
	}
	seen := map[int]bool{}
	var tids []int
	for _, r := range rows {
		if !seen[r.Tid] {
			seen[r.Tid] = true
			tids = append(tids, r.Tid)
		}
	}
	return tids, nil
}

// referenceRelatedNodeIDs returns the target node IDs stored in reference
// field fieldStorageID for nodeID (single- and multi-value tables), plus,
// when bothDirections, the nodes whose same field references nodeID.
func referenceRelatedNodeIDs(ctx context.Context, client *generated.Client, fieldStorageID, nodeID int, bothDirections bool) ([]int, error) {
	seen := map[int]bool{}
	var ids []int
	add := func(id int) {
		if id > 0 && id != nodeID && !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	single, err := client.FieldValueReference.Query().
		Where(fieldvaluereference.FieldIDEQ(fieldStorageID), fieldvaluereference.NodeID(nodeID)).All(ctx)
	if err != nil {
		return nil, fmt.Errorf("referenceRelatedNodeIDs: %w", err)
	}
	for _, r := range single {
		add(r.TargetNodeID)
	}
	multi, err := client.FieldValueReferenceMulti.Query().
		Where(fieldvaluereferencemulti.FieldIDEQ(fieldStorageID), fieldvaluereferencemulti.NodeID(nodeID)).All(ctx)
	if err != nil {
		return nil, fmt.Errorf("referenceRelatedNodeIDs: %w", err)
	}
	for _, r := range multi {
		add(r.TargetNodeID)
	}
	if bothDirections {
		revSingle, err := client.FieldValueReference.Query().
			Where(fieldvaluereference.FieldIDEQ(fieldStorageID), fieldvaluereference.TargetNodeID(nodeID)).All(ctx)
		if err != nil {
			return nil, fmt.Errorf("referenceRelatedNodeIDs: %w", err)
		}
		for _, r := range revSingle {
			add(r.NodeID)
		}
		revMulti, err := client.FieldValueReferenceMulti.Query().
			Where(fieldvaluereferencemulti.FieldIDEQ(fieldStorageID), fieldvaluereferencemulti.TargetNodeID(nodeID)).All(ctx)
		if err != nil {
			return nil, fmt.Errorf("referenceRelatedNodeIDs: %w", err)
		}
		for _, r := range revMulti {
			add(r.NodeID)
		}
	}
	return ids, nil
}

// buildFilterPredicates resolves filters into the AND-combined
// []predicate.Node used by RunView. Shared with matchingNodeIDs so the
// glossary sees exactly the same candidate set. impossible=true means a
// positive constraint already intersected to the empty set - the caller
// should short-circuit instead of querying. similar is the ranking of a
// similar_to_current_node filter (nil without one), RunView's order.
func buildFilterPredicates(ctx context.Context, client *generated.Client, filters entity.ViewFilterDefs, fieldStorages map[int]entity.FieldStorageDef, queryParams map[string]string, contextCreatedByID, contextNodeID, contextTermID int) (predicates []predicate.Node, impossible bool, similar similarRanking, err error) {
	// No implicit content-type scoping: a View is scoped through an ordinary
	// ViewFilterBuiltinContentType filter (see entity.ViewDef).
	q := &filterQuery{ctx: ctx, client: client, fieldStorages: fieldStorages, queryParams: queryParams,
		createdBy: contextCreatedByID, nodeID: contextNodeID, termID: contextTermID}
	for _, f := range filters {
		var possible bool
		if f.Contextual {
			possible, err = q.addContextual(f)
		} else {
			possible, err = q.add(f)
		}
		if err != nil {
			return nil, false, nil, err
		}
		if !possible {
			return nil, true, nil, nil
		}
	}
	if q.haveIDs {
		q.predicates = append(q.predicates, node.IDIn(q.ids...))
	}
	return q.predicates, false, q.similar, nil
}

// filterQuery - buildFilterPredicates under way: what the filters read
// (the field storages, the request's query parameters, the context's
// user, node and term) and what they made so far - the predicates, the
// positive constraint (ids, haveIDs: the nodes must be among them,
// intersected filter after filter) and the similar ranking. Each add
// reports false when the query can match nothing any more.
type filterQuery struct {
	ctx                       context.Context
	client                    *generated.Client
	fieldStorages             map[int]entity.FieldStorageDef
	queryParams               map[string]string
	createdBy, nodeID, termID int

	predicates []predicate.Node
	ids        []int
	haveIDs    bool
	similar    similarRanking
}

// addContextual adds a contextual filter (created_by or
// similar_to_current_node, enforced by views.DefaultService.validateFilter):
// its value is the currently viewed node's. Without that context
// (page-level View, cache refresh, glossary query) the whole query is
// impossible rather than unfiltered, so it can't leak every author's
// content. See entity.ViewFilterDef.Contextual.
func (q *filterQuery) addContextual(f entity.ViewFilterDef) (bool, error) {
	switch {
	case f.FieldStorageID != nil && f.Operator == entity.ViewFilterOperatorIsProfileUser:
		// User field: nodes naming the profile's user in it.
		if q.createdBy <= 0 {
			return false, nil
		}
		ids, err := q.client.FieldValueUser.Query().
			Where(fieldvalueuser.FieldIDEQ(*f.FieldStorageID), fieldvalueuser.UserID(q.createdBy)).
			Select(fieldvalueuser.FieldNodeID).Ints(q.ctx)
		if err != nil {
			return false, fmt.Errorf("nodes naming the profile's user: %w", err)
		}
		return q.only(f, ids)
	case f.FieldStorageID != nil:
		// Reference field: nodes the current node references via it (and, for
		// RelatedToCurrent, nodes referencing it via the same field).
		if q.nodeID <= 0 {
			return false, nil
		}
		ids, err := referenceRelatedNodeIDs(q.ctx, q.client, *f.FieldStorageID, q.nodeID, f.Operator == entity.ViewFilterOperatorRelatedToCurrent)
		if err != nil {
			return false, err
		}
		return q.only(f, ids)
	}
	switch f.BuiltinField {
	case entity.ViewFilterBuiltinHasCurrentTerm:
		if q.termID <= 0 {
			return false, nil
		}
		q.predicates = append(q.predicates, node.HasTaxonomyIndicesWith(taxonomyindex.Tid(q.termID)))
	case entity.ViewFilterBuiltinSharesTermsWithCurrentNode:
		if q.nodeID <= 0 {
			return false, nil
		}
		vid, _ := strconv.Atoi(strings.TrimSpace(f.Value))
		tids, err := currentNodeTermIDs(q.ctx, q.client, q.nodeID, vid)
		if err != nil || len(tids) == 0 {
			return false, err
		}
		q.predicates = append(q.predicates, node.HasTaxonomyIndicesWith(taxonomyindex.TidIn(tids...)))
	case entity.ViewFilterBuiltinSimilarToCurrentNode:
		if q.nodeID <= 0 {
			return false, nil
		}
		// Ranked live (similarToNode), every candidate, not
		// params.PageSize: the IN predicate only bounds the candidates
		// for the View's other filters; RunView orders them by the
		// ranking and slices the page after it.
		vid, _ := strconv.Atoi(strings.TrimSpace(f.Value))
		ranking, err := similarToNode(q.ctx, q.client, q.nodeID, vid)
		if err != nil || len(ranking) == 0 {
			return false, err
		}
		q.predicates = append(q.predicates, node.IDIn(ranking.ids()...))
		q.similar = ranking
	default:
		if q.createdBy <= 0 {
			return false, nil
		}
		q.predicates = append(q.predicates, node.CreatedByEQ(q.createdBy))
	}
	return true, nil
}

// add adds a filter with a value (fixed, or exposed: from the query
// parameters - none, no constraint), a date state, or empty/not_empty
// (no value: always active, see ViewFilterOperatorEmpty).
func (q *filterQuery) add(f entity.ViewFilterDef) (bool, error) {
	isPresenceOperator := f.Operator == entity.ViewFilterOperatorEmpty || f.Operator == entity.ViewFilterOperatorNotEmpty
	var value string
	if !isPresenceOperator && !f.Operator.IsDateState() {
		v, ok := resolveViewFilterValue(f, q.queryParams)
		if !ok {
			return true, nil
		}
		value = v
	}
	if f.FieldStorageID == nil {
		pred, err := builtinFilterPredicate(f.BuiltinField, f.Operator, value)
		if err != nil {
			return false, err
		}
		q.predicates = append(q.predicates, pred)
		return true, nil
	}
	fs, ok := q.fieldStorages[*f.FieldStorageID]
	if !ok {
		return false, fmt.Errorf("RunView: missing FieldStorageDef for field_storage_id %d", *f.FieldStorageID)
	}
	switch {
	case f.Operator.IsDateState():
		if fs.FieldType != entity.FieldTypeDate {
			return false, fmt.Errorf("RunView: %s on a %s field", f.Operator, fs.FieldType)
		}
		ids, err := nodeIDsByDateState(q.ctx, q.client, fs, f.Operator, time.Now(), sitetime.Location())
		return q.require(f, ids, err)
	case isPresenceOperator:
		ids, err := nodeIDsWithFieldPresence(q.ctx, q.client, fs)
		if f.Operator == entity.ViewFilterOperatorEmpty {
			return q.exclude(f, ids, err)
		}
		return q.require(f, ids, err)
	case f.Operator == entity.ViewFilterOperatorNotIn:
		// Exclusion (see ViewFilterOperatorNotIn): the set "in" would
		// match, excluded.
		ids, err := nodeIDsForGenericFilter(q.ctx, q.client, fs, entity.ViewFilterOperatorIn, value, false)
		return q.exclude(f, ids, err)
	}
	ids, err := nodeIDsForGenericFilter(q.ctx, q.client, fs, f.Operator, value, f.ValueIsRelative)
	return q.require(f, ids, err)
}

// only - the nodes must be among ids (viaParents), a predicate of its
// own; none - impossible.
func (q *filterQuery) only(f entity.ViewFilterDef, ids []int) (bool, error) {
	ids, err := viaParents(q.ctx, q.client, f, ids)
	if err != nil || len(ids) == 0 {
		return false, err
	}
	q.predicates = append(q.predicates, node.IDIn(ids...))
	return true, nil
}

// require - the nodes must be among ids (viaParents; err - reading them
// failed), intersected with the earlier filters' constraint; none left -
// impossible.
func (q *filterQuery) require(f entity.ViewFilterDef, ids []int, err error) (bool, error) {
	if err != nil {
		return false, err
	}
	if ids, err = viaParents(q.ctx, q.client, f, ids); err != nil {
		return false, err
	}
	if q.haveIDs {
		q.ids = intersectInts(q.ids, ids)
	} else {
		q.ids, q.haveIDs = ids, true
	}
	return len(q.ids) > 0, nil
}

// exclude - the nodes must not be among ids (viaParents; err - reading
// them failed): a predicate of its own, since the positive constraint
// only intersects.
func (q *filterQuery) exclude(f entity.ViewFilterDef, ids []int, err error) (bool, error) {
	if err != nil {
		return false, err
	}
	if ids, err = viaParents(q.ctx, q.client, f, ids); err != nil {
		return false, err
	}
	q.predicates = append(q.predicates, node.IDNotIn(ids...))
	return true, nil
}

// viaParents - the nodes a filter's ids stand for: ids themselves, or -
// a filter in a blocks field's blocks (ViaFieldStorageID) - the nodes
// holding those blocks in that field, each once. So "empty" and "not in"
// mean "no block of the node has it".
func viaParents(ctx context.Context, client *generated.Client, f entity.ViewFilterDef, ids []int) ([]int, error) {
	if f.ViaFieldStorageID == nil || len(ids) == 0 {
		return ids, nil
	}
	parents, err := client.FieldValueBlock.Query().
		Where(fieldvalueblock.FieldIDEQ(*f.ViaFieldStorageID), fieldvalueblock.ItemIDIn(ids...)).
		Select(fieldvalueblock.FieldNodeID).Ints(ctx)
	if err != nil {
		return nil, fmt.Errorf("the nodes of blocks of field %d: %w", *f.ViaFieldStorageID, err)
	}
	return dedupeInts(parents), nil
}

// matchingNodeIDs resolves filters to the full set of matching node IDs,
// without pagination or sorting - used by GlossaryForView.
func matchingNodeIDs(ctx context.Context, client *generated.Client, filters entity.ViewFilterDefs, moduleFilters []entity.ViewRunModuleFilter, fieldStorages map[int]entity.FieldStorageDef, queryParams map[string]string) ([]int, error) {
	// A glossary query has no "currently viewed node", so contextual filters
	// degrade to "impossible" rather than leaking unfiltered content.
	// The similar ranking is irrelevant here.
	predicates, impossible, _, err := buildFilterPredicates(ctx, client, filters, fieldStorages, queryParams, 0, 0, 0)
	if err != nil {
		return nil, err
	}
	if impossible {
		return nil, nil
	}
	predicates = database.WithContentListFilter(ctx, predicates)
	predicates, ok, err := withModuleFilters(ctx, client, predicates, moduleFilters, queryParams)
	if err != nil || !ok {
		return nil, err
	}
	return client.Node.Query().Where(predicates...).IDs(ctx)
}

// resolveViewFilterValue returns the filter's effective value for the
// current request - Value for a fixed filter, QueryParams[ExposedParam]
// for an exposed one - and false when there's nothing to apply (an empty
// optional exposed filter imposes no constraint).
func resolveViewFilterValue(f entity.ViewFilterDef, queryParams map[string]string) (string, bool) {
	if f.Exposed {
		v, ok := queryParams[f.ExposedParam]
		if !ok || v == "" {
			return "", false
		}
		return v, true
	}
	if f.Value == "" {
		return "", false
	}
	return f.Value, true
}

func builtinFilterPredicate(builtinField entity.ViewFilterBuiltinField, operator entity.ViewFilterOperator, value string) (predicate.Node, error) {
	switch builtinField {
	case entity.ViewFilterBuiltinStatus:
		n, err := strconv.Atoi(value)
		if err != nil {
			return nil, fmt.Errorf("RunView: invalid status filter value %q: %w", value, err)
		}
		return node.StatusEQ(n), nil
	case entity.ViewFilterBuiltinOnModeration:
		n, err := strconv.Atoi(value)
		if err != nil {
			return nil, fmt.Errorf("RunView: invalid on_moderation filter value %q: %w", value, err)
		}
		return node.OnModerationEQ(n), nil
	case entity.ViewFilterBuiltinPromote, entity.ViewFilterBuiltinSticky:
		on, err := strconv.ParseBool(strings.TrimSpace(value))
		if err != nil {
			return nil, fmt.Errorf("RunView: invalid %s filter value %q: %w", builtinField, value, err)
		}
		if builtinField == entity.ViewFilterBuiltinPromote {
			return node.Promote(on), nil
		}
		return node.Sticky(on), nil
	case entity.ViewFilterBuiltinCreatedBy:
		if operator == entity.ViewFilterOperatorIn {
			ids, err := parseIntList(value)
			if err != nil {
				return nil, fmt.Errorf("RunView: invalid created_by filter value %q: %w", value, err)
			}
			return node.CreatedByIn(ids...), nil
		}
		n, err := strconv.Atoi(value)
		if err != nil {
			return nil, fmt.Errorf("RunView: invalid created_by filter value %q: %w", value, err)
		}
		return node.CreatedByEQ(n), nil
	case entity.ViewFilterBuiltinTitle:
		switch operator {
		case entity.ViewFilterOperatorContains:
			return node.TitleContainsFold(value), nil
		case entity.ViewFilterOperatorStartsWith:
			// ent generates no TitleHasPrefixFold, so build it from the same
			// primitive TitleContainsFold uses - keeps the glossary letter match
			// case-insensitive regardless of the column collation.
			return predicate.Node(entsql.FieldHasPrefixFold(node.FieldTitle, value)), nil
		default:
			return node.TitleEQ(value), nil
		}
	case entity.ViewFilterBuiltinContentType:
		// The only mechanism that scopes a View to content types (see
		// entity.ViewDef). Value is a ContentTypeDef.MachineName, or a
		// comma-separated list for "in".
		if operator == entity.ViewFilterOperatorIn {
			return node.TypeIn(parseStringList(value)...), nil
		}
		return node.TypeEQ(value), nil
	default:
		return nil, fmt.Errorf("RunView: unknown builtin field %q", builtinField)
	}
}

// nodeIDsForGenericFilter resolves one generic-field filter to the
// deduplicated set of matching node IDs (a multi-value field can match
// several rows per node).
func nodeIDsForGenericFilter(ctx context.Context, client *generated.Client, fs entity.FieldStorageDef, operator entity.ViewFilterOperator, value string, valueIsRelative bool) ([]int, error) {
	switch fs.FieldType {
	case entity.FieldTypeTaxonomy:
		termIDs, err := parseIntList(value)
		if err != nil {
			return nil, fmt.Errorf("RunView: invalid taxonomy filter value %q: %w", value, err)
		}
		if fs.Multiple() {
			rows, err := client.FieldValueTaxonomyMulti.Query().
				Where(fieldvaluetaxonomymulti.FieldIDEQ(fs.ID), fieldvaluetaxonomymulti.TermIDIn(termIDs...)).
				All(ctx)
			if err != nil {
				return nil, fmt.Errorf("nodeIDsForGenericFilter: %w", err)
			}
			ids := make([]int, len(rows))
			for i, row := range rows {
				ids[i] = row.NodeID
			}
			return dedupeInts(ids), nil
		}
		rows, err := client.FieldValueTaxonomy.Query().
			Where(fieldvaluetaxonomy.FieldIDEQ(fs.ID), fieldvaluetaxonomy.TermIDIn(termIDs...)).
			All(ctx)
		if err != nil {
			return nil, fmt.Errorf("nodeIDsForGenericFilter: %w", err)
		}
		ids := make([]int, len(rows))
		for i, row := range rows {
			ids[i] = row.NodeID
		}
		return dedupeInts(ids), nil

	case entity.FieldTypeText, entity.FieldTypeEmail, entity.FieldTypePhone:
		var pred predicate.FieldValueText
		switch operator {
		case entity.ViewFilterOperatorContains:
			pred = fieldvaluetext.ValueContainsFold(value)
		case entity.ViewFilterOperatorStartsWith:
			// ent generates no ValueHasPrefixFold - same manual predicate as the
			// case-insensitive title starts_with in builtinFilterPredicate.
			pred = predicate.FieldValueText(entsql.FieldHasPrefixFold(fieldvaluetext.FieldValue, value))
		default:
			pred = fieldvaluetext.ValueEQ(value)
		}
		rows, err := client.FieldValueText.Query().
			Where(fieldvaluetext.FieldIDEQ(fs.ID), pred).
			All(ctx)
		if err != nil {
			return nil, fmt.Errorf("nodeIDsForGenericFilter: %w", err)
		}
		ids := make([]int, len(rows))
		for i, row := range rows {
			ids[i] = row.NodeID
		}
		return ids, nil

	case entity.FieldTypeNumber:
		n, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return nil, fmt.Errorf("RunView: invalid number filter value %q: %w", value, err)
		}
		var pred predicate.FieldValueNumber
		switch operator {
		case entity.ViewFilterOperatorGte:
			pred = fieldvaluenumber.ValueGTE(n)
		case entity.ViewFilterOperatorLte:
			pred = fieldvaluenumber.ValueLTE(n)
		default:
			pred = fieldvaluenumber.ValueEQ(n)
		}
		rows, err := client.FieldValueNumber.Query().
			Where(fieldvaluenumber.FieldIDEQ(fs.ID), pred).
			All(ctx)
		if err != nil {
			return nil, fmt.Errorf("nodeIDsForGenericFilter: %w", err)
		}
		ids := make([]int, len(rows))
		for i, row := range rows {
			ids[i] = row.NodeID
		}
		return ids, nil

	case entity.FieldTypeDate:
		var t time.Time
		var err error
		if valueIsRelative {
			t, err = entity.ParseRelativeDate(value, time.Now())
		} else {
			t, err = time.Parse("2006-01-02", value)
		}
		if err != nil {
			return nil, fmt.Errorf("RunView: invalid date filter value %q: %w", value, err)
		}
		// The start compared; a date (not a relative instant) covers its
		// whole day, so a start with a time on it is "equal" and "not
		// later" too. A date alone is stored at UTC midnight, a start with
		// a time as its instant: the day of those is the site's.
		var pred predicate.FieldValueDate
		if valueIsRelative {
			pred = dateFrom(operator, t, true)
		} else {
			inSite := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, sitetime.Location())
			pred = fieldvaluedate.Or(
				fieldvaluedate.And(fieldvaluedate.WithTime(false), dateFrom(operator, t, false)),
				fieldvaluedate.And(fieldvaluedate.WithTime(true), dateFrom(operator, inSite, false)))
		}
		rows, err := client.FieldValueDate.Query().
			Where(fieldvaluedate.FieldIDEQ(fs.ID), pred).
			All(ctx)
		if err != nil {
			return nil, fmt.Errorf("nodeIDsForGenericFilter: %w", err)
		}
		ids := make([]int, len(rows))
		for i, row := range rows {
			ids[i] = row.NodeID
		}
		return ids, nil

	case entity.FieldTypeList:
		// eq - one option's value, in - any of several (comma-separated).
		values := parseStringList(value)
		var ids []int
		if fs.Multiple() {
			rows, err := client.FieldValueListMulti.Query().
				Where(fieldvaluelistmulti.FieldIDEQ(fs.ID), fieldvaluelistmulti.ValueIn(values...)).All(ctx)
			if err != nil {
				return nil, fmt.Errorf("nodeIDsForGenericFilter: %w", err)
			}
			for _, row := range rows {
				ids = append(ids, row.NodeID)
			}
		} else {
			rows, err := client.FieldValueList.Query().
				Where(fieldvaluelist.FieldIDEQ(fs.ID), fieldvaluelist.ValueIn(values...)).All(ctx)
			if err != nil {
				return nil, fmt.Errorf("nodeIDsForGenericFilter: %w", err)
			}
			for _, row := range rows {
				ids = append(ids, row.NodeID)
			}
		}
		return dedupeInts(ids), nil

	case entity.FieldTypeBool:
		b, err := strconv.ParseBool(value)
		if err != nil {
			return nil, fmt.Errorf("RunView: invalid bool filter value %q: %w", value, err)
		}
		rows, err := client.FieldValueBool.Query().
			Where(fieldvaluebool.FieldIDEQ(fs.ID), fieldvaluebool.ValueEQ(b)).
			All(ctx)
		if err != nil {
			return nil, fmt.Errorf("nodeIDsForGenericFilter: %w", err)
		}
		ids := make([]int, len(rows))
		for i, row := range rows {
			ids[i] = row.NodeID
		}
		return ids, nil

	default:
		return nil, fmt.Errorf("RunView: field type %q doesn't support view filters", fs.FieldType)
	}
}

// nodeIDsWithFieldPresence returns the node IDs that have any row for fs,
// regardless of value - backs the empty/not_empty operators. Same
// per-FieldType table dispatch as nodeIDsForGenericFilter.
func nodeIDsWithFieldPresence(ctx context.Context, client *generated.Client, fs entity.FieldStorageDef) ([]int, error) {
	switch fs.FieldType {
	case entity.FieldTypeTaxonomy, entity.FieldTypeText, entity.FieldTypeNumber, entity.FieldTypeEmail, entity.FieldTypePhone,
		entity.FieldTypeDate, entity.FieldTypeBool, entity.FieldTypeImage, entity.FieldTypeUser, entity.FieldTypeList:
	default:
		return nil, fmt.Errorf("RunView: field type %q doesn't support empty/not_empty", fs.FieldType)
	}
	ids, _, err := database.NodeIDsWithField(ctx, client, fs)
	if err != nil {
		return nil, err
	}
	return dedupeInts(ids), nil
}

func parseIntList(raw string) ([]int, error) {
	parts := strings.Split(raw, ",")
	ids := make([]int, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		n, err := strconv.Atoi(p)
		if err != nil {
			return nil, err
		}
		ids = append(ids, n)
	}
	return ids, nil
}

func parseStringList(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		out = append(out, p)
	}
	return out
}

// dedupeInts - ids without repeats, in their order.
func dedupeInts(ids []int) []int {
	seen := make(map[int]struct{}, len(ids))
	out := make([]int, 0, len(ids))
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func intersectInts(a, b []int) []int {
	set := make(map[int]struct{}, len(b))
	for _, id := range b {
		set[id] = struct{}{}
	}
	out := make([]int, 0, len(a))
	for _, id := range a {
		if _, ok := set[id]; ok {
			out = append(out, id)
		}
	}
	return out
}

// nodeIDsByDateState - the nodes whose date field fs is going on
// (ViewFilterOperatorOngoing), not ended or ended at now: a value
// without a time compared by today in zone (its last day - the end, else
// the start), one with a time by now (its end, else its start). Several
// dates: any going on or not ended; ended - none not ended.
func nodeIDsByDateState(ctx context.Context, client *generated.Client, fs entity.FieldStorageDef, op entity.ViewFilterOperator, now time.Time, zone *time.Location) ([]int, error) {
	today := entity.Today(now, zone)
	// last ≥ t: the end when there is one, else the start.
	lastFrom := func(t time.Time, strict bool) predicate.FieldValueDate {
		if strict {
			return fieldvaluedate.Or(
				fieldvaluedate.And(fieldvaluedate.EndValueNotNil(), fieldvaluedate.EndValueGT(t)),
				fieldvaluedate.And(fieldvaluedate.EndValueIsNil(), fieldvaluedate.ValueGT(t)))
		}
		return fieldvaluedate.Or(
			fieldvaluedate.And(fieldvaluedate.EndValueNotNil(), fieldvaluedate.EndValueGTE(t)),
			fieldvaluedate.And(fieldvaluedate.EndValueIsNil(), fieldvaluedate.ValueGTE(t)))
	}
	notEnded := fieldvaluedate.Or(
		fieldvaluedate.And(fieldvaluedate.WithTime(false), lastFrom(today, false)),
		fieldvaluedate.And(fieldvaluedate.WithTime(true), lastFrom(now, true)))
	cond := notEnded
	if op == entity.ViewFilterOperatorOngoing {
		cond = fieldvaluedate.And(notEnded, fieldvaluedate.Or(
			fieldvaluedate.And(fieldvaluedate.WithTime(false), fieldvaluedate.ValueLTE(today)),
			fieldvaluedate.And(fieldvaluedate.WithTime(true), fieldvaluedate.ValueLTE(now))))
	}
	ids, err := client.FieldValueDate.Query().Where(fieldvaluedate.FieldIDEQ(fs.ID), cond).Select(fieldvaluedate.FieldNodeID).Ints(ctx)
	if err != nil {
		return nil, fmt.Errorf("nodeIDsByDateState: %w", err)
	}
	if op != entity.ViewFilterOperatorEnded {
		return uniqueInts(ids), nil
	}
	all, err := client.FieldValueDate.Query().Where(fieldvaluedate.FieldIDEQ(fs.ID)).Select(fieldvaluedate.FieldNodeID).Ints(ctx)
	if err != nil {
		return nil, fmt.Errorf("nodeIDsByDateState: %w", err)
	}
	open := make(map[int]bool, len(ids))
	for _, id := range ids {
		open[id] = true
	}
	ended := all[:0]
	for _, id := range all {
		if !open[id] {
			ended = append(ended, id)
		}
	}
	return uniqueInts(ended), nil
}

// uniqueInts - ids, each once, sorted.
func uniqueInts(ids []int) []int {
	slices.Sort(ids)
	return slices.Compact(ids)
}

// dateFrom - a date filter's operator on the start: from day (an instant -
// instant: compared as it is; else the start of a day, which it covers
// to its end).
func dateFrom(operator entity.ViewFilterOperator, day time.Time, instant bool) predicate.FieldValueDate {
	next := day.AddDate(0, 0, 1)
	switch {
	case operator == entity.ViewFilterOperatorGte:
		return fieldvaluedate.ValueGTE(day)
	case operator == entity.ViewFilterOperatorLte && instant:
		return fieldvaluedate.ValueLTE(day)
	case operator == entity.ViewFilterOperatorLte:
		return fieldvaluedate.ValueLT(next)
	case instant:
		return fieldvaluedate.ValueEQ(day)
	default:
		return fieldvaluedate.And(fieldvaluedate.ValueGTE(day), fieldvaluedate.ValueLT(next))
	}
}
