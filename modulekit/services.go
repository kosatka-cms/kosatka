package modulekit

import (
	"context"
	"mime/multipart"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/files"
	"github.com/kosatka-cms/kosatka/pkg/mailtemplate"
)

// The core's services and repositories a module gets (Deps): each only
// what a module needs of it - none of the core's own setup, nor a way
// past its paths (a material is deleted through NodeDeleter, never by
// its row).

// ContentTypeService - the content types and the materials' field values
// (Deps.ContentTypes).
type ContentTypeService interface {
	ContentTypeByMachineName(ctx context.Context, machineName string) (*ContentTypeDef, error)
	// ContentTypes - the content types, not the field sets.
	ContentTypes(ctx context.Context) (ContentTypeDefs, error)
	FieldsByContentTypeID(ctx context.Context, contentTypeID int) (FieldInstanceDefs, error)
	FieldValues(ctx context.Context, nodeID int, fields FieldInstanceDefs) (map[int]any, error)
	SetFieldValues(ctx context.Context, nodeID int, fields FieldInstanceDefs, values map[int]any, textFormats map[int]int) error
	// NodeTypeLabels - every node type's label by machine name: the
	// content types' own and the modules' node types' (translated).
	NodeTypeLabels(ctx context.Context) map[string]string
}

// TaxonomyService - the vocabularies and their terms (Deps.Taxonomy).
type TaxonomyService interface {
	Vocabularies(ctx context.Context) (TaxonomyVocabularies, error)
	VocabularyByID(ctx context.Context, id int) (*TaxonomyVocabulary, error)
	TermByID(ctx context.Context, id int) (*TaxonomyTerm, error)
	TermBySlug(ctx context.Context, slug string) (*TaxonomyTerm, error)
	// TermsByIDs - the terms with these ids, by id (a missing one is
	// absent).
	TermsByIDs(ctx context.Context, ids []int) (map[int]TaxonomyTerm, error)
	TermsByVocabularyID(ctx context.Context, vocabularyID int) (TaxonomyTerms, error)
	// NodeTermsByID - a material's terms (its terms index).
	NodeTermsByID(ctx context.Context, nodeID int) (TaxonomyTerms, error)
	// FieldTermIDsForNode - the terms a material's taxonomy fields hold.
	FieldTermIDsForNode(ctx context.Context, nodeID int) ([]int, error)
	// UpdateNodeTerms sets a material's terms index.
	UpdateNodeTerms(ctx context.Context, nodeID int, termIDs []int) error
}

// CommentsService - the materials' comments (Deps.Comments).
type CommentsService interface {
	ByID(ctx context.Context, id int) (*Comment, error)
	// Approve publishes a comment waiting for approval or marked as spam.
	Approve(ctx context.Context, id int) error
	// Hold takes a published comment back to waiting for approval for
	// reason (CommentReason*).
	Hold(ctx context.Context, id int, reason string) error
	// MarkSpam marks the comment as spam: shown to nobody.
	MarkSpam(ctx context.Context, id int) error
	// Delete removes the comment together with all its replies.
	Delete(ctx context.Context, id int) error
	ByNodeIDPaginated(ctx context.Context, nid int, status int8, page int) (Comments, int, error)
	CountByNodeID(ctx context.Context, nid int, status int8, rootOnly bool) (int, error)
	// OnCommentPosted registers the module's fn, called after a published
	// comment is created (a reply too: its ParentID) while the module is
	// enabled: a module telling of it. Its error is logged; the comment
	// stands. Called once, from Wire.
	OnCommentPosted(module string, fn func(ctx context.Context, c Comment) error)
	// OnNodeCommentsChanged registers the module's fn, called after a
	// comment of a material is created, updated or deleted while the
	// module is enabled: a module keeping its own counts of a material's
	// comments updates them. Its error is logged; the comment's change
	// stands. Called once, from Wire.
	OnNodeCommentsChanged(module string, fn func(ctx context.Context, nodeID int) error)
}

