// Package viewquery runs Views on the ent schema (entity.ViewQuery): a
// display's filters, sorts and pager over nodes or comments, its glossary
// bar and the "similar content" ranking.
package viewquery

import "github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated"

// Engine implements entity.ViewQuery.
type Engine struct {
	client *generated.Client
}

// New creates an Engine; a query inside a unit of work runs in its
// transaction (database.GetClient).
func New(client *generated.Client) *Engine {
	return &Engine{client: client}
}
