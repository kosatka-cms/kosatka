package database

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"sync"

	"entgo.io/ent/dialect/sql"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/imageurl"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/database/dao"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/contenttypedef"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/fieldinstancedef"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/fieldstoragedef"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/fieldvaluefile"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/fieldvaluefilemulti"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/fieldvaluetext"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/file"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/node"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/predicate"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/searchword"
	"github.com/kosatka-cms/kosatka/internal/lib/searchtext"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
	"github.com/kosatka-cms/kosatka/pkg/panicsafe"
)

// SearchRepository runs the live search over the searched node types
// (entity.SearchTypeDef).
type SearchRepository struct {
	client *generated.Client
}

// NewSearchRepository creates a new SearchRepository.
func NewSearchRepository(client *generated.Client) *SearchRepository {
	return &SearchRepository{client: client}
}

func nodeWithEdges(nq *generated.NodeQuery) {
	nq.WithTaxonomyIndices(func(tiq *generated.TaxonomyIndexQuery) {
		tiq.WithTaxonomyTermData(func(ttq *generated.TaxonomyTermDataQuery) {
			ttq.WithVocabulary()
		})
	})
}

// imageFieldDefault - the default image (FieldInstanceDef.DefaultImageID)
// of the image field machineName on the node type; an empty File without
// one.
func imageFieldDefault(ctx context.Context, client *generated.Client, nodeType, machineName string) (entity.File, error) {
	if machineName == "" {
		return entity.File{}, nil
	}
	fi, err := client.FieldInstanceDef.Query().
		Where(
			fieldinstancedef.HasContentTypeWith(contenttypedef.MachineName(nodeType)),
			fieldinstancedef.HasFieldStorageWith(fieldstoragedef.MachineName(machineName)),
			fieldinstancedef.DefaultImageIDNotNil(),
		).
		WithDefaultImage().
		First(ctx)
	if generated.IsNotFound(err) {
		return entity.File{}, nil
	}
	if err != nil {
		return entity.File{}, fmt.Errorf("loading the default image of %s.%s: %w", nodeType, machineName, err)
	}
	if fi.Edges.DefaultImage == nil {
		return entity.File{}, nil
	}
	return dao.ToFileEntity(fi.Edges.DefaultImage), nil
}

// fieldStorageID - the ID of the field storage machineName; 0 - none.
func fieldStorageID(ctx context.Context, client *generated.Client, machineName string) (int, error) {
	storage, err := client.FieldStorageDef.Query().Where(fieldstoragedef.MachineName(machineName)).Only(ctx)
	if generated.IsNotFound(err) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("loading the field storage %q: %w", machineName, err)
	}
	return storage.ID, nil
}

// imageFieldValues - value of the image field storage machineName for each
// of nodeIDs (field engine, field_value_file; a field with several images -
// its first, field_value_file_multi); nodes without it are absent.
func imageFieldValues(ctx context.Context, client *generated.Client, machineName string, nodeIDs []int) (map[int]entity.File, error) {
	result := make(map[int]entity.File)
	if len(nodeIDs) == 0 || machineName == "" {
		return result, nil
	}
	storageID, err := fieldStorageID(ctx, client, machineName)
	if err != nil || storageID == 0 {
		return result, err
	}
	rows, err := client.FieldValueFile.Query().
		Where(fieldvaluefile.FieldIDEQ(storageID), fieldvaluefile.NodeIDIn(nodeIDs...)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("loading the values of %q: %w", machineName, err)
	}
	multi, err := client.FieldValueFileMulti.Query().
		Where(fieldvaluefilemulti.FieldIDEQ(storageID), fieldvaluefilemulti.NodeIDIn(nodeIDs...)).
		Order(generated.Asc(fieldvaluefilemulti.FieldDelta), generated.Asc(fieldvaluefilemulti.FieldID)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("loading the values of %q: %w", machineName, err)
	}
	fileOfNode := make(map[int]int, len(rows)+len(multi))
	for _, row := range rows {
		fileOfNode[row.NodeID] = row.FileID
	}
	for _, row := range multi {
		if _, ok := fileOfNode[row.NodeID]; !ok {
			fileOfNode[row.NodeID] = row.FileID
		}
	}
	if len(fileOfNode) == 0 {
		return result, nil
	}
	fileIDs := make([]int, 0, len(fileOfNode))
	nodeByFile := make(map[int]int, len(fileOfNode))
	for nodeID, fileID := range fileOfNode {
		fileIDs = append(fileIDs, fileID)
		nodeByFile[fileID] = nodeID
	}
	files, err := client.File.Query().Where(file.IDIn(fileIDs...)).All(ctx)
	if err != nil {
		return nil, fmt.Errorf("loading the files of %q: %w", machineName, err)
	}
	for _, f := range files {
		if nodeID, ok := nodeByFile[f.ID]; ok {
			result[nodeID] = dao.ToFileEntity(f)
		}
	}
	return result, nil
}

