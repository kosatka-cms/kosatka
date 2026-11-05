package database

import (
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/database/dao"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/fieldvaluetaxonomy"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/fieldvaluetaxonomymulti"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/node"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/predicate"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/taxonomyindex"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/taxonomytermdata"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/taxonomyvocabulary"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/schema"
)

// NewTaxonomyRepository - the vocabulary and term repository on client.
func NewTaxonomyRepository(
	client *generated.Client,
) *TaxonomyRepository {
	return &TaxonomyRepository{
		client: client,
	}
}

// TaxonomyRepository provides database access to vocabularies and terms.
type TaxonomyRepository struct {
	client *generated.Client
}

// NodeTermsByID - the node's terms (each with its vocabulary) from the
// terms index, in the order they were indexed.
func (r TaxonomyRepository) NodeTermsByID(ctx context.Context, nID int) (entity.TaxonomyTerms, error) {
	client := GetClient(ctx, r.client)
	res, err := client.TaxonomyIndex.Query().Where(
		taxonomyindex.Nid(nID),
	).
		WithTaxonomyTermData(
			func(q *generated.TaxonomyTermDataQuery) {
				q.WithVocabulary()
			},
		).
		Order(generated.Asc(taxonomyindex.FieldID)).
		All(ctx)

	if err != nil {
		return nil, fmt.Errorf("TaxonomyRepository.NodeTermsByID: %w", err)
	}

	termsEntity := dao.ToTaxonomyTermsEntity(res)

	return termsEntity, nil
}

// NodeTermsByIDs is NodeTermsByID batched over many node IDs in one
// query - for list pages that need per-item terms.
func (r TaxonomyRepository) NodeTermsByIDs(ctx context.Context, nodeIDs []int) (map[int]entity.TaxonomyTerms, error) {
	termsByNode := make(map[int]entity.TaxonomyTerms)
	if len(nodeIDs) == 0 {
		return termsByNode, nil
	}
	client := GetClient(ctx, r.client)
	res, err := client.TaxonomyIndex.Query().
		Where(taxonomyindex.NidIn(nodeIDs...)).
		WithTaxonomyTermData(func(q *generated.TaxonomyTermDataQuery) {
			q.WithVocabulary()
		}).
		Order(generated.Asc(taxonomyindex.FieldID)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("TaxonomyRepository.NodeTermsByIDs: %w", err)
	}
	for _, idx := range res {
		termsByNode[idx.Nid] = append(termsByNode[idx.Nid], dao.ToTaxonomyTermsEntity([]*generated.TaxonomyIndex{idx})...)
	}
	return termsByNode, nil
}

// TermBySlug - the term with slug, with its vocabulary; ErrEntityNotFound
// if there is none.
func (r TaxonomyRepository) TermBySlug(ctx context.Context, slug string) (*entity.TaxonomyTerm, error) {
	client := GetClient(ctx, r.client)
	res, err := client.TaxonomyTermData.Query().Where(
		taxonomytermdata.Slug(slug),
	).
		WithVocabulary().
		First(ctx)

	if err != nil {
		return nil, notFoundAs(err, "taxonomy term with slug %s", slug)
	}

	termEntity := dao.ToTaxonomyTermEntity(res)

	return termEntity, nil
}

// CountNodesByTermID - see entity.TaxonomyRepository.CountNodesByTermID.
func (r TaxonomyRepository) CountNodesByTermID(ctx context.Context, termID int) (int, error) {
	client := GetClient(ctx, r.client)
	nodes := map[int]bool{}
	indexed, err := client.TaxonomyIndex.Query().Where(taxonomyindex.Tid(termID)).Select(taxonomyindex.FieldNid).Ints(ctx)
	if err != nil {
		return 0, fmt.Errorf("TaxonomyRepository.CountNodesByTermID: %w", err)
	}
	single, err := client.FieldValueTaxonomy.Query().Where(fieldvaluetaxonomy.TermID(termID)).Select(fieldvaluetaxonomy.FieldNodeID).Ints(ctx)
	if err != nil {
		return 0, fmt.Errorf("TaxonomyRepository.CountNodesByTermID: %w", err)
	}
	multi, err := client.FieldValueTaxonomyMulti.Query().Where(fieldvaluetaxonomymulti.TermID(termID)).Select(fieldvaluetaxonomymulti.FieldNodeID).Ints(ctx)
	if err != nil {
		return 0, fmt.Errorf("TaxonomyRepository.CountNodesByTermID: %w", err)
	}
	for _, list := range [][]int{indexed, single, multi} {
		for _, id := range list {
			nodes[id] = true
		}
	}
	return len(nodes), nil
}

