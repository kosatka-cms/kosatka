//go:build dbintegration

package testkit

import (
	"context"
	"testing"

	"entgo.io/ent/dialect"

	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/database"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/migrations"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/sqldialect"
	"github.com/kosatka-cms/kosatka/internal/modules/manifests"
	"github.com/kosatka-cms/kosatka/internal/testdb"
	"github.com/kosatka-cms/kosatka/modulekit"
)

// DB - a fresh test database: DB (*sql.DB), Dialect (its Rebind puts a
// query's ? placeholders in the database's).
type DB = testdb.DB

// SchemaDB runs fn on each test database with every set's schema
// installed - the core's, the engine's modules', and those of the
// modules the test binary imports (the module under test) - and the
// repositories speaking its dialect (modulekit.DBDialect).
func SchemaDB(t *testing.T, fn func(t *testing.T, db DB)) {
	t.Helper()
	manifests.Register() // the modules' tables make their own sets
	testdb.Each(t, func(t *testing.T, db testdb.DB) {
		ctx := context.Background()
		r := migrations.Runner{DB: db.DB, Dialect: db.Dialect}
		for _, s := range migrations.Sets() {
			if _, err := r.Install(ctx, s.Name); err != nil {
				t.Fatal(err)
			}
		}
		d := dialect.MySQL
		if db.Dialect == sqldialect.Postgres {
			d = dialect.Postgres
		}
		database.SetDialect(d)
		t.Cleanup(func() { database.SetDialect(dialect.MySQL) })
		fn(t, db)
	})
}

// Deps - what Wire gets on a site (modulekit.Deps), the core's
// repositories and services on db: DB, Logger, NodeRepo, Users, Files,
// ContentTypes, ImageStyles, Taxonomy, Comments; the rest nil.
func Deps(db DB) modulekit.Deps {
	client := database.ClientFor(db.DB)
	logger := Logger()
	nodes := database.NewNodeRepository(client)
	return modulekit.Deps{
		DB:           db.DB,
		Logger:       logger,
		NodeRepo:     nodes,
		Users:        database.NewUserRepository(client.User, nil),
		Files:        database.NewFileRepository(client.File),
		ContentTypes: service.NewContentTypeService(database.NewContentTypeRepository(client), nodes, database.NewUnitOfWork(client), logger),
		ImageStyles:  service.NewImageStyleService(database.NewImageStyleRepository(client), logger),
		Taxonomy:     service.NewTaxonomyService(database.NewTaxonomyRepository(client), logger),
		Comments:     service.NewCommentsService(database.NewCommentRepository(client), logger),
	}
}

// CommentWriter - Deps' comments (Deps.Comments) as a test writes them:
// as on the site, each change tells the modules' hooks.
type CommentWriter interface {
	modulekit.CommentsService
	Create(ctx context.Context, c *modulekit.Comment) (*modulekit.Comment, error)
	Delete(ctx context.Context, id int) error
}

// Comments - the comments of d (a Deps of this package) a test writes.
func Comments(d modulekit.Deps) CommentWriter { return d.Comments.(CommentWriter) }
