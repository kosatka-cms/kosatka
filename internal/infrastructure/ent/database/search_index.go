package database

import (
	"context"
	"fmt"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/fieldvaluetext"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/node"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/searchword"
	"github.com/kosatka-cms/kosatka/internal/lib/searchtext"
	"github.com/kosatka-cms/kosatka/pkg/metadescription"
)

// The search index (search_words): the words of each node's title (field
// 0) and of its text fields' values (internal/lib/searchtext), stemmed in the
// node's language. They are written where the text is: the title's with
// the node's row, a field's with its values (SetFieldValues); app search
// reindex (RebuildSearchIndex) writes them all again.

// titleField - the field_id of a node's title words.
const titleField = 0

// searchWordsBatch - the rows written in one statement.
const searchWordsBatch = 500

// indexAll - ctx whose node queries see every node: the index has the
// nodes of every type, in the trash too (the search leaves them out).
func indexAll(ctx context.Context) context.Context {
	return entity.WithAllNodeTypes(entity.WithTrashed(ctx))
}

// siteLanguage - the site's default language (site_information); "" -
// none set.
func siteLanguage(ctx context.Context, client *generated.Client) (string, error) {
	s, err := client.SiteInformation.Get(ctx, settingsRowID)
	if generated.IsNotFound(err) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("loading the site's language: %w", err)
	}
	return deref(s.DefaultLanguage), nil
}

// nodeLanguage - the language a node's words are stemmed in: its own,
// else the site's (def).
func nodeLanguage(langcode, def string) string {
	if langcode != "" {
		return langcode
	}
	return def
}

// indexNodeTexts rewrites the node's words of the fields of texts (field
// id → plain text; "" - none) in the language lang.
func indexNodeTexts(ctx context.Context, client *generated.Client, nodeID int, texts map[int]string, lang string) error {
	fieldIDs := make([]int, 0, len(texts))
	for id := range texts {
		fieldIDs = append(fieldIDs, id)
	}
	if _, err := client.SearchWord.Delete().Where(searchword.Nid(nodeID), searchword.FieldIDIn(fieldIDs...)).Exec(ctx); err != nil {
		return fmt.Errorf("deleting the search words of node %d: %w", nodeID, err)
	}
	var rows []*generated.SearchWordCreate
	for fieldID, text := range texts {
		for _, w := range searchtext.Words(text, lang) {
			rows = append(rows, client.SearchWord.Create().SetNid(nodeID).SetFieldID(fieldID).SetWord(w))
		}
	}
	return createSearchWords(ctx, client, rows)
}

// createSearchWords writes the rows, searchWordsBatch a statement.
func createSearchWords(ctx context.Context, client *generated.Client, rows []*generated.SearchWordCreate) error {
	for len(rows) > 0 {
		n := min(len(rows), searchWordsBatch)
		if err := client.SearchWord.CreateBulk(rows[:n]...).Exec(ctx); err != nil {
			return fmt.Errorf("writing the search words: %w", err)
		}
		rows = rows[n:]
	}
	return nil
}

// indexTitle rewrites the node's title words (after its row is saved).
func indexTitle(ctx context.Context, client *generated.Client, nodeID int) error {
	n, err := client.Node.Query().Where(node.ID(nodeID)).Select(node.FieldTitle, node.FieldLangcode).Only(indexAll(ctx))
	if err != nil {
		return fmt.Errorf("loading node %d for the search: %w", nodeID, err)
	}
	def, err := siteLanguage(ctx, client)
	if err != nil {
		return err
	}
	return indexNodeTexts(ctx, client, nodeID, map[int]string{titleField: n.Title}, nodeLanguage(n.Langcode, def))
}

