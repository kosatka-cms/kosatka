package dao

import (
	"strings"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/ent/generated"
)

// ToContentTypeDefEntity converts a generated ContentTypeDef to its domain entity.
func ToContentTypeDefEntity(v *generated.ContentTypeDef) entity.ContentTypeDef {
	if v == nil {
		return entity.ContentTypeDef{}
	}
	return entity.ContentTypeDef{
		ID:                 v.ID,
		MachineName:        v.MachineName,
		Label:              v.Label,
		PathPattern:        v.PathPattern,
		Description:        v.Description,
		Status:             v.Status,
		DefaultCommentMode: entity.CommentMode(v.DefaultCommentMode),
		HideEmptyComments:  v.HideEmptyComments,
		DefaultPromote:     v.DefaultPromote,
		DefaultSticky:      v.DefaultSticky,
		DefaultPublishing:  entity.NodePublishing(v.DefaultPublishing),
		TitleDisplay:       entity.TitleDisplay(v.TitleDisplay),
		AltTitleField:      v.AltTitleField,
		RedirectField:      v.RedirectField,
		ShowSubmitted:      v.ShowSubmitted,
		ShowViewCount:      v.ShowViewCount,
		Revisions:          v.Revisions,
		RevisionsKeep:      v.RevisionsKeep,
		Embedded:           v.Embedded,
		ViewRoles:          splitViewRoles(v.ViewRoles),
		CreatedBy:          v.CreatedBy,
		CreatedAt:          v.CreatedAt,
		UpdatedAt:          v.UpdatedAt,
	}
}

// ToFieldStorageDefEntity converts a generated FieldStorageDef to its domain entity.
func ToFieldStorageDefEntity(v *generated.FieldStorageDef) entity.FieldStorageDef {
	if v == nil {
		return entity.FieldStorageDef{}
	}
	return entity.FieldStorageDef{
		ID:          v.ID,
		MachineName: v.MachineName,
		FieldType:   entity.ContentTypeFieldType(v.FieldType),
		Config:      v.Config,
		CreatedAt:   v.CreatedAt,
		UpdatedAt:   v.UpdatedAt,
	}
}

// ToFieldInstanceDefEntity converts a generated FieldInstanceDef to a flat
// domain entity: MachineName/FieldType/Config come from the FieldStorageDef
// edge, WidgetConfig from the instance's own config column. Panics if the
// edge isn't loaded - callers must query .WithFieldStorage().
func ToFieldInstanceDefEntity(f *generated.FieldInstanceDef) entity.FieldInstanceDef {
	if f == nil {
		return entity.FieldInstanceDef{}
	}
	storage := f.Edges.FieldStorage
	return entity.FieldInstanceDef{
		ID:               f.ID,
		ContentTypeID:    f.ContentTypeID,
		FieldStorageID:   f.FieldStorageID,
		MachineName:      storage.MachineName,
		Label:            f.Label,
		FieldType:        entity.ContentTypeFieldType(storage.FieldType),
		Required:         f.Required,
		HiddenInForm:     f.HiddenInForm,
		DefaultImageID:   f.DefaultImageID,
		ShowOnDetail:     f.ShowOnDetail,
		ShowLabel:        f.ShowLabel,
		RendersInBody:    f.RendersInBody,
		DetailImageStyle: f.DetailImageStyle,
		ListImageStyle:   f.ListImageStyle,
		DetailFormatter:  f.DetailFormatter,
		DetailLayout:     entity.ParseFieldLayout(f.DetailLayout),
		DateFormat:       f.DateFormat,
		SortOrder:        f.SortOrder,
		DisplayOrder:     f.DisplayOrder,
		Config:           storage.Config,
		WidgetConfig:     f.Config,
		ParentGroupID:    f.ParentGroupID,
		FormGroupID:      f.FormGroupID,
		UsableInForm:     f.UsableInForm,
		UsableInDetail:   f.UsableInDetail,
		CreatedAt:        f.CreatedAt,
		UpdatedAt:        f.UpdatedAt,
	}
}

// splitViewRoles - the role names of a content type's view_roles column
// (comma-separated, "" - none: everyone may view).
func splitViewRoles(s string) []string {
	var out []string
	for _, r := range strings.Split(s, ",") {
		if r = strings.TrimSpace(r); r != "" {
			out = append(out, r)
		}
	}
	return out
}