// TermNodeCounts - the published content per term that ctx's visitor
// may view, out of the trash (schema.LiveNodes: the subquery isn't a node
// query the interceptor sees).
func (r TaxonomyRepository) TermNodeCounts(ctx context.Context, vocabularyID int) (map[int]int, error) {
	visible := append([]predicate.Node{node.Status(1), node.OnModeration(0)}, schema.LiveNodes(ctx)...)
	query := GetClient(ctx, r.client).TaxonomyIndex.Query().
		Where(taxonomyindex.HasNodeWith(visible...))
	if vocabularyID > 0 {
		query = query.Where(taxonomyindex.HasTaxonomyTermDataWith(taxonomytermdata.Vid(vocabularyID)))
	}
	var rows []struct {
		Tid   int `json:"tid"`
		Count int `json:"count"`
	}
	if err := query.GroupBy(taxonomyindex.FieldTid).Aggregate(generated.Count()).Scan(ctx, &rows); err != nil {
		return nil, fmt.Errorf("counting materials by term (vocabulary %d): %w", vocabularyID, err)
	}
	counts := make(map[int]int, len(rows))
	for _, row := range rows {
		counts[row.Tid] = row.Count
	}
	return counts, nil
}

// SearchTermsByName - at most limit terms of the vocabulary whose name
// contains query (letter case aside), by weight, then name.
func (r TaxonomyRepository) SearchTermsByName(ctx context.Context, vocabularyID int, query string, limit int) (entity.TaxonomyTerms, error) {
	client := GetClient(ctx, r.client)
	res, err := client.TaxonomyTermData.Query().
		Where(
			taxonomytermdata.Vid(vocabularyID),
			taxonomytermdata.NameContainsFold(query),
		).
		WithVocabulary().
		Limit(limit).
		Order(generated.Asc(taxonomytermdata.FieldWeight), generated.Asc(taxonomytermdata.FieldName)).
		All(ctx)

	if err != nil {
		return nil, fmt.Errorf("TaxonomyRepository.SearchTermsByName: %w", err)
	}

	terms := make(entity.TaxonomyTerms, len(res))
	for i, t := range res {
		terms[i] = *dao.ToTaxonomyTermEntity(t)
	}

	return terms, nil
}

// TermsByVocabularyID - every term of the vocabulary, all levels in one
// flat list, by weight, then name.
func (r TaxonomyRepository) TermsByVocabularyID(ctx context.Context, vocabularyID int) (entity.TaxonomyTerms, error) {
	client := GetClient(ctx, r.client)
	res, err := client.TaxonomyTermData.Query().
		Where(
			taxonomytermdata.Vid(vocabularyID),
		).
		WithVocabulary().
		Order(generated.Asc(taxonomytermdata.FieldWeight), generated.Asc(taxonomytermdata.FieldName)).
		All(ctx)

	if err != nil {
		return nil, fmt.Errorf("TaxonomyRepository.TermsByVocabularyID: %w", err)
	}

	terms := make(entity.TaxonomyTerms, len(res))
	for i, t := range res {
		terms[i] = *dao.ToTaxonomyTermEntity(t)
	}

	return terms, nil
}

// CreateTerm adds a top-level term to the vocabulary and returns it with
// its vocabulary.
func (r TaxonomyRepository) CreateTerm(ctx context.Context, vocabularyID int, name, slug string, createdBy int) (*entity.TaxonomyTerm, error) {
	client := GetClient(ctx, r.client)
	now := time.Now()
	term, err := client.TaxonomyTermData.Create().
		SetVocabularyID(vocabularyID).
		SetName(name).
		SetSlug(slug).
		SetDescription("").
		SetWeight(0).
		SetNillableParentID(nil). // top-level term
		SetCreatedBy(createdBy).
		SetCreatedAt(now).
		SetUpdatedAt(now).
		Save(ctx)

	if err != nil {
		return nil, err
	}

	// Reload with vocabulary
	term, err = client.TaxonomyTermData.Query().
		Where(taxonomytermdata.ID(term.ID)).
		WithVocabulary().
		Only(ctx)

	if err != nil {
		return nil, fmt.Errorf("TaxonomyRepository.CreateTerm: %w", err)
	}

	termEntity := dao.ToTaxonomyTermEntity(term)
	return termEntity, nil
}

