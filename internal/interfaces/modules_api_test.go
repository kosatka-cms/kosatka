package interfaces

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
)

// TestStepBudget - a step of a long job gets half the server's write
// timeout (its answer must get out), at least a second.
func TestStepBudget(t *testing.T) {
	t.Cleanup(func() { SetTimeouts(30*time.Second, 15*time.Second) })
	for write, want := range map[time.Duration]time.Duration{15 * time.Second: 7500 * time.Millisecond, 60 * time.Second: 30 * time.Second, time.Second: time.Second} {
		SetTimeouts(30*time.Second, write)
		if got := StepBudget(); got != want {
			t.Errorf("write %v: budget %v, want %v", write, got, want)
		}
	}
}

// TestModuleNodeHooks - the node page, node form, content type form,
// account settings and profile form hooks of a module run only while it
// is on.
func TestModuleNodeHooks(t *testing.T) {
	saved := []any{nodePageHooks, nodeSettings, contentTypeSettings, accountSettingsSections, userSettingsSections}
	t.Cleanup(func() {
		nodePageHooks = saved[0].([]nodePageHook)
		nodeSettings = saved[1].([]NodeSettings)
		contentTypeSettings = saved[2].([]ContentTypeSettings)
		accountSettingsSections = saved[3].([]AccountSettingsSection)
		userSettingsSections = saved[4].([]UserSettingsSection)
		service.SetModules(map[string]bool{"example": true})
	})
	nodePageHooks, nodeSettings, contentTypeSettings, accountSettingsSections, userSettingsSections = nil, nil, nil, nil, nil
	var calls []string
	AddNodePage("example", func(context.Context, entity.Node) NodePageParts {
		calls = append(calls, "page")
		return NodePageParts{Data: 1, Links: []NodeLink{{URL: "/q"}}, Comments: true}
	})
	AddNodeSettings(NodeSettings{Module: "example",
		Load: func(context.Context, string, *entity.Node, url.Values) (any, error) {
			calls = append(calls, "node load")
			return 2, nil
		},
		Save: func(context.Context, entity.Node, *http.Request) error {
			calls = append(calls, "node save")
			return nil
		},
	})
	AddContentTypeSettings(ContentTypeSettings{Module: "example",
		Load: func(context.Context, entity.ContentTypeDef) (any, error) {
			calls = append(calls, "type load")
			return 3, nil
		},
		Save: func(context.Context, entity.ContentTypeDef, *http.Request) error {
			calls = append(calls, "type save")
			return nil
		},
	})
	AddAccountSettingsSection(AccountSettingsSection{Module: "example",
		Load: func(context.Context) (any, error) {
			calls = append(calls, "account load")
			return 4, nil
		},
		Save: func(context.Context, *http.Request) error {
			calls = append(calls, "account save")
			return nil
		},
	})
	AddUserSettingsSection(UserSettingsSection{Module: "example",
		Load: func(_ context.Context, u entity.User) (any, error) {
			calls = append(calls, "user load")
			return u.ID, nil
		},
		Save: func(context.Context, entity.User, *http.Request) error {
			calls = append(calls, "user save")
			return nil
		},
	})
	c := &BaseController{logger: slog.New(slog.DiscardHandler)}
	ctx := context.Background()
	r := httptest.NewRequest(http.MethodPost, "/", nil)
	run := func() {
		parts := moduleNodePage(ctx, entity.Node{ID: 1})
		if service.ModuleEnabled("example") && (parts.data["example"] != 1 || len(parts.links) != 1 || parts.comments != "example") {
			t.Errorf("node page: %+v", parts)
		}
		if !service.ModuleEnabled("example") && parts.comments != "" {
			t.Errorf("the module off, its comments: %+v", parts)
		}
		if got := c.loadNodeSettings(ctx, "article", nil, nil); service.ModuleEnabled("example") && got["example"] != 2 {
			t.Errorf("node settings: %v", got)
		}
		if got := c.LoadContentTypeSettings(ctx, entity.ContentTypeDef{}); service.ModuleEnabled("example") && got["example"] != 3 {
			t.Errorf("type settings: %v", got)
		}
		if got := c.LoadAccountSettingsSections(ctx); service.ModuleEnabled("example") && got["example"] != 4 {
			t.Errorf("account settings: %v", got)
		}
		if got := c.loadUserSettingsSections(ctx, entity.User{ID: 5}); service.ModuleEnabled("example") && got["example"] != 5 {
			t.Errorf("user settings: %v", got)
		}
		_ = saveNodeSettings(ctx, entity.Node{ID: 1}, r, true, false)
		_ = SaveContentTypeSettings(ctx, entity.ContentTypeDef{}, r)
		_ = SaveAccountSettingsSections(ctx, r)
		_ = saveUserSettingsSections(ctx, entity.User{ID: 5}, r)
	}
	service.SetModules(map[string]bool{"example": true})
	run()
	if want := "page,node load,type load,account load,user load,node save,type save,account save,user save"; strings.Join(calls, ",") != want {
		t.Errorf("module on: %v", calls)
	}
	calls = nil
	service.SetModules(map[string]bool{"example": false})
	run()
	if len(calls) != 0 {
		t.Errorf("module off: %v", calls)
	}
}

// TestSaveNodeSettingsShown - a node form's module sections are saved
// only when the form showed them; otherwise a new node gets their
// defaults and an existing one is left as it is.
func TestSaveNodeSettingsShown(t *testing.T) {
	saved := nodeSettings
	t.Cleanup(func() {
		nodeSettings = saved
		service.SetModules(map[string]bool{"example": true})
	})
	service.SetModules(map[string]bool{"example": true})
	nodeSettings = nil
	var calls []string
	AddNodeSettings(NodeSettings{Module: "example",
		Load: func(context.Context, string, *entity.Node, url.Values) (any, error) { return nil, nil },
		Save: func(context.Context, entity.Node, *http.Request) error {
			calls = append(calls, "save")
			return nil
		},
		Default: func(context.Context, entity.Node) error {
			calls = append(calls, "default")
			return nil
		},
	})
	ctx := context.Background()
	r := httptest.NewRequest(http.MethodPost, "/", nil)
	for _, c := range []struct{ shown, isNew bool }{{true, true}, {true, false}, {false, true}, {false, false}} {
		if err := saveNodeSettings(ctx, entity.Node{ID: 1}, r, c.shown, c.isNew); err != nil {
			t.Fatal(err)
		}
	}
	if got := strings.Join(calls, ","); got != "save,save,default" {
		t.Errorf("calls: %s, want save,save,default", got)
	}
}

// TestAddUserMenuCount - the modules' numbers on the user menu's toggle
// add up; nothing (0 or less) leaves it unset.
func TestAddUserMenuCount(t *testing.T) {
	data := map[string]any{}
	AddUserMenuCount(data, 0)
	if _, ok := data["userMenuCount"]; ok {
		t.Errorf("0 set the count: %v", data["userMenuCount"])
	}
	AddUserMenuCount(data, 2)
	AddUserMenuCount(data, 3)
	if data["userMenuCount"] != 5 {
		t.Errorf("the count: %v, want 5", data["userMenuCount"])
	}
}