// indexFields rewrites the node's words of the text values of the field
// storages storageIDs (after they are saved; a storage of another type
// has no text values - nothing to write).
func indexFields(ctx context.Context, client *generated.Client, nodeID int, storageIDs []int) error {
	if len(storageIDs) == 0 {
		return nil
	}
	n, err := client.Node.Query().Where(node.ID(nodeID)).Select(node.FieldLangcode).Only(indexAll(ctx))
	if err != nil {
		return fmt.Errorf("loading node %d for the search: %w", nodeID, err)
	}
	def, err := siteLanguage(ctx, client)
	if err != nil {
		return err
	}
	values, err := client.FieldValueText.Query().
		Where(fieldvaluetext.NodeID(nodeID), fieldvaluetext.FieldIDIn(storageIDs...)).
		All(ctx)
	if err != nil {
		return fmt.Errorf("loading the text values of node %d: %w", nodeID, err)
	}
	texts := make(map[int]string, len(storageIDs))
	for _, id := range storageIDs {
		texts[id] = ""
	}
	for _, v := range values {
		texts[v.FieldID] = metadescription.PlainText(v.Value)
	}
	return indexNodeTexts(ctx, client, nodeID, texts, nodeLanguage(n.Langcode, def))
}

// indexNode rewrites all the node's words: its title's and its text
// values' (its language changed, say).
func indexNode(ctx context.Context, client *generated.Client, nodeID int) error {
	values, err := client.FieldValueText.Query().Where(fieldvaluetext.NodeID(nodeID)).All(ctx)
	if err != nil {
		return fmt.Errorf("loading the text values of node %d: %w", nodeID, err)
	}
	storageIDs := make([]int, len(values))
	for i, v := range values {
		storageIDs[i] = v.FieldID
	}
	if err := indexTitle(ctx, client, nodeID); err != nil {
		return err
	}
	return indexFields(ctx, client, nodeID, storageIDs)
}

// RebuildSearchIndex writes the words of every node again (app search
// reindex, a new site's content): a batch of nodes at a time, each
// batch's old words replaced, so the search keeps finding meanwhile. It
// returns how many nodes it indexed.
func RebuildSearchIndex(ctx context.Context, client *generated.Client) (int, error) {
	def, err := siteLanguage(ctx, client)
	if err != nil {
		return 0, err
	}
	count, lastID := 0, 0
	for {
		nodes, err := client.Node.Query().Where(node.IDGT(lastID)).Order(generated.Asc(node.FieldID)).
			Limit(searchWordsBatch).Select(node.FieldID, node.FieldTitle, node.FieldLangcode).All(indexAll(ctx))
		if err != nil {
			return count, fmt.Errorf("loading the nodes for the search: %w", err)
		}
		if len(nodes) == 0 {
			return count, nil
		}
		ids := make([]int, len(nodes))
		for i, n := range nodes {
			ids[i] = n.ID
		}
		values, err := client.FieldValueText.Query().Where(fieldvaluetext.NodeIDIn(ids...)).All(ctx)
		if err != nil {
			return count, fmt.Errorf("loading the text values for the search: %w", err)
		}
		texts := make(map[int]map[int]string, len(nodes))
		for _, n := range nodes {
			texts[n.ID] = map[int]string{titleField: n.Title}
		}
		for _, v := range values {
			texts[v.NodeID][v.FieldID] = metadescription.PlainText(v.Value)
		}
		err = NewUnitOfWork(client).Do(ctx, func(ctx context.Context) error {
			tx := GetClient(ctx, client)
			if _, err := tx.SearchWord.Delete().Where(searchword.NidIn(ids...)).Exec(ctx); err != nil {
				return fmt.Errorf("deleting the search words: %w", err)
			}
			var rows []*generated.SearchWordCreate
			for _, n := range nodes {
				lang := nodeLanguage(n.Langcode, def)
				for fieldID, text := range texts[n.ID] {
					for _, w := range searchtext.Words(text, lang) {
						rows = append(rows, tx.SearchWord.Create().SetNid(n.ID).SetFieldID(fieldID).SetWord(w))
					}
				}
			}
			return createSearchWords(ctx, tx, rows)
		})
		if err != nil {
			return count, err
		}
		count += len(nodes)
		lastID = ids[len(ids)-1]
	}
}
