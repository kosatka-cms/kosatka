package feeds

import (
	"context"
	"log/slog"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/modulekit"
	"github.com/kosatka-cms/kosatka/modulekit/testkit"
)

func TestFeedAdminPagesRender(t *testing.T) {
	testkit.UseEngineRoot(t)
	feed := Feed{ID: 3, Label: "Dzen feed", Path: "feeds/dzen.xml", Format: FeedFormatDzen, Status: 1,
		ContentTypes: []string{"news"}, ItemLimit: 100, ImageStyle: "feed_jpeg",
		FieldMaps: FeedFieldMaps{{ContentType: "news", ImageField: "cover_image", CategoryField: "type_tag"}}}

	list := testkit.RenderNamed(t, "content", map[string]any{"feeds": []Feed{feed}, "error": "delete"}, "admin/feeds.html", "admin/alerts.html", "admin/icons.html")
	for _, want := range []string{`href="/admin/config/feeds/3"`, `<code>/feeds/dzen.xml</code>`, "Dzen (full text)", "<code>news</code>", ">enabled<", "The feed could not be deleted."} {
		if !strings.Contains(list, want) {
			t.Errorf("list: missing %q", want)
		}
	}

	types := []feedTypeFields{{
		Type:       modulekit.ContentTypeDef{MachineName: "news", Label: "News"},
		TextFields: modulekit.FieldInstanceDefs{{MachineName: "content", Label: "Text"}},
		Images:     modulekit.FieldInstanceDefs{{MachineName: "cover_image", Label: "Picture"}},
		Taxonomies: modulekit.FieldInstanceDefs{{MachineName: "type_tag", Label: "News type"}},
	}, {Type: modulekit.ContentTypeDef{MachineName: "article", Label: "Articles"}}}
	data := map[string]any{"feed": feed, "types": types, "formats": FeedFormats, "siteLanguage": "de",
		"imageStyles": modulekit.ImageStyleDefs{{Name: "feed_jpeg", Label: "Feed", Format: "jpeg"}}}
	form := testkit.RenderNamed(t, "content", data, "admin/feed_edit.html", "admin/alerts.html")
	for _, want := range []string{
		`action="/admin/config/feeds/3"`,
		`value="news" checked`,
		`<option value="cover_image" selected>Picture (cover_image)</option>`,
		`<option value="type_tag" selected>News type (type_tag)</option>`,
		`<option value="feed_jpeg" selected>Feed (feed_jpeg, JPEG)</option>`,
		`name="body_article"`,
		`name="language" class="form-control admin-max-xs" value="" placeholder="de"`,
	} {
		if !strings.Contains(form, want) {
			t.Errorf("form: missing %q", want)
		}
	}
	if strings.Contains(form, `value="article" checked`) {
		t.Error("article must not be checked")
	}
}

type fakeTypeService struct {
	modulekit.ContentTypeService
	fakeFeedContentTypes
}

func (f fakeTypeService) ContentTypes(ctx context.Context) (modulekit.ContentTypeDefs, error) {
	return f.fakeFeedContentTypes.ContentTypes(ctx)
}

func (f fakeTypeService) FieldsByContentTypeID(ctx context.Context, id int) (modulekit.FieldInstanceDefs, error) {
	return f.fakeFeedContentTypes.FieldsByContentTypeID(ctx, id)
}

type fakeImageStyles struct{ modulekit.ImageStyleService }

func (fakeImageStyles) ImageStyles(context.Context) (modulekit.ImageStyleDefs, error) {
	return nil, nil
}

// TestFeedSaveKeepsInput - a feed the form can't save comes back in the
// form with what was typed and why; nothing is saved.
func TestFeedSaveKeepsInput(t *testing.T) {
	testkit.UseEngineRoot(t)
	repo := &fakeFeedRepo{}
	feeds := NewFeedService(repo, &fakeFeedNodes{}, fakeFeedContentTypes{}, pathOwners(map[string]string{"rss.xml": "routes"}), slog.New(slog.DiscardHandler))
	store, cookies, _ := testkit.Session(t)
	deps := modulekit.BaseDeps{Templates: testkit.Templates(), Logger: testkit.Logger(), Store: store,
		Auth: testkit.Auth{UserID: 1}, Users: testkit.Users{Known: map[int]*modulekit.User{1: {ID: 1}}}}
	c := NewFeedController(modulekit.NewBaseController(deps), feeds, nil, nil, fakeTypeService{}, fakeImageStyles{})
	req := testkit.PostForm("/admin/config/feeds", url.Values{"label": {"News feed"}, "path": {"rss.xml"}, "content_types": {"news"}, "title": {"Typed title"}}, cookies)
	rec := httptest.NewRecorder()
	c.FeedSave(rec, req)
	body := rec.Body.String()
	for _, want := range []string{`value="News feed"`, `value="rss.xml"`, `value="Typed title"`, `value="news" checked`, "This address is already used"} {
		if !strings.Contains(body, want) {
			t.Errorf("the form again: no %q (%d)", want, rec.Code)
		}
	}
	if len(repo.feeds) != 0 {
		t.Errorf("saved: %v", repo.feeds)
	}
}