// textFieldValues - value of the text field storage machineName for each
// of nodeIDs (field engine, field_value_text); nodes without it are
// absent.
func textFieldValues(ctx context.Context, client *generated.Client, machineName string, nodeIDs []int) (map[int]string, error) {
	out := map[int]string{}
	if len(nodeIDs) == 0 || machineName == "" {
		return out, nil
	}
	storageID, err := fieldStorageID(ctx, client, machineName)
	if err != nil || storageID == 0 {
		return out, err
	}
	rows, err := client.FieldValueText.Query().
		Where(fieldvaluetext.FieldIDEQ(storageID), fieldvaluetext.NodeIDIn(nodeIDs...)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("loading the values of %q: %w", machineName, err)
	}
	for _, row := range rows {
		out[row.NodeID] = row.Value
	}
	return out, nil
}

// queryLanguages - the languages a query's words are stemmed in: the
// site's (its text may be in any of them) and the page's.
func queryLanguages(ctx context.Context, client *generated.Client) (def string, langs []string, err error) {
	def, others := entity.SiteLanguages()
	if def == "" {
		if def, err = siteLanguage(ctx, client); err != nil {
			return "", nil, err
		}
	}
	for _, l := range append([]string{def, i18n.FromContext(ctx)}, languageCodes(others)...) {
		if l != "" && !slices.Contains(langs, l) {
			langs = append(langs, l)
		}
	}
	return def, langs, nil
}

// languageCodes - the codes of the languages.
func languageCodes(ls []entity.SiteLanguage) []string {
	out := make([]string, len(ls))
	for i, l := range ls {
		out[i] = l.Code
	}
	return out
}

