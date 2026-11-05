package database

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/searchtype"
)

// SearchConfigRepository - search_types and search_settings
// (entity.SearchConfigRepository).
type SearchConfigRepository struct {
	client *generated.Client
}

// NewSearchConfigRepository - the search settings repository on db.
func NewSearchConfigRepository(db *sql.DB) *SearchConfigRepository {
	return &SearchConfigRepository{client: clientFor(db)}
}

func splitCSV(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// SearchTypes - see entity.SearchConfigRepository.SearchTypes.
func (r *SearchConfigRepository) SearchTypes(ctx context.Context) (entity.SearchTypeDefs, error) {
	rows, err := GetClient(ctx, r.client).SearchType.Query().
		Order(generated.Asc(searchtype.FieldWeight), generated.Asc(searchtype.FieldID)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("SearchConfigRepository.SearchTypes: %w", err)
	}
	out := make(entity.SearchTypeDefs, 0, len(rows))
	for _, t := range rows {
		d := entity.SearchTypeDef{
			NodeType: t.ID, Enabled: t.Enabled, Weight: t.Weight, Limit: t.ResultLimit, Label: t.Label,
			MatchFields: splitCSV(t.MatchFields), AllFields: t.AllFields, TitleField: t.TitleField, SubtitleField: t.SubtitleField,
			ImageField: t.ImageField, ImageStyle: t.ImageStyle,
		}
		if t.BadgeVocabularyID != nil {
			d.BadgeVocabularyID = *t.BadgeVocabularyID
		}
		out = append(out, d)
	}
	return out, nil
}

// SaveSearchType saves a content type's search settings, creating the row
// if it isn't there.
func (r *SearchConfigRepository) SaveSearchType(ctx context.Context, t entity.SearchTypeDef) error {
	client := GetClient(ctx, r.client)
	u := client.SearchType.UpdateOneID(t.NodeType).SetEnabled(t.Enabled).
		SetWeight(t.Weight).SetResultLimit(t.Limit).SetLabel(t.Label).SetMatchFields(strings.Join(t.MatchFields, ",")).SetAllFields(t.AllFields).
		SetTitleField(t.TitleField).SetSubtitleField(t.SubtitleField).SetImageField(t.ImageField).SetImageStyle(t.ImageStyle)
	if t.BadgeVocabularyID > 0 {
		u.SetBadgeVocabularyID(t.BadgeVocabularyID)
	} else {
		u.ClearBadgeVocabularyID()
	}
	err := u.Exec(ctx)
	if !generated.IsNotFound(err) {
		return err
	}
	c := client.SearchType.Create().SetID(t.NodeType).SetEnabled(t.Enabled).
		SetWeight(t.Weight).SetResultLimit(t.Limit).SetLabel(t.Label).SetMatchFields(strings.Join(t.MatchFields, ",")).SetAllFields(t.AllFields).
		SetTitleField(t.TitleField).SetSubtitleField(t.SubtitleField).SetImageField(t.ImageField).SetImageStyle(t.ImageStyle)
	if t.BadgeVocabularyID > 0 {
		c.SetBadgeVocabularyID(t.BadgeVocabularyID)
	}
	return c.Exec(ctx)
}

// UpdateSearchTypeWeight sets only the searched type's weight (its place
// in the order).
func (r *SearchConfigRepository) UpdateSearchTypeWeight(ctx context.Context, nodeType string, weight int) error {
	_, err := GetClient(ctx, r.client).SearchType.Update().Where(searchtype.ID(nodeType)).SetWeight(weight).Save(ctx)
	return err
}

// SearchSettings - the live search's site-wide settings from their single
// row.
func (r *SearchConfigRepository) SearchSettings(ctx context.Context) (*entity.SearchSettings, error) {
	s, err := GetClient(ctx, r.client).SearchSettings.Get(ctx, settingsRowID)
	if err != nil {
		return nil, fmt.Errorf("SearchConfigRepository.SearchSettings: %w", err)
	}
	return &entity.SearchSettings{PlaceholderImage: s.PlaceholderImage, CategoriesMenu: s.CategoriesMenu}, nil
}

// SaveSearchSettings saves s into the search settings row.
func (r *SearchConfigRepository) SaveSearchSettings(ctx context.Context, s entity.SearchSettings) error {
	return GetClient(ctx, r.client).SearchSettings.UpdateOneID(settingsRowID).
		SetPlaceholderImage(s.PlaceholderImage).SetCategoriesMenu(s.CategoriesMenu).Exec(ctx)
}
