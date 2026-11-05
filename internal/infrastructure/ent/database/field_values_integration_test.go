//go:build dbintegration

package database_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/database"
	"github.com/kosatka-cms/kosatka/internal/testdb"
)

// TestTextFormatIDsByNode - the text format each formatted text field's
// value was saved with, by field instance; a field without allowed
// formats, or a value without a format, isn't there.
func TestTextFormatIDsByNode(t *testing.T) {
	schemaDB(t, func(t *testing.T, db testdb.DB) {
		ctx := context.Background()
		seedUser(t, db)
		for _, q := range []string{
			`INSERT INTO nodes (id, type, title, slug, created_by) VALUES (5, 'article', 'Harbour', 'news/harbour', 1)`,
			`INSERT INTO text_formats (id, machine_name, label, created_at, updated_at) VALUES (50, 'test_basic', 'Test basic', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
			`INSERT INTO field_storages (id, machine_name, field_type, created_at, updated_at) VALUES (7, 'body', 'text', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP), (8, 'lead', 'text', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP), (9, 'note', 'text', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`,
			`INSERT INTO field_value_text (node_id, field_id, value, text_format_id) VALUES (5, 7, '<p>Body</p>', 50), (5, 8, 'Lead', NULL), (5, 9, 'Note', 50)`,
		} {
			seedRow(t, db, q)
		}
		formatted := map[string]any{"allowed_text_formats": []any{float64(50)}}
		fields := entity.FieldInstanceDefs{
			{ID: 70, FieldStorageID: 7, FieldType: entity.FieldTypeText, WidgetConfig: formatted},
			{ID: 80, FieldStorageID: 8, FieldType: entity.FieldTypeText, WidgetConfig: formatted},
			{ID: 90, FieldStorageID: 9, FieldType: entity.FieldTypeText},
		}
		got, err := database.NewContentTypeRepository(entClient(db)).TextFormatIDsByNode(ctx, 5, fields)
		if err != nil || !reflect.DeepEqual(got, map[int]int{70: 50}) {
			t.Errorf("formats: %v %v", got, err)
		}
		if got, err := database.NewContentTypeRepository(entClient(db)).TextFormatIDsByNode(ctx, 6, fields); err != nil || len(got) != 0 {
			t.Errorf("a node without values: %v %v", got, err)
		}
	})
}
