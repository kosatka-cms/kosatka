package manifests

import (
	"testing"

	entschema "entgo.io/ent/dialect/sql/schema"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/migrations"
)

// TestModulesManifest - module descriptions stay consistent: unique names,
// a node type, plugin block or permission owned by one module at most,
// and only plugin blocks the block picker knows.
func TestModulesManifest(t *testing.T) {
	Register()
	known := map[entity.PluginBlockID]bool{}
	for _, id := range entity.PluginBlocks() {
		known[id] = true
	}
	names := map[string]bool{}
	owner := map[string]string{}
	claim := func(kind, item, module string) {
		key := kind + ":" + item
		if prev, ok := owner[key]; ok {
			t.Errorf("%s %q belongs to both %s and %s", kind, item, prev, module)
		}
		owner[key] = module
	}
	for _, m := range entity.Modules {
		if names[m.Name] {
			t.Errorf("module %q listed twice", m.Name)
		}
		names[m.Name] = true
		if !m.External && (m.Category == "" || m.CategoryName() != m.Category) {
			t.Errorf("module %s: category %q, not one of entity.ModuleCategories", m.Name, m.Category)
		}
		for _, nt := range m.NodeTypes {
			claim("node type", nt.Name, m.Name)
		}
		for _, req := range m.Requires {
			if _, ok := entity.ModuleByName(req); !ok || req == m.Name {
				t.Errorf("module %s requires unknown or itself: %q", m.Name, req)
			}
		}
		for _, b := range m.PluginBlocks {
			if !known[b.ID] || b.Label == "" {
				t.Errorf("module %s: plugin block %q unknown or without a label", m.Name, b.ID)
			}
			claim("plugin block", string(b.ID), m.Name)
		}
		for _, p := range m.Permissions {
			if p.Label == "" {
				t.Errorf("module %s: permission %q without a label", m.Name, p.Key)
			}
			claim("permission", p.Key, m.Name)
		}
		for _, f := range m.CaptchaForms {
			claim("captcha form", f.Key, m.Name)
		}
		for _, f := range m.FileSources {
			claim("file source", f.Source, m.Name)
		}
		tables := map[string]bool{}
		for _, tb := range m.Tables {
			claim("table", tb, m.Name)
			tables[tb] = true
		}
		for _, d := range m.Data {
			if !tables[d.Table] {
				t.Errorf("module %s counts %q, a table it doesn't own (Tables)", m.Name, d.Table)
			}
		}
	}
	for _, f := range entity.CoreCaptchaForms {
		claim("captcha form", f.Key, "core")
	}
	for b, feature := range entity.FeaturePluginBlocks {
		claim("plugin block", string(b), feature)
	}
}

// TestModuleDataTablesExist - every table a module names (Tables, Data,
// FileSources, NodeFiles, LabelColumns) has an ent schema, the core's or
// the module's own (the schema migrations are generated from them), so
// the Modules page counts real tables; a module's Tables are its set's.
func TestModuleDataTablesExist(t *testing.T) {
	Register()
	schema := map[string]bool{}
	setTables := map[string]map[string]bool{}
	for _, s := range migrations.Sets() {
		setTables[s.Name] = map[string]bool{}
		for _, tb := range s.Tables {
			schema[tb.Name] = true
			setTables[s.Name][tb.Name] = true
		}
	}
	for _, m := range entity.Modules {
		tables := append([]string{}, m.Tables...)
		for _, f := range m.FileSources {
			tables = append(tables, f.Table)
		}
		for _, d := range m.Data {
			tables = append(tables, d.Table)
		}
		for _, f := range m.NodeFiles {
			tables = append(tables, f.Table)
			if f.Via != "" {
				tables = append(tables, f.Via)
			}
		}
		for _, l := range m.LabelColumns {
			tables = append(tables, l.Table)
		}
		for _, tb := range tables {
			if !schema[tb] {
				t.Errorf("module %s: table %q has no ent schema", m.Name, tb)
			}
		}
		for _, tb := range m.Tables {
			if !setTables[m.Name][tb] {
				t.Errorf("module %s: its table %q isn't in its set", m.Name, tb)
			}
		}
	}
}

// TestModuleNodeKeysCascade - a module's reference to a node doesn't block
// deleting it (ON DELETE CASCADE or SET NULL): deleting a node clears only
// the core's tables.
func TestModuleNodeKeysCascade(t *testing.T) {
	Register()
	owned := map[string]string{}
	for _, m := range entity.Modules {
		for _, tb := range m.Tables {
			owned[tb] = m.Name
		}
	}
	var tables []*entschema.Table
	for _, s := range migrations.Sets() {
		tables = append(tables, s.Tables...)
	}
	for _, tb := range tables {
		module, ok := owned[tb.Name]
		if !ok {
			continue
		}
		for _, fk := range tb.ForeignKeys {
			if fk.RefTable != nil && fk.RefTable.Name == "nodes" && fk.OnDelete != entschema.Cascade && fk.OnDelete != entschema.SetNull {
				t.Errorf("module %s: %s.%s references nodes without ON DELETE CASCADE or SET NULL (%q)", module, tb.Name, fk.Columns[0].Name, fk.OnDelete)
			}
		}
	}
}
