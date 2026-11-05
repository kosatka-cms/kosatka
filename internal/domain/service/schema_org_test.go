package service

import (
	"context"
	"encoding/json"
	"log/slog"
	"reflect"
	"testing"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/pkg/sitetime"
)

// schemaTemplates - the SEO templates in memory (no rules).
type schemaTemplates struct {
	entity.SEORuleRepository
	tpl entity.SEOTemplates
}

func (r schemaTemplates) SEOTemplates(context.Context) (entity.SEOTemplates, error) {
	return r.tpl, nil
}

func (r schemaTemplates) SaveSEOTemplate(context.Context, entity.SEOTemplate) error { return nil }

func (r schemaTemplates) SEORules(context.Context) (entity.SEORuleDefs, error) { return nil, nil }

// schemaJSON - js decoded.
func schemaJSON(t *testing.T, js string) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal([]byte(js), &out); err != nil {
		t.Fatalf("%v: %s", err, js)
	}
	return out
}

// TestNodeJSONLD - a node's markup: an article's headline, description,
// image and address, its dates in the site's zone, its author and the
// site as the publisher with its logo, its properties' templates; an
// event's name, its dates and place in an object, the site as its
// organizer; a type's choice over the default's, "none" - no markup;
// without the site's address - none either.
func TestNodeJSONLD(t *testing.T) {
	ctx := context.Background()
	entity.SetSiteDomain("https://example.com")
	t.Cleanup(func() { entity.SetSiteDomain("") })
	_ = sitetime.Set("UTC")
	repo := schemaTemplates{tpl: entity.SEOTemplates{
		entity.SEOTemplateDefaultScope: {SchemaType: "Article", SchemaProps: map[string]string{"keywords": "[node:field:tags]"}},
		entity.SEOTemplateTypeScope("event"): {SchemaType: "Event", SchemaProps: map[string]string{
			"startDate": "[node:field:date:iso]", "location.name": "[node:field:place]"}},
		entity.SEOTemplateTypeScope("page"): {SchemaType: entity.SchemaNone},
	}}
	s := NewSEOService(repo, repo, nil, nil, slog.New(slog.DiscardHandler))
	s.SetSite(func(context.Context) (*entity.SiteInformation, error) {
		return &entity.SiteInformation{SiteName: "Example", Logo: entity.File{URI: "/files/logo.png"},
			SchemaOrg: entity.SiteSchemaOrg{SameAs: []string{"https://t.me/example"}}}, nil
	})
	at := time.Date(2026, 10, 30, 9, 0, 0, 0, time.UTC)
	article := entity.DynamicContent{
		Node:   entity.Node{ID: 1, Type: "article", Title: "How themes work", Slug: "articles/themes", CreatedAt: at, UpdatedAt: at.Add(time.Hour), CreatedBy: entity.User{Username: "anna", Slug: "users/anna"}},
		Fields: entity.FieldInstanceDefs{{ID: 5, MachineName: "tags", FieldType: entity.FieldTypeText}},
		Values: map[int]any{5: "themes, design"},
	}
	got := schemaJSON(t, string(s.NodeJSONLD(ctx, article, entity.PageSEO{Description: "About themes", OGImage: "/files/cover.webp"})))
	want := map[string]any{
		"@context": "https://schema.org", "@type": "Article", "headline": "How themes work",
		"url": "https://example.com/articles/themes", "mainEntityOfPage": "https://example.com/articles/themes",
		"description": "About themes", "image": "https://example.com/files/cover.webp",
		"datePublished": "2026-10-30T09:00:00Z", "dateModified": "2026-10-30T10:00:00Z",
		"author":   map[string]any{"@type": "Person", "name": "anna", "url": "https://example.com/users/anna"},
		"keywords": "themes, design",
		"publisher": map[string]any{"@type": "Organization", "name": "Example", "url": "https://example.com/",
			"logo": map[string]any{"@type": "ImageObject", "url": "https://example.com/files/logo.png"}, "sameAs": []any{"https://t.me/example"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("article:\n%v\nwant\n%v", got, want)
	}

	event := entity.DynamicContent{
		Node: entity.Node{ID: 2, Type: "event", Title: "Meetup", Slug: "events/meetup", CreatedAt: at},
		Fields: entity.FieldInstanceDefs{{ID: 7, MachineName: "date", FieldType: entity.FieldTypeDate},
			{ID: 8, MachineName: "place", FieldType: entity.FieldTypeText}},
		Values: map[int]any{7: at.Add(48 * time.Hour), 8: "Library"},
	}
	got = schemaJSON(t, string(s.NodeJSONLD(ctx, event, entity.PageSEO{})))
	if got["@type"] != "Event" || got["name"] != "Meetup" || got["startDate"] != "2026-11-01T09:00:00Z" || got["datePublished"] != nil ||
		!reflect.DeepEqual(got["location"], map[string]any{"@type": "Place", "name": "Library"}) || got["organizer"] == nil {
		t.Errorf("event: %v", got)
	}

	page := entity.DynamicContent{Node: entity.Node{ID: 3, Type: "page", Title: "About", Slug: "about"}}
	if js := s.NodeJSONLD(ctx, page, entity.PageSEO{}); js != "" {
		t.Errorf("a type with none: %s", js)
	}
	entity.SetSiteDomain("")
	if js := s.NodeJSONLD(ctx, article, entity.PageSEO{}); js != "" {
		t.Errorf("no site address: %s", js)
	}
}

// TestSiteJSONLD - the front page's markup: the site, its search when
// SearchBox, a local business with its address and logo as its image.
func TestSiteJSONLD(t *testing.T) {
	entity.SetSiteDomain("https://example.com")
	t.Cleanup(func() { entity.SetSiteDomain("") })
	s := NewSEOService(schemaTemplates{}, schemaTemplates{}, nil, nil, slog.New(slog.DiscardHandler))
	s.SetSite(func(context.Context) (*entity.SiteInformation, error) {
		return &entity.SiteInformation{SiteName: "Bakery", Logo: entity.File{URI: "/files/logo.png"},
			SchemaOrg: entity.SiteSchemaOrg{Kind: entity.SchemaOrgLocalBusiness, Address: "Main st. 1", Phone: "+1 555", SearchBox: true}}, nil
	})
	got := schemaJSON(t, string(s.SiteJSONLD(context.Background())))
	graph, _ := got["@graph"].([]any)
	if len(graph) != 2 {
		t.Fatalf("graph: %v", got)
	}
	site, org := graph[0].(map[string]any), graph[1].(map[string]any)
	action, _ := site["potentialAction"].(map[string]any)
	if site["@type"] != "WebSite" || site["name"] != "Bakery" || action["target"] != "https://example.com/search?q={search_term_string}" {
		t.Errorf("site: %v", site)
	}
	if org["@type"] != "LocalBusiness" || org["address"] != "Main st. 1" || org["telephone"] != "+1 555" || org["image"] != "https://example.com/files/logo.png" {
		t.Errorf("who runs it: %v", org)
	}
}