// positiveIDs - the IDs above 0 of ids, without repeats, in their order.
func positiveIDs(ids []int) []int {
	return dedupeInts(slices.DeleteFunc(slices.Clone(ids), func(id int) bool { return id <= 0 }))
}

// UpdateNodeTerms replaces the node's terms index with termIDs, in one
// transaction (the caller's when ctx carries one): a failure keeps the old
// index.
func (r TaxonomyRepository) UpdateNodeTerms(ctx context.Context, nodeID int, termIDs []int) error {
	termIDs = positiveIDs(termIDs)
	return NewUnitOfWork(r.client).Do(ctx, func(ctx context.Context) error {
		client := GetClient(ctx, r.client)
		if _, err := client.TaxonomyIndex.Delete().Where(taxonomyindex.Nid(nodeID)).Exec(ctx); err != nil {
			return fmt.Errorf("deleting the terms index of node %d: %w", nodeID, err)
		}
		now := time.Now()
		for _, termID := range termIDs {
			if _, err := client.TaxonomyIndex.Create().SetNid(nodeID).SetTid(termID).SetCreatedAt(now).Save(ctx); err != nil {
				return fmt.Errorf("indexing term %d of node %d: %w", termID, nodeID, err)
			}
		}
		return nil
	})
}

// FieldTermIDsForNode - see entity.TaxonomyRepository.FieldTermIDsForNode.
func (r TaxonomyRepository) FieldTermIDsForNode(ctx context.Context, nodeID int) ([]int, error) {
	client := GetClient(ctx, r.client)
	single, err := client.FieldValueTaxonomy.Query().Where(fieldvaluetaxonomy.NodeID(nodeID)).All(ctx)
	if err != nil {
		return nil, fmt.Errorf("TaxonomyRepository.FieldTermIDsForNode: %w", err)
	}
	multi, err := client.FieldValueTaxonomyMulti.Query().Where(fieldvaluetaxonomymulti.NodeID(nodeID)).
		Order(generated.Asc(fieldvaluetaxonomymulti.FieldID)).All(ctx)
	if err != nil {
		return nil, fmt.Errorf("TaxonomyRepository.FieldTermIDsForNode: %w", err)
	}
	ids := make([]int, 0, len(single)+len(multi))
	for _, v := range single {
		ids = append(ids, v.TermID)
	}
	for _, v := range multi {
		ids = append(ids, v.TermID)
	}
	return positiveIDs(ids), nil
}

// Vocabularies - every vocabulary, by weight, then name.
func (r TaxonomyRepository) Vocabularies(ctx context.Context) (entity.TaxonomyVocabularies, error) {
	client := GetClient(ctx, r.client)
	res, err := client.TaxonomyVocabulary.Query().
		Order(generated.Asc(taxonomyvocabulary.FieldWeight), generated.Asc(taxonomyvocabulary.FieldName)).
		All(ctx)

	if err != nil {
		return nil, fmt.Errorf("TaxonomyRepository.Vocabularies: %w", err)
	}

	vocabularies := make(entity.TaxonomyVocabularies, len(res))
	for i, v := range res {
		vocabularies[i] = dao.ToTaxonomyVocabularyEntity(v)
	}

	return vocabularies, nil
}

// VocabularyByID - the vocabulary with id; ErrEntityNotFound if there is
// none.
func (r TaxonomyRepository) VocabularyByID(ctx context.Context, id int) (*entity.TaxonomyVocabulary, error) {
	client := GetClient(ctx, r.client)
	res, err := client.TaxonomyVocabulary.Get(ctx, id)

	if err != nil {
		return nil, notFoundAs(err, "taxonomy vocabulary with id %d", id)
	}

	vocabularyEntity := dao.ToTaxonomyVocabularyEntity(res)
	return &vocabularyEntity, nil
}

