// Package modulekit - the module SDK: what the engine hands a module at
// startup (Deps) and what the module hands back (Parts), and everything
// else a module uses of the core: its controllers' base, the hooks, the
// core's types and helpers. A module's Wire builds its repositories,
// services and controllers from Deps and returns Parts - its routes, its
// content types' pages, its 404-path fallback, its uninstall step; the
// engine calls the Wire of each module it has (its own, and those
// registered with Register) and knows none by name.
//
// A module from its own repository imports this package, its
// subpackages and the engine's pkg/*, nothing else of the engine. The
// data types are the core's own (type aliases), so a module and the core
// speak the same types; the services and repositories a module gets are
// interfaces with what a module needs. The SDK may change before the
// engine's 1.0 (the documentation's "The module SDK").
package modulekit

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"

	"github.com/gorilla/sessions"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/internal/interfaces"
)

// Deps - the core's database handles and services a module builds on.
type Deps struct {
	// Base - what the module's controllers' base is built from
	// (NewBaseController); what the module itself needs of the core is
	// below.
	Base   BaseDeps
	DB     *sql.DB
	Logger *slog.Logger

	// Auth - who the user is and what they may.
	Auth AuthService
	// Templates - the site's templates.
	Templates TemplateService
	// SiteInformation - the site's name, e-mail and languages.
	SiteInformation SiteInformationRepository
	// RefreshBlocks rebuilds the blocks' cache (in the background): after
	// a change the cached blocks show that isn't a material's or a
	// comment's (those rebuild it themselves).
	RefreshBlocks func()

	Nodes        NodesService
	NodeRepo     NodesRepository
	NodeDeleter  NodeDeleter
	ContentTypes ContentTypeService
	Taxonomy     TaxonomyService
	Teasers      TeaserBuilder
	ImageStyles  ImageStyleService
	Comments     CommentsService
	FileUpload   FileUploader
	Files        FileRepository
	Users        UserRepository
	Mail         Mailer
	// UploadSettings - the site's upload policy (file size limit).
	UploadSettings UploadSettingsRepository
	// Redirects - the site's redirects (a module adding them).
	Redirects RedirectService
	// Roles - the site's roles (a module's choice of who may do
	// something).
	Roles interface {
		Roles(ctx context.Context) (Roles, error)
	}
	// Vocabularies - deleting a vocabulary nothing else uses (a module's
	// uninstall step).
	Vocabularies interface {
		DeleteVocabularyIfUnused(ctx context.Context, vid int) (bool, error)
	}
}

// Parts - what a wired module gives the core. Every field is optional.
type Parts struct {
	// Routes registers the module's routes; their mux answers 404 while
	// the module is off.
	Routes func(Routes)
	// NodeTypes - the module's content types with pages of their own
	// (the node dispatcher sends their nodes there).
	NodeTypes map[entity.ContentType]interfaces.NodeTypeController
	// Fallback answers a path no route matched (a feed, a forum section)
	// while the module is on: true - answered.
	Fallback func(w http.ResponseWriter, r *http.Request) bool
	// Uninstall - the module's own uninstall step, run when the module is
	// deleted on the Modules page, after its content and before its tables.
	Uninstall func(ctx context.Context) error
	// Requirements - the module's lines of the status report (checked
	// only while it is enabled).
	Requirements StatusCheck
	// UserContent - the module's content of a user that "delete the
	// account and all its content" (/admin/users) counts and deletes too.
	UserContent *UserContent
}

// UserContent - a module's content of a user (Parts.UserContent): Label
// names its count on the deletion's confirmation (a translation source
// with @count, "private messages: @count"), Count - how much the user has,
// Delete - deletes it, before the account.
type UserContent = service.ModuleUserContent

// Mux - where a module registers its routes (Go's net/http patterns).
type Mux interface {
	Handle(pattern string, handler http.Handler)
	HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request))
}

// Routes - what route registration gives a module.
type Routes struct {
	// Mux - the router, gated by the module's state.
	Mux   Mux
	Store *sessions.CookieStore
	// AdminOnly wraps a handler of the admin: signed in, with the admin
	// permission.
	AdminOnly func(http.Handler) http.Handler
}
