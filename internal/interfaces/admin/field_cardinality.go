package admin

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// fieldCardinalityShownNodes - how many of the nodes blocking a change to
// a single value the confirmation page lists.
const fieldCardinalityShownNodes = 20

// fieldCardinalityNode - a node holding several values of the field, as the
// confirmation page links it.
type fieldCardinalityNode struct {
	ID    int
	Title string
	Type  string
}

// ContentTypeFieldCardinality renders GET
// /admin/config/content-types/{id}/fields/{field_id}/cardinality - the
// confirmation for switching a field between one and several values
// (service.ContentTypeService.SetFieldMultiple): what changes, the content
// types sharing the field and, when going to one value, the content that
// still has several (it blocks the change). Posts to
// ContentTypeFieldCardinalitySave.
func (c *Controller) ContentTypeFieldCardinality(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	contentType, field, ok := c.contentTypeField(w, r)
	if !ok {
		return
	}
	if !field.FieldType.AllowsMultiple() {
		c.RenderNotFound(w, r)
		return
	}
	fieldID := field.ID

	data := c.getAdminData(r)
	data["title"] = i18n.T(ctx, "Number of values of \"@field\": @type", "@field", field.Label, "@type", contentType.Label)
	data["pageSEO"] = entity.PageSEO{Title: i18n.T(ctx, "Number of values of @field - @type - Admin panel", "@field", field.Label, "@type", contentType.Label)}
	data["contentType"] = contentType
	markFieldSetMenu(data, contentType.Embedded)
	data["field"] = field
	data["activeTab"] = "fields"
	data["toMultiple"] = !field.Multiple()
	data["error"] = r.URL.Query().Get("error")

	if usage, err := c.contentTypeService.FieldStorageUsage(ctx, field.FieldStorageID); err != nil {
		c.LogFailure(err, "error checking field storage usage for the cardinality page", "fieldID", fieldID)
	} else {
		data["storageUsage"] = usage
	}
	if field.Multiple() {
		several, err := c.contentTypeService.NodesWithSeveralFieldValues(ctx, field.FieldStorageID)
		if err != nil {
			c.LogFailure(err, "error finding nodes with several values", "fieldID", fieldID)
			data["severalUnknown"] = true
		}
		var shown []fieldCardinalityNode
		for _, nid := range several[:min(len(several), fieldCardinalityShownNodes)] {
			n, err := c.nodeRepo.ByID(ctx, nid)
			if err != nil {
				c.LogFailure(err, "error loading a node with several values", "nodeID", nid)
				shown = append(shown, fieldCardinalityNode{ID: nid, Title: "#" + strconv.Itoa(nid)})
				continue
			}
			shown = append(shown, fieldCardinalityNode{ID: n.ID, Title: n.Title, Type: n.Type.ToString()})
		}
		data["severalCount"] = len(several)
		data["severalNodes"] = shown
	}

	c.RenderOrFail(w, r, "admin/content-types/field_cardinality.html", data)
}

// ContentTypeFieldCardinalitySave handles POST
// /admin/config/content-types/{id}/fields/{field_id}/cardinality: "multiple"
// = 1 - several values, 0 - one. Back to the field's page when done, to the
// confirmation with the reason when refused.
func (c *Controller) ContentTypeFieldCardinalitySave(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	contentType, field, ok := c.contentTypeField(w, r)
	if !ok {
		return
	}
	fieldID := field.ID
	fieldURL := "/admin/config/content-types/" + strconv.Itoa(contentType.ID) + "/fields/" + strconv.Itoa(fieldID)
	multiple := r.FormValue("multiple") == "1"
	if err := c.contentTypeService.SetFieldMultiple(ctx, field.FieldStorageID, multiple); err != nil {
		if errors.Is(err, service.ErrFieldHasSeveralValues) {
			http.Redirect(w, r, fieldURL+"/cardinality?error=several", http.StatusSeeOther)
			return
		}
		c.LogFailure(err, "error changing field cardinality", "fieldID", fieldID, "multiple", multiple)
		http.Redirect(w, r, fieldURL+"/cardinality?error=save", http.StatusSeeOther)
		return
	}
	// The View blocks and lists rendered in the cache read the field's
	// values from the table it no longer uses.
	c.invalidateContentCaches()
	http.Redirect(w, r, fieldURL+"/edit?cardinality=1", http.StatusSeeOther)
}
