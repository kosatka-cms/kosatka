package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/internal/interfaces"
)

// seoSchemaFromForm - the schema.org type the SEO form chose (one of
// entity.SchemaTypes, entity.SchemaNone where inherit allows it - a
// content type's; else "") and its properties' templates.
func seoSchemaFromForm(r *http.Request, inherit bool) (string, map[string]string) {
	name := strings.TrimSpace(r.FormValue("schema_type"))
	if name == entity.SchemaNone && inherit {
		return name, nil
	}
	def, ok := entity.SchemaTypeByName(name)
	if !ok {
		return "", nil
	}
	props := map[string]string{}
	for _, p := range def.Props {
		if v := strings.TrimSpace(r.FormValue("schema_prop." + def.Name + "." + p.Name)); v != "" {
			props[p.Name] = v
		}
	}
	return def.Name, props
}

// seoSchemaSample - the schema.org markup of ct's newest published item,
// as saved: the JSON indented, the item's address (for a test by Google);
// none - no item, or no markup.
type seoSchemaSample struct {
	JSON  string
	Title string
	URL   string
	// TestURL - Google's Rich Results Test of the item's page.
	TestURL string
}

// schemaSample - seoSchemaSample of ct (nil - none; a failure logged).
func (c *Controller) schemaSample(ctx context.Context, ct *entity.ContentTypeDef) *seoSchemaSample {
	if c.dynamic == nil || c.nodeRepo == nil {
		return nil
	}
	published, typ := 1, ct.MachineName
	nodes, _, err := c.nodeRepo.ListForAdmin(ctx, entity.AdminNodesFilter{Type: &typ, Status: &published,
		SortBy: entity.AdminNodesSortCreated, Desc: true, Limit: 1})
	if err != nil || nodes == nil || len(*nodes) == 0 {
		if err != nil {
			c.LogFailure(err, "the schema.org sample's item", "type", typ)
		}
		return nil
	}
	fields, err := c.contentTypeService.FieldsByContentTypeID(ctx, ct.ID)
	if err != nil {
		c.LogFailure(err, "the schema.org sample's fields", "type", typ)
		return nil
	}
	dc, err := service.NewDynamicContentService(c.dynamic(*ct, fields.SortBySortOrder())).ByNodeID(ctx, (*nodes)[0].ID)
	if err != nil || dc == nil {
		c.LogFailure(err, "the schema.org sample's item", "type", typ)
		return nil
	}
	js := c.seoService.NodeJSONLD(ctx, *dc, c.seoService.Apply(ctx, *dc, interfaces.PageSEO(dc)))
	if js == "" {
		return nil
	}
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, []byte(js), "", "  "); err != nil {
		return nil
	}
	page := strings.TrimSuffix(entity.NewSystemSettings().Domain, "/") + dc.Node.URL()
	return &seoSchemaSample{JSON: pretty.String(), Title: dc.Node.Title, URL: dc.Node.URL(),
		TestURL: "https://search.google.com/test/rich-results?url=" + url.QueryEscape(page)}
}
