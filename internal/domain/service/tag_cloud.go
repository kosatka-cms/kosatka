package service

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// The tag cloud block (entity.PluginBlockTagCloud): the terms on the most published materials, by name, each in
// a size level by how many it is on.

// Tag cloud setting limits and defaults.
const (
	tagCloudDefaultCount  = 10
	tagCloudMaxCount      = 100
	tagCloudDefaultLevels = 6
	tagCloudMaxLevels     = 10
)

// RegisterTagCloudBlock gives the tag cloud block its settings and data
// (wiring).
func RegisterTagCloudBlock(taxonomy TaxonomyService) {
	settings := func(ctx context.Context) ([]entity.PluginBlockSetting, error) {
		return tagCloudSettings(ctx, taxonomy)
	}
	SetPluginBlockSettings(entity.PluginBlockTagCloud, settings)
	SetPluginBlockData(entity.PluginBlockTagCloud, func(ctx context.Context, p entity.BlockPlacementDef) (any, error) {
		list, err := settings(ctx)
		if err != nil {
			return nil, err
		}
		values := map[string]int{}
		for _, s := range list {
			values[s.Key], _ = strconv.Atoi(p.PluginSettingOr(s))
		}
		return taxonomy.TagCloud(ctx, values[entity.TagCloudVocabularyKey], values[entity.TagCloudCountKey], values[entity.TagCloudLevelsKey])
	})
}

// tagCloudSettings - the vocabulary (every one, or one of the site's),
// how many terms, in how many size levels.
func tagCloudSettings(ctx context.Context, taxonomy TaxonomyService) ([]entity.PluginBlockSetting, error) {
	vocabularies, err := taxonomy.Vocabularies(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing vocabularies for the tag cloud: %w", err)
	}
	options := []entity.PluginBlockTarget{{Value: "0", Label: i18n.T(ctx, "All vocabularies")}}
	for _, v := range vocabularies {
		options = append(options, entity.PluginBlockTarget{Value: strconv.Itoa(v.ID), Label: v.Name})
	}
	return []entity.PluginBlockSetting{
		{Key: entity.TagCloudVocabularyKey, Label: i18n.T(ctx, "Vocabulary"), Type: entity.PluginBlockSettingSelect, Default: "0", Options: options},
		{Key: entity.TagCloudCountKey, Label: i18n.T(ctx, "Number of terms"), Type: entity.PluginBlockSettingNumber,
			Default: strconv.Itoa(tagCloudDefaultCount), Min: 1, Max: tagCloudMaxCount},
		{Key: entity.TagCloudLevelsKey, Label: i18n.T(ctx, "Size levels"), Type: entity.PluginBlockSettingNumber,
			Default: strconv.Itoa(tagCloudDefaultLevels), Min: 1, Max: tagCloudMaxLevels},
	}, nil
}

// TagCloud - the count terms of the vocabulary (0 - every one) on the
// most published materials, by name, in levels size levels
// (tagCloudLevels).
func (s *DefaultTaxonomyService) TagCloud(ctx context.Context, vocabularyID, count, levels int) ([]entity.TagCloudTerm, error) {
	counts, err := s.repository.TermNodeCounts(ctx, vocabularyID)
	if err != nil {
		return nil, fmt.Errorf("building the tag cloud: %w", err)
	}
	ids := topTerms(counts, count)
	terms, err := s.repository.TermsByIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("loading the tag cloud's terms: %w", err)
	}
	out := make([]entity.TagCloudTerm, 0, len(ids))
	for _, id := range ids {
		if t, ok := terms[id]; ok {
			out = append(out, entity.TagCloudTerm{Term: t, Count: counts[id], Linked: t.HasPublicPage()})
		}
	}
	tagCloudLevels(out, levels)
	sort.SliceStable(out, func(i, j int) bool {
		return strings.ToLower(out[i].Term.Name) < strings.ToLower(out[j].Term.Name)
	})
	return out, nil
}

// topTerms - the IDs of the limit terms with the biggest counts (equal
// counts - the lower ID first).
func topTerms(counts map[int]int, limit int) []int {
	ids := make([]int, 0, len(counts))
	for id := range counts {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		if counts[ids[i]] != counts[ids[j]] {
			return counts[ids[i]] > counts[ids[j]]
		}
		return ids[i] < ids[j]
	})
	if limit >= 0 && len(ids) > limit {
		ids = ids[:limit]
	}
	return ids
}

// tagCloudLevels sets each term's Level, 1 to levels, on a logarithmic
// scale of its count between the fewest and the most: a
// term on a few more materials than the rarest isn't lost among them. All
// on the same count - the middle level.
func tagCloudLevels(terms []entity.TagCloudTerm, levels int) {
	if len(terms) == 0 {
		return
	}
	levels = max(levels, 1)
	lo, hi := math.Inf(1), math.Inf(-1)
	for _, t := range terms {
		w := math.Log(float64(max(t.Count, 1)))
		lo, hi = math.Min(lo, w), math.Max(hi, w)
	}
	for i := range terms {
		if hi == lo {
			terms[i].Level = (levels + 1) / 2
			continue
		}
		w := math.Log(float64(max(terms[i].Count, 1)))
		terms[i].Level = 1 + int(math.Round((w-lo)/(hi-lo)*float64(levels-1)))
	}
}
