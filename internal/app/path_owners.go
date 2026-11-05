package app

import (
	"context"
	"net/http"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/internal/domain/views"
	"github.com/kosatka-cms/kosatka/internal/interfaces"
)

// catchAllPattern - the route of every path no other route takes:
// content by its address, then the fallbacks (a module's page, a View
// page, a term page).
const catchAllPattern = "GET /"

// registerPathOwners registers what answers the core's addresses
// (interfaces.AddPathOwner): the engine's routes, content, View pages
// (with a "%" argument too), the term pages of vocabularies that have
// them.
func registerPathOwners(mux *http.ServeMux, nodes entity.NodesRepository, viewService views.Service, taxonomy service.TaxonomyService) {
	interfaces.AddPathOwner(interfaces.PathOwnerRoutes, func(ctx context.Context, path string) (bool, error) {
		return routeAnswers(ctx, mux, path)
	})
	// Content of any type takes its path, whoever asks.
	interfaces.AddPathOwner(interfaces.PathOwnerContent, func(ctx context.Context, path string) (bool, error) {
		return found(nodes.BySlug(entity.WithAllNodeTypes(ctx), path))
	})
	interfaces.AddPathOwner(interfaces.PathOwnerViews, func(ctx context.Context, path string) (bool, error) {
		if ok, err := found(viewService.ViewByURLPath(ctx, path)); ok || err != nil {
			return ok, err
		}
		v, _, err := viewService.ViewByURLArgument(ctx, path)
		return found(v, err)
	})
	interfaces.AddPathOwner(interfaces.PathOwnerTerms, func(ctx context.Context, path string) (bool, error) {
		term, err := taxonomy.TermBySlug(ctx, path)
		if ok, err := found(term, err); !ok || err != nil {
			return ok, err
		}
		return term.Vocabulary.PublicTermPages, nil
	})
}

// routeAnswers - whether one of the engine's own routes (not the
// catch-all) takes GET /path.
func routeAnswers(ctx context.Context, mux *http.ServeMux, path string) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "/"+path, nil)
	if err != nil {
		return false, err
	}
	_, pattern := mux.Handler(req)
	return pattern != "" && pattern != catchAllPattern, nil
}

// found - whether a lookup found its record: a missing one is no error.
func found[T any](v *T, err error) (bool, error) {
	if service.IsNotFound(err) {
		return false, nil
	}
	return err == nil && v != nil, err
}