// NodeDeleter - deleting a material with everything of it: its fields'
// values, comments, files nothing else uses, the modules' data
// (Deps.NodeDeleter).
type NodeDeleter interface {
	// Delete deletes nodeID for good, whether in the trash or not.
	Delete(ctx context.Context, nodeID int) error
	// Remove takes nodeID out of sight as user's action: a content type's
	// item into the trash, a module's type's deleted (trashed - which).
	Remove(ctx context.Context, nodeID int, user *User) (trashed bool, err error)
	// AfterDelete adds a module's own cleanup of a deleted material
	// (files it keeps outside the database), run after every delete,
	// however the material was deleted: called once, from Wire.
	AfterDelete(fn func(ctx context.Context, n Node))
}

// TeaserBuilder - the materials' cards (Deps.Teasers).
type TeaserBuilder interface {
	// Build returns one card per material, in the same order.
	Build(ctx context.Context, nodes Nodes) ([]TeaserItem, error)
}

// ImageStyleService - the image styles (Deps.ImageStyles).
type ImageStyleService interface {
	ImageStyles(ctx context.Context) (ImageStyleDefs, error)
	ImageStyleByName(ctx context.Context, name string) (*ImageStyleDef, error)
}

// UploadResult - what an upload stored: its type, size and dimensions.
type UploadResult = files.UploadResult

// FileUploader - the uploads (Deps.FileUpload).
type FileUploader interface {
	// ImageTypes - the image MIME types the upload policy allows (empty -
	// every format the engine accepts, ImageFormatsLabel).
	ImageTypes(ctx context.Context) ([]string, error)
	// UploadImage saves an image to dir (relative to the public files,
	// e.g. a module's "galleries/12") under a unique name, checked by
	// policy (its zero limits - the site's); its public:// URI.
	UploadImage(ctx context.Context, file multipart.File, header *multipart.FileHeader, dir string, policy FieldUploadPolicy) (string, *UploadResult, error)
}

// FileRepository - the uploaded files (Deps.Files).
type FileRepository interface {
	// Create records an uploaded file (its URI, type, size, dimensions)
	// by createdBy.
	Create(ctx context.Context, uri, mimeType string, width, height, sizeBytes int, createdBy int) (*File, error)
	// ByIDs - the files with these ids, by id (a missing one is absent).
	ByIDs(ctx context.Context, ids []int) (map[int]File, error)
}

// UploadSettings - the site's upload policy.
type UploadSettings = entity.UploadSettings

// UploadSettingsRepository - the site's upload policy
// (Deps.UploadSettings).
type UploadSettingsRepository interface {
	Get(ctx context.Context) (*UploadSettings, error)
}

// UserRepository - the users (Deps.Users).
type UserRepository interface {
	ByID(ctx context.Context, id int) (*User, error)
	// ByIDs - the users with these ids, by id (a missing one is absent).
	ByIDs(ctx context.Context, ids []int) (map[int]User, error)
	// UsersWithRoles - the active users whose role is one of roleIDs
	// (Deps.Roles gives the roles), by id: whom to tell about what a role
	// does, such as a module's e-mail to it.
	UsersWithRoles(ctx context.Context, roleIDs []int) ([]User, error)
}

// SiteInformationRepository - the site's name, e-mail and languages
// (Deps.SiteInformation).
type SiteInformationRepository interface {
	Get(ctx context.Context) (*SiteInformation, error)
	Update(ctx context.Context, s *SiteInformation) error
	// SetLanguages saves the site's languages besides its default one.
	SetLanguages(ctx context.Context, languages []SiteLanguage) error
}

// RedirectService - the site's redirects (Deps.Redirects).
type RedirectService interface {
	// Create adds a redirect from the path from to to.
	Create(ctx context.Context, from, to string) error
	// GetRedirect - where fromPath redirects to.
	GetRedirect(fromPath string) (toPath string, ok bool)
}

// Mailer - the site's e-mail (Deps.Mail): with e-mail off nothing is
// sent. Send sends one of the site's emails (AddMail) by its key: its
// template (the Mail page's text) with data's placeholders, unless it is
// switched off there; SendPlain - a message as given.
type Mailer interface {
	SendPlain(to, subject, body string) error
	Send(ctx context.Context, key, to string, data mailtemplate.TokenData) error
}
