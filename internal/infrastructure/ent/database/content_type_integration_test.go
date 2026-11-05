//go:build dbintegration

package database_test

import (
	"context"
	"testing"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/database"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated/fieldinstancedef"
	"github.com/kosatka-cms/kosatka/internal/testdb"
)

// TestDeleteContentTypeWithFields - a type goes with its fields, a group
// and its members among them; a field only it used leaves the catalog, a
// field another type has stays.
func TestDeleteContentTypeWithFields(t *testing.T) {
	schemaDB(t, func(t *testing.T, db testdb.DB) {
		ctx := context.Background()
		seedUser(t, db)
		client := entClient(db)
		repo := database.NewContentTypeRepository(client)
		now := time.Now()

		newType := func(name string) int {
			ct, err := client.ContentTypeDef.Create().SetMachineName(name).SetLabel(name).
				SetStatus(1).SetCreatedBy(1).SetCreatedAt(now).SetUpdatedAt(now).Save(ctx)
			if err != nil {
				t.Fatal(err)
			}
			return ct.ID
		}
		newStorage := func(name, fieldType string) int {
			st, err := client.FieldStorageDef.Create().SetMachineName(name).SetFieldType(fieldType).
				SetCreatedAt(now).SetUpdatedAt(now).Save(ctx)
			if err != nil {
				t.Fatal(err)
			}
			return st.ID
		}
		attach := func(typeID, storageID int, group *int) int {
			fi, err := client.FieldInstanceDef.Create().SetContentTypeID(typeID).SetFieldStorageID(storageID).SetLabel("F").
				SetNillableParentGroupID(group).SetNillableFormGroupID(group).
				SetCreatedAt(now).SetUpdatedAt(now).Save(ctx)
			if err != nil {
				t.Fatal(err)
			}
			return fi.ID
		}

		event, page := newType("event"), newType("page")
		group, venue, body := newStorage("group_info", "group"), newStorage("field_venue", "text"), newStorage("field_body", "text")
		groupID := attach(event, group, nil)
		attach(event, venue, &groupID)
		attach(event, body, &groupID)
		attach(page, body, nil)

		if err := repo.DeleteContentType(ctx, event); err != nil {
			t.Fatal(err)
		}
		if _, err := client.ContentTypeDef.Get(ctx, event); err == nil {
			t.Error("the type is still there")
		}
		if n, err := client.FieldInstanceDef.Query().Where(fieldinstancedef.ContentTypeID(event)).Count(ctx); err != nil || n != 0 {
			t.Errorf("the type's fields: %d (%v), want 0", n, err)
		}
		for _, id := range []int{group, venue} {
			if _, err := client.FieldStorageDef.Get(ctx, id); err == nil {
				t.Errorf("storage %d only the type used is still in the catalog", id)
			}
		}
		if _, err := client.FieldStorageDef.Get(ctx, body); err != nil {
			t.Errorf("the shared storage: %v", err)
		}
		if n, err := client.FieldInstanceDef.Query().Where(fieldinstancedef.ContentTypeID(page)).Count(ctx); err != nil || n != 1 {
			t.Errorf("the other type's fields: %d (%v), want 1", n, err)
		}
	})
}

// TestDuplicateContentType - a taken machine name is a duplicate key on
// both databases (entity.IsDuplicateKey), so the service tells the user
// instead of failing.
func TestDuplicateContentType(t *testing.T) {
	schemaDB(t, func(t *testing.T, db testdb.DB) {
		ctx := context.Background()
		seedUser(t, db)
		client := entClient(db)
		now := time.Now()
		create := func() error {
			return client.ContentTypeDef.Create().SetMachineName("event").SetLabel("Event").
				SetStatus(1).SetCreatedBy(1).SetCreatedAt(now).SetUpdatedAt(now).Exec(ctx)
		}
		if err := create(); err != nil {
			t.Fatal(err)
		}
		if err := create(); !entity.IsDuplicateKey(err) {
			t.Fatalf("a taken machine name: %v", err)
		}
	})
}
