package viewquery

import (
	"context"
	"fmt"
	"math"

	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/database"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated"
)

// similarRank - how much a candidate of the similar_to_current_node
// filter has in common with the current node: Score, the rarity-weighted
// sum of the terms they share, and Shared, how many terms that is.
type similarRank struct {
	Score  float64
	Shared int
}

// similarRanking - the candidates of similar_to_current_node by node ID.
type similarRanking map[int]similarRank

// similarQuery ranks the published nodes sharing taxonomy terms with
// the current node (nid, the first and last placeholders). Each shared
// term t weighs 1 / ln(1 + n_t), n_t - the published nodes indexed under
// t (the current one included when it is published): the rarer the term,
// the more it counts (a term of 2 nodes weighs 0.91, of 10 - 0.42, of
// 300 - 0.18). The subquery d counts n_t for the current node's terms
// only; %s is the vocabulary restriction (similarVocabularyClause).
// Portable: LN and the arithmetic are the same on MySQL and PostgreSQL.
const similarQuery = `SELECT c.nid, SUM(1.0 / LN(1 + d.df)), COUNT(*)
FROM (
	SELECT cur.tid, COUNT(*) AS df
	FROM taxonomy_index cur
	JOIN taxonomy_index o ON o.tid = cur.tid
	JOIN nodes n ON n.id = o.nid
	WHERE cur.nid = ? AND n.status = 1 AND n.on_moderation = 0%s
	GROUP BY cur.tid
) d
JOIN taxonomy_index c ON c.tid = d.tid
JOIN nodes cn ON cn.id = c.nid
WHERE c.nid <> ? AND cn.status = 1 AND cn.on_moderation = 0
GROUP BY c.nid`

// similarVocabularyClause - similarQuery's restriction of the current
// node's terms to one vocabulary (its ID is the placeholder).
const similarVocabularyClause = `
	  AND cur.tid IN (SELECT t.id FROM taxonomy_term_data t WHERE t.vid = ?)`

// similarToNode ranks the nodes similar to nodeID by the terms they share
// with it (similarQuery), of vocabulary vid when vid > 0, of every one
// otherwise. Empty when nodeID has no such terms or nobody shares them.
func similarToNode(ctx context.Context, client *generated.Client, nodeID, vid int) (similarRanking, error) {
	clause, args := "", []any{nodeID}
	if vid > 0 {
		clause = similarVocabularyClause
		args = append(args, vid)
	}
	args = append(args, nodeID)
	rows, err := client.QueryContext(ctx, database.Rebind(fmt.Sprintf(similarQuery, clause)), args...)
	if err != nil {
		return nil, fmt.Errorf("ranking the nodes similar to node %d: %w", nodeID, err)
	}
	defer func() { _ = rows.Close() }()
	ranking := similarRanking{}
	for rows.Next() {
		var nid int
		var r similarRank
		if err := rows.Scan(&nid, &r.Score, &r.Shared); err != nil {
			return nil, fmt.Errorf("ranking the nodes similar to node %d: %w", nodeID, err)
		}
		ranking[nid] = r
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ranking the nodes similar to node %d: %w", nodeID, err)
	}
	return ranking, nil
}

// ids - the ranked nodes' IDs, in no particular order.
func (r similarRanking) ids() []int {
	out := make([]int, 0, len(r))
	for id := range r {
		out = append(out, id)
	}
	return out
}

// sortKeys - the ranking as the leading keys of sortIDsByKeys: the score,
// then how many terms are shared, both descending. The score is rounded
// (to 1e-9) so two nodes sharing equally weighted terms tie whatever order
// the database summed them in; the View's own sort criteria come after
// these keys and break the ties.
func (r similarRanking) sortKeys() []sortKey {
	score := make(map[int]float64, len(r))
	shared := make(map[int]float64, len(r))
	for id, rank := range r {
		score[id] = math.Round(rank.Score*1e9) / 1e9
		shared[id] = float64(rank.Shared)
	}
	return []sortKey{
		{num: score, isNum: true, desc: true},
		{num: shared, isNum: true, desc: true},
	}
}