// CreateVocabulary adds a vocabulary and returns it with its ID.
func (r TaxonomyRepository) CreateVocabulary(ctx context.Context, name, slug, termPathPrefix, desc string, publicTermPages bool, weight, createdBy int) (*entity.TaxonomyVocabulary, error) {
	client := GetClient(ctx, r.client)
	now := time.Now()
	v, err := client.TaxonomyVocabulary.Create().
		SetName(name).
		SetSlug(slug).
		SetTermPathPrefix(termPathPrefix).
		SetPublicTermPages(publicTermPages).
		SetDescription(desc).
		SetWeight(weight).
		SetCreatedBy(createdBy).
		SetCreatedAt(now).
		SetUpdatedAt(now).
		Save(ctx)

	if err != nil {
		return nil, err
	}

	vocabularyEntity := dao.ToTaxonomyVocabularyEntity(v)

	return &vocabularyEntity, nil
}

// UpdateVocabulary saves the vocabulary's name, slug, term path prefix,
// description, public term pages and weight, and returns it;
// ErrEntityNotFound if there is none.
func (r TaxonomyRepository) UpdateVocabulary(ctx context.Context, id int, name, slug, termPathPrefix, desc string, publicTermPages bool, weight int) (*entity.TaxonomyVocabulary, error) {
	client := GetClient(ctx, r.client)
	v, err := client.TaxonomyVocabulary.UpdateOneID(id).
		SetName(name).
		SetSlug(slug).
		SetTermPathPrefix(termPathPrefix).
		SetPublicTermPages(publicTermPages).
		SetDescription(desc).
		SetWeight(weight).
		SetUpdatedAt(time.Now()).
		Save(ctx)

	if err != nil {
		return nil, notFoundAs(err, "taxonomy vocabulary with id %d", id)
	}

	vocabularyEntity := dao.ToTaxonomyVocabularyEntity(v)

	return &vocabularyEntity, nil
}

// UpdateVocabularySortOrder updates only a vocabulary's Weight (the
// drag-and-drop reordering), never its Name/Slug/Description.
func (r TaxonomyRepository) UpdateVocabularySortOrder(ctx context.Context, id int, weight int) error {
	client := GetClient(ctx, r.client)
	err := client.TaxonomyVocabulary.UpdateOneID(id).
		SetWeight(weight).
		SetUpdatedAt(time.Now()).
		Exec(ctx)

	if err != nil {
		return notFoundAs(err, "taxonomy vocabulary with id %d", id)
	}
	return nil
}

// DeleteVocabulary deletes the vocabulary with id; ErrEntityNotFound if
// there is none.
func (r TaxonomyRepository) DeleteVocabulary(ctx context.Context, id int) error {
	client := GetClient(ctx, r.client)
	err := client.TaxonomyVocabulary.DeleteOneID(id).Exec(ctx)

	if err != nil {
		if generated.IsNotFound(err) {
			return fmt.Errorf("%w: taxonomy vocabulary with id %d", entity.ErrEntityNotFound, id)
		}
		return fmt.Errorf("error deleting taxonomy vocabulary: %w", err)
	}

	return nil
}

// TermByID - the term with id, with its vocabulary; ErrEntityNotFound if
// there is none.
func (r TaxonomyRepository) TermByID(ctx context.Context, id int) (*entity.TaxonomyTerm, error) {
	client := GetClient(ctx, r.client)
	res, err := client.TaxonomyTermData.Query().
		Where(taxonomytermdata.ID(id)).
		WithVocabulary().
		Only(ctx)

	if err != nil {
		return nil, notFoundAs(err, "taxonomy term with id %d", id)
	}

	return dao.ToTaxonomyTermEntity(res), nil
}

// TermsByIDs is TermByID batched over many term IDs in one query -
// see entity.TaxonomyRepository's doc comment for why.
func (r TaxonomyRepository) TermsByIDs(ctx context.Context, ids []int) (map[int]entity.TaxonomyTerm, error) {
	termsByID := make(map[int]entity.TaxonomyTerm, len(ids))
	if len(ids) == 0 {
		return termsByID, nil
	}
	client := GetClient(ctx, r.client)
	res, err := client.TaxonomyTermData.Query().
		Where(taxonomytermdata.IDIn(ids...)).
		WithVocabulary().
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("TaxonomyRepository.TermsByIDs: %w", err)
	}
	for _, t := range res {
		term := dao.ToTaxonomyTermEntity(t)
		termsByID[term.ID] = *term
	}
	return termsByID, nil
}