var likeEscaper = strings.NewReplacer(`\`, `\\`, "%", `\%`, "_", `\_`)

// foldedContains - the column col contains sub, folded (searchtext.Fold):
// the column is folded in SQL, so both databases match alike.
func foldedContains(col, sub string) *sql.Predicate {
	return sql.P(func(b *sql.Builder) {
		b.WriteString("REPLACE(LOWER(").Ident(col).WriteString("), ").Arg("ё").WriteString(", ").Arg("е").
			WriteString(") LIKE ").Arg("%" + likeEscaper.Replace(sub) + "%")
	})
}

// hasWord - the node of s has the i-th word of q (one of its forms; the
// last word - a word beginning with one) among its words of the fields
// fieldIDs (titleField - the title).
func hasWord(s *sql.Selector, q searchtext.Query, i int, fieldIDs []int) *sql.Predicate {
	d := sql.Dialect(s.Dialect())
	w := d.Table(searchword.Table)
	var match *sql.Predicate
	if q.Prefix(i) {
		ps := make([]*sql.Predicate, len(q.Words[i]))
		for j, f := range q.Words[i] {
			ps[j] = sql.HasPrefix(w.C(searchword.FieldWord), f)
		}
		match = sql.Or(ps...)
	} else {
		forms := make([]any, len(q.Words[i]))
		for j, f := range q.Words[i] {
			forms[j] = f
		}
		match = sql.In(w.C(searchword.FieldWord), forms...)
	}
	return sql.In(s.C(node.FieldID), d.Select(w.C(searchword.FieldNid)).From(w).
		Where(sql.And(sql.InInts(w.C(searchword.FieldFieldID), fieldIDs...), match)))
}

// allWords - the nodes with every word of q among their words of the
// fields fieldIDs.
func allWords(q searchtext.Query, fieldIDs []int) predicate.Node {
	return func(s *sql.Selector) {
		for i := range q.Words {
			s.Where(hasWord(s, q, i, fieldIDs))
		}
	}
}

// titleRank orders the nodes with the phrase in the title first, then
// those with all the words in it, then the rest (titleRankOf in SQL).
func titleRank(q searchtext.Query) node.OrderOption {
	return func(s *sql.Selector) {
		all := make([]*sql.Predicate, len(q.Words))
		for i := range q.Words {
			all[i] = hasWord(s, q, i, []int{titleField})
		}
		// sql.ExprFunc keeps the arguments (OrderExprFunc drops them).
		s.OrderExpr(sql.ExprFunc(func(b *sql.Builder) {
			b.WriteString("CASE WHEN ").Join(foldedContains(s.C(node.FieldTitle), q.Phrase)).
				WriteString(" THEN 0 WHEN ").Join(sql.And(all...)).WriteString(" THEN 1 ELSE 2 END")
		}))
	}
}

// titleRankOf - 0: the phrase is in the title, 1: all the words are (in
// the title's words, stemmed in lang), 2: neither (they are in the match
// fields).
func titleRankOf(title, lang string, q searchtext.Query) int {
	if strings.Contains(searchtext.Fold(title), q.Phrase) {
		return 0
	}
	if q.Matches(searchtext.Words(title, lang)) {
		return 1
	}
	return 2
}

// searchesFields - whether the type t searches fields besides the title:
// all its text fields or the chosen ones, its limit above 0.
func searchesFields(t entity.SearchTypeDef) bool {
	return t.Limit > 0 && (t.AllFields || len(t.MatchFields) > 0)
}

// searchFields - the field storages the type t searches besides the
// title (searchesFields), in order: all its text fields (AllFields) in
// the type's order, else its MatchFields in theirs.
func searchFields(ctx context.Context, client *generated.Client, t entity.SearchTypeDef) ([]int, error) {
	if !searchesFields(t) {
		return nil, nil
	}
	var ids []int
	if t.AllFields {
		fields, err := client.FieldInstanceDef.Query().
			Where(
				fieldinstancedef.HasContentTypeWith(contenttypedef.MachineName(t.NodeType)),
				fieldinstancedef.HasFieldStorageWith(fieldstoragedef.FieldTypeEQ(string(entity.FieldTypeText))),
			).
			Order(generated.Asc(fieldinstancedef.FieldSortOrder), generated.Asc(fieldinstancedef.FieldID)).
			All(ctx)
		if err != nil {
			return nil, fmt.Errorf("loading the text fields of %s: %w", t.NodeType, err)
		}
		for _, f := range fields {
			if !slices.Contains(ids, f.FieldStorageID) {
				ids = append(ids, f.FieldStorageID)
			}
		}
		return ids, nil
	}
	storages, err := client.FieldStorageDef.Query().Where(fieldstoragedef.MachineNameIn(t.MatchFields...)).All(ctx)
	if err != nil {
		return nil, fmt.Errorf("loading the match fields %v: %w", t.MatchFields, err)
	}
	for _, name := range t.MatchFields {
		for _, st := range storages {
			if st.MachineName == name {
				ids = append(ids, st.ID)
			}
		}
	}
	return ids, nil
}

// badgeFromTerms - the names of terms of vocabularyID in lang, "A / B".
func badgeFromTerms(terms entity.TaxonomyTerms, vocabularyID int, lang string) string {
	if vocabularyID <= 0 {
		return ""
	}
	badgeTerms := terms.FilterByVID(vocabularyID)
	if len(badgeTerms) == 0 {
		return ""
	}
	names := make([]string, 0, len(badgeTerms))
	for _, t := range badgeTerms {
		names = append(names, entity.LocalizeTerm(lang, t).Name)
	}
	return strings.Join(names, " / ")
}

// Search runs the live search for the words of query (searchtext.Query:
// stemmed in the site's languages, the last one as a beginning), each to
// be found, in any order, in the search index (search_words):
//  1. per type searching fields (searchesFields; in parallel): nodes with
//     every word in the title or one of its fields, up to the type's Limit;
//  2. one title search over every searched type (limit*2), every word in
//     the title, adding nodes not found yet;
//  3. batch enrichment per type: title/subtitle fields, image field (or
//     its default image, or the placeholder image) in the type's style,
//     badge terms, label;
//  4. grouped by type in weight order, inside a type by titleRankOf, then
//     the newest; cut to limit.
func (r *SearchRepository) Search(ctx context.Context, query string, limit int, types entity.SearchTypeDefs, settings entity.SearchSettings) ([]entity.SearchResult, error) {
	client := GetClient(ctx, r.client)
	if len(types) == 0 {
		return nil, nil
	}
	def, langs, err := queryLanguages(ctx, client)
	if err != nil {
		return nil, fmt.Errorf("searching %q: %w", query, err)
	}
	q := searchtext.ParseQuery(query, langs)
	if len(q.Words) == 0 {
		return nil, nil
	}
	known, err := knownNodeTypes(ctx, client)
	if err != nil {
		return nil, fmt.Errorf("searching %q: %w", query, err)
	}
	nodePred := node.And(node.Status(1), node.OnModeration(0), known)
	// The content of the page's language (SetContentListFilter).
	if p := ContentListPredicate(ctx); p != nil {
		nodePred = node.And(nodePred, p)
	}

	var mu sync.Mutex
	seen := make(map[int]bool)
	byType := make(map[string][]*generated.Node, len(types))
	add := func(n *generated.Node) {
		mu.Lock()
		defer mu.Unlock()
		if seen[n.ID] {
			return
		}
		seen[n.ID] = true
		byType[n.Type] = append(byType[n.Type], n)
	}

	// Step 1: field matches, one query pair per type, in parallel. A query
	// that fails fails the search (no partial results passed off as all).
	var wg sync.WaitGroup
	var errs []error
	fail := func(err error) {
		mu.Lock()
		defer mu.Unlock()
		errs = append(errs, err)
	}
	// The best title matches, then the newest: which of many matches come
	// is no accident.
	ranked := []node.OrderOption{titleRank(q), generated.Desc(node.FieldCreatedAt), generated.Desc(node.FieldID)}
	for _, t := range types {
		if !searchesFields(t) {
			continue
		}
		wg.Add(1)
		go func(t entity.SearchTypeDef) {
			defer wg.Done()
			defer panicsafe.Recover(slog.Default(), "search."+t.NodeType)
			storageIDs, err := searchFields(ctx, client, t)
			if err != nil {
				fail(err)
				return
			}
			if len(storageIDs) == 0 {
				return
			}
			nq := client.Node.Query().Where(node.Type(t.NodeType), nodePred, allWords(q, append(storageIDs, titleField))).Order(ranked...).Limit(t.Limit)
			nodeWithEdges(nq)
			nodes, err := nq.All(ctx)
			if err != nil {
				fail(fmt.Errorf("loading the %s matches: %w", t.NodeType, err))
				return
			}
			for _, n := range nodes {
				add(n)
			}
		}(t)
	}
	wg.Wait()
	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}

	// Step 2: one node.title search over every searched type.
	typeNames := make([]string, len(types))
	for i, t := range types {
		typeNames[i] = t.NodeType
	}
	titleQuery := client.Node.Query().Where(nodePred, node.TypeIn(typeNames...), allWords(q, []int{titleField})).Order(ranked...).Limit(limit * 2)
	nodeWithEdges(titleQuery)
	nodesByTitle, err := titleQuery.All(ctx)
	if err != nil {
		return nil, fmt.Errorf("matching the titles: %w", err)
	}
	for _, n := range nodesByTitle {
		add(n)
	}

	// Step 3: enrich per type, in weight order.
	results := make([]entity.SearchResult, 0, limit)
	for _, t := range types {
		nodes := byType[t.NodeType]
		if len(nodes) == 0 {
			continue
		}
		slices.SortStableFunc(nodes, func(a, b *generated.Node) int {
			if ra, rb := titleRankOf(a.Title, nodeLanguage(a.Langcode, def), q), titleRankOf(b.Title, nodeLanguage(b.Langcode, def), q); ra != rb {
				return ra - rb
			}
			if c := b.CreatedAt.Compare(a.CreatedAt); c != 0 {
				return c
			}
			return b.ID - a.ID
		})
		ids := make([]int, len(nodes))
		for i, n := range nodes {
			ids[i] = n.ID
		}
		titles, err := textFieldValues(ctx, client, t.TitleField, ids)
		if err != nil {
			return nil, err
		}
		subtitles, err := textFieldValues(ctx, client, t.SubtitleField, ids)
		if err != nil {
			return nil, err
		}
		images, err := imageFieldValues(ctx, client, t.ImageField, ids)
		if err != nil {
			return nil, err
		}
		defaultImage, err := imageFieldDefault(ctx, client, t.NodeType, t.ImageField)
		if err != nil {
			return nil, err
		}
		for _, n := range nodes {
			entNode := dao.ToNodeEntity(n)
			title := entNode.Title
			if v, ok := titles[n.ID]; ok {
				title = v
			}
			img, ok := images[n.ID]
			if !ok {
				img = defaultImage
			}
			imageURL := imageurl.Styled(img, t.ImageStyle).URI
			if imageURL == "" {
				imageURL = settings.PlaceholderImage
			}
			label := t.Label
			if label == "" {
				label = t.NodeType
			}
			results = append(results, entity.SearchResult{
				ID:        n.ID,
				Type:      t.NodeType,
				URL:       entNode.URL(),
				Title:     title,
				Subtitle:  subtitles[n.ID],
				ImageURL:  imageURL,
				Badge:     badgeFromTerms(dao.ToTaxonomyTermsEntity(n.Edges.TaxonomyIndices), t.BadgeVocabularyID, i18n.FromContext(ctx)),
				TypeLabel: label,
			})
		}
	}

	if len(results) > limit {
		results = results[:limit]
	}
	return results, nil
}
