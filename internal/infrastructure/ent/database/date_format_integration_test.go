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

func TestDateFormatRepository(t *testing.T) {
	schemaDB(t, func(t *testing.T, db testdb.DB) {
		ctx := context.Background()
		formats := database.NewDateFormatRepository(db.DB)
		id, err := formats.CreateDateFormat(ctx, entity.DateFormat{MachineName: "compact", Label: "Compact", Pattern: "d.m"})
		if err != nil {
			t.Fatal(err)
		}
		if err := formats.UpdateDateFormat(ctx, entity.DateFormat{ID: id, Label: "Brief", Pattern: "d.m.Y"}); err != nil {
			t.Fatal(err)
		}
		if f, err := formats.DateFormatByID(ctx, id); err != nil || f.MachineName != "compact" || f.Label != "Brief" || f.Pattern != "d.m.Y" {
			t.Errorf("date format: %+v %v", f, err)
		}
		if list, err := formats.DateFormats(ctx); err != nil || len(list) != 4 { // with the 3 the install step adds
			t.Errorf("date formats: %+v %v", list, err)
		}

		seedUser(t, db)
		seedRow(t, db, `INSERT INTO content_types (id, machine_name, label, created_by, created_at, updated_at) VALUES (1, 'article', 'Article', 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`)
		seedRow(t, db, `INSERT INTO field_storages (id, machine_name, field_type, created_at, updated_at) VALUES (7, 'published', 'date', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`)
		seedRow(t, db, `INSERT INTO field_instances (label, content_type_id, field_storage_id, date_format, created_at, updated_at) VALUES ('Published', 1, 7, 'compact', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`)
		seedRow(t, db, `INSERT INTO view_displays (id, machine_name, label, created_by, created_at, updated_at) VALUES (1, 'latest', 'Latest', 1, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`)
		seedRow(t, db, `INSERT INTO view_fields (label, view_display_id, date_format, created_at, updated_at) VALUES ('Date', 1, 'compact', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)`)

		want := []string{`Field "Published" of the type "Article"`, `View field "Latest"`}
		if usage, err := formats.DateFormatUsage(ctx, "compact"); err != nil || !reflect.DeepEqual(usage, want) {
			t.Errorf("usage: %q %v", usage, err)
		}
		if m, err := formats.InstanceDateFormats(ctx); err != nil || m[entity.InstanceDateFormatKey("article", 7)] != "compact" || len(m) != 1 {
			t.Errorf("instance date formats: %+v %v", m, err)
		}
		_ = formats.DeleteDateFormat(ctx, id)
		if list, _ := formats.DateFormats(ctx); len(list) != 3 {
			t.Errorf("date formats after delete: %+v", list)
		}
	})
}
