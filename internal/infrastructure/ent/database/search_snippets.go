package database

import (
	"context"
	"fmt"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/fieldvaluetext"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/node"
	"github.com/kosatka-cms/kosatka/internal/lib/searchtext"
	"github.com/kosatka-cms/kosatka/pkg/metadescription"
)

// snippetWidth - about how many letters a snippet has.
const snippetWidth = 200

// Snippets - per result its snippet (searchtext.Query.Snippet, the words
// stemmed in the item's language): from the first of the fields its type
// searches (searchFields), in their order, whose text has the query's
// words; a type searching only the title has none.
func (r *SearchRepository) Snippets(ctx context.Context, query string, results []entity.SearchResult, types entity.SearchTypeDefs) (map[int]entity.SearchSnippet, error) {
	out := map[int]entity.SearchSnippet{}
	client := GetClient(ctx, r.client)
	def, langs, err := queryLanguages(ctx, client)
	if err != nil {
		return nil, fmt.Errorf("the snippets of %q: %w", query, err)
	}
	q := searchtext.ParseQuery(query, langs)
	if len(q.Words) == 0 || len(results) == 0 {
		return out, nil
	}
	byType := map[string][]int{}
	ids := make([]int, len(results))
	for i, res := range results {
		byType[res.Type] = append(byType[res.Type], res.ID)
		ids[i] = res.ID
	}
	nodes, err := client.Node.Query().Where(node.IDIn(ids...)).Select(node.FieldID, node.FieldLangcode).All(ctx)
	if err != nil {
		return nil, fmt.Errorf("loading the results' languages: %w", err)
	}
	lang := make(map[int]string, len(nodes))
	for _, n := range nodes {
		lang[n.ID] = nodeLanguage(n.Langcode, def)
	}
	for _, t := range types {
		nodeIDs := byType[t.NodeType]
		if len(nodeIDs) == 0 {
			continue
		}
		storageIDs, err := searchFields(ctx, client, t)
		if err != nil {
			return nil, err
		}
		if len(storageIDs) == 0 {
			continue
		}
		rows, err := client.FieldValueText.Query().
			Where(fieldvaluetext.NodeIDIn(nodeIDs...), fieldvaluetext.FieldIDIn(storageIDs...)).
			All(ctx)
		if err != nil {
			return nil, fmt.Errorf("loading the %s results' texts: %w", t.NodeType, err)
		}
		values := map[int]map[int]string{}
		for _, row := range rows {
			if values[row.NodeID] == nil {
				values[row.NodeID] = map[int]string{}
			}
			values[row.NodeID][row.FieldID] = row.Value
		}
		for _, id := range nodeIDs {
			for _, storageID := range storageIDs {
				v, ok := values[id][storageID]
				if !ok {
					continue
				}
				if s, ok := q.Snippet(metadescription.PlainText(v), lang[id], snippetWidth); ok {
					out[id] = snippetEntity(s)
					break
				}
			}
		}
	}
	return out, nil
}

// snippetEntity - s as the entity.
func snippetEntity(s searchtext.Snippet) entity.SearchSnippet {
	parts := make([]entity.SearchSnippetPart, len(s.Parts))
	for i, p := range s.Parts {
		parts[i] = entity.SearchSnippetPart{Text: p.Text, Match: p.Match}
	}
	return entity.SearchSnippet{Parts: parts, Before: s.Before, After: s.After}
}