// CreateTermFull adds a term with all its settings (parentID 0 - a
// top-level one) and returns it with its vocabulary.
func (r TaxonomyRepository) CreateTermFull(ctx context.Context, vocabularyID int, name, slug, desc string, weight, parentID, createdBy int) (*entity.TaxonomyTerm, error) {
	client := GetClient(ctx, r.client)
	now := time.Now()
	create := client.TaxonomyTermData.Create().
		SetVocabularyID(vocabularyID).
		SetName(name).
		SetSlug(slug).
		SetDescription(desc).
		SetWeight(weight).
		SetCreatedBy(createdBy).
		SetCreatedAt(now).
		SetUpdatedAt(now)

	if parentID > 0 {
		create = create.SetParentID(parentID)
	}

	term, err := create.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("TaxonomyRepository.CreateTermFull: %w", err)
	}

	// Reload with vocabulary
	term, err = client.TaxonomyTermData.Query().
		Where(taxonomytermdata.ID(term.ID)).
		WithVocabulary().
		Only(ctx)

	if err != nil {
		return nil, fmt.Errorf("TaxonomyRepository.CreateTermFull: %w", err)
	}

	return dao.ToTaxonomyTermEntity(term), nil
}

// UpdateTermPageTitle sets only the term's page title.
func (r TaxonomyRepository) UpdateTermPageTitle(ctx context.Context, id int, pageTitle string) error {
	return GetClient(ctx, r.client).TaxonomyTermData.UpdateOneID(id).
		SetPageTitle(pageTitle).
		SetUpdatedAt(time.Now()).
		Exec(ctx)
}

// UpdateTerm saves the term's name, slug, description, weight and parent
// (0 - none) and returns it with its vocabulary; ErrEntityNotFound if
// there is none.
func (r TaxonomyRepository) UpdateTerm(ctx context.Context, id int, name, slug, desc string, weight, parentID int) (*entity.TaxonomyTerm, error) {
	client := GetClient(ctx, r.client)
	update := client.TaxonomyTermData.UpdateOneID(id).
		SetName(name).
		SetSlug(slug).
		SetDescription(desc).
		SetWeight(weight).
		SetUpdatedAt(time.Now())

	if parentID > 0 {
		update = update.SetParentID(parentID)
	} else {
		update = update.ClearParentID()
	}

	term, err := update.Save(ctx)
	if err != nil {
		return nil, notFoundAs(err, "taxonomy term with id %d", id)
	}

	// Reload with vocabulary
	term, err = client.TaxonomyTermData.Query().
		Where(taxonomytermdata.ID(term.ID)).
		WithVocabulary().
		Only(ctx)

	if err != nil {
		return nil, fmt.Errorf("TaxonomyRepository.UpdateTerm: %w", err)
	}

	return dao.ToTaxonomyTermEntity(term), nil
}

// UpdateTermSortOrder updates only a term's Weight/ParentID (a bulk
// reorder), never its Name/Slug/Description.
func (r TaxonomyRepository) UpdateTermSortOrder(ctx context.Context, id int, weight, parentID int) error {
	client := GetClient(ctx, r.client)
	update := client.TaxonomyTermData.UpdateOneID(id).
		SetWeight(weight).
		SetUpdatedAt(time.Now())

	if parentID > 0 {
		update = update.SetParentID(parentID)
	} else {
		update = update.ClearParentID()
	}

	return notFoundAs(update.Exec(ctx), "taxonomy term with id %d", id)
}

// DeleteTerm deletes the term with id; ErrEntityNotFound if there is none.
func (r TaxonomyRepository) DeleteTerm(ctx context.Context, id int) error {
	client := GetClient(ctx, r.client)
	err := client.TaxonomyTermData.DeleteOneID(id).Exec(ctx)

	if err != nil {
		if generated.IsNotFound(err) {
			return fmt.Errorf("%w: taxonomy term with id %d", entity.ErrEntityNotFound, id)
		}
		return fmt.Errorf("error deleting taxonomy term: %w", err)
	}

	return nil
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
