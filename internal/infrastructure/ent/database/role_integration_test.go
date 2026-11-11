//go:build dbintegration

package database_test

import (
	"context"
	"slices"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/testdb"
	"github.com/kosatka-cms/kosatka/modulekit/manifestkit"
)

// TestFullAccessRoles - the roles with full access by name, the core's
// administrator among them; a role with other permissions, or with full
// access switched off, isn't.
func TestFullAccessRoles(t *testing.T) {
	schemaDB(t, func(t *testing.T, db testdb.DB) {
		ctx := context.Background()
		client := entClient(db)
		for name, perms := range map[string]map[string]any{
			"chief":  {entity.PermFullAccess: true},
			"editor": {"edit any article content": true},
			"lapsed": {entity.PermFullAccess: false},
		} {
			if err := client.Role.Create().SetName(name).SetPermissions(perms).Exec(ctx); err != nil {
				t.Fatal(err)
			}
		}
		got, err := manifestkit.FullAccessRoles(ctx, manifestkit.InstallEnv{DB: db.DB, Dialect: db.Dialect})
		if err != nil || !slices.Equal(got, []string{entity.RoleAdmin, "chief"}) {
			t.Errorf("full access roles: %v %v", got, err)
		}
	})
}
