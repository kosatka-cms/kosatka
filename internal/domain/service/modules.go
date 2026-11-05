package service

import (
	"context"
	"sort"
	"sync"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

// The switched-off built-in modules and features, loaded at startup and
// on save, read on every request that touches one.
// disabledModules starts as the registered modules' defaults on first use
// (the registry is filled at startup, after package initialisation).
var (
	modulesMu       sync.RWMutex
	disabledModules map[string]bool
)

// defaultsLocked fills disabledModules with the defaults if it hasn't been
// set yet (modulesMu held for writing).
func defaultsLocked() {
	if disabledModules == nil {
		disabledModules = defaultDisabledModules()
	}
}

// defaultDisabledModules - the modules off without a state row
// (entity.Module.OffByDefault).
func defaultDisabledModules() map[string]bool {
	out := map[string]bool{}
	for _, m := range entity.Modules {
		if m.OffByDefault {
			out[m.Name] = true
		}
	}
	return out
}

// ModuleEnabled - whether the built-in module is on; unknown names are
// treated as enabled.
func ModuleEnabled(name string) bool {
	modulesMu.RLock()
	if disabledModules != nil {
		defer modulesMu.RUnlock()
		return !disabledModules[name]
	}
	modulesMu.RUnlock()
	modulesMu.Lock()
	defer modulesMu.Unlock()
	defaultsLocked()
	return !disabledModules[name]
}

// SetModules updates the states of the given modules and features: name →
// enabled; others keep theirs.
func SetModules(states map[string]bool) {
	modulesMu.Lock()
	defer modulesMu.Unlock()
	defaultsLocked()
	for name, enabled := range states {
		if enabled {
			delete(disabledModules, name)
		} else {
			disabledModules[name] = true
		}
	}
}

// FeatureEnabled - whether a feature switch (entity.Feature*) is on.
func FeatureEnabled(name string) bool {
	return ModuleEnabled(name)
}

// LoadModules reads the module states from repo and makes them current;
// a module without a row is on, unless it is OffByDefault.
func LoadModules(ctx context.Context, repo entity.ModuleRepository) error {
	states, err := repo.States(ctx)
	if err != nil {
		return err
	}
	for _, m := range entity.Modules {
		if _, ok := states[m.Name]; !ok {
			states[m.Name] = !m.OffByDefault
		}
	}
	SetModules(states)
	return nil
}

// NodeTypeEnabled - whether a node type is available: false for a type
// owned by a disabled module.
func NodeTypeEnabled(nodeType string) bool {
	module, ok := entity.ModuleOfNodeType(nodeType)
	return !ok || ModuleEnabled(module)
}

// DisabledNodeTypes - the node types of disabled modules, sorted.
func DisabledNodeTypes() []string {
	var out []string
	for _, nodeType := range entity.ModuleNodeTypes() {
		if !NodeTypeEnabled(nodeType) {
			out = append(out, nodeType)
		}
	}
	sort.Strings(out)
	return out
}

// PluginBlockEnabled - whether a plugin block is available: false for a
// block provided by a disabled module.
func PluginBlockEnabled(id entity.PluginBlockID) bool {
	module, ok := entity.ModuleOfPluginBlock(id)
	return !ok || ModuleEnabled(module)
}

// EnabledPluginBlocks - the plugin blocks of enabled modules (the block
// picker).
func EnabledPluginBlocks() []entity.PluginBlockID {
	out := make([]entity.PluginBlockID, 0, len(entity.PluginBlocks()))
	for _, id := range entity.PluginBlocks() {
		if PluginBlockEnabled(id) {
			out = append(out, id)
		}
	}
	return out
}

// pluginBlockTargets - for a plugin block that shows one thing chosen on
// its placement page: the things it can show (SetPluginBlockTargets).
var (
	pluginBlockTargetsMu sync.RWMutex
	pluginBlockTargets   = map[entity.PluginBlockID]func(ctx context.Context) ([]entity.PluginBlockTarget, error){}
)

// SetPluginBlockTargets makes the plugin block one that shows a thing
// chosen on its placement page (stored as its PluginConfig "target"),
// among those list returns (a module's wiring).
func SetPluginBlockTargets(id entity.PluginBlockID, list func(ctx context.Context) ([]entity.PluginBlockTarget, error)) {
	pluginBlockTargetsMu.Lock()
	defer pluginBlockTargetsMu.Unlock()
	pluginBlockTargets[id] = list
}

// PluginBlockTargets - the things the plugin block can show; ok=false for
// a block without a target.
func PluginBlockTargets(ctx context.Context, id entity.PluginBlockID) ([]entity.PluginBlockTarget, bool, error) {
	pluginBlockTargetsMu.RLock()
	list, ok := pluginBlockTargets[id]
	pluginBlockTargetsMu.RUnlock()
	if !ok {
		return nil, false, nil
	}
	targets, err := list(ctx)
	return targets, true, err
}

// pluginBlockSettings/pluginBlockData - a plugin block's settings on its
// placement page (SetPluginBlockSettings) and the data the cache refresh
// loads for it from them (SetPluginBlockData).
var (
	pluginBlockSettingsMu sync.RWMutex
	pluginBlockSettings   = map[entity.PluginBlockID]func(ctx context.Context) ([]entity.PluginBlockSetting, error){}
	pluginBlockData       = map[entity.PluginBlockID]func(ctx context.Context, p entity.BlockPlacementDef) (any, error){}
)

// SetPluginBlockSettings gives the plugin block settings, offered on its
// placement page and stored in its PluginConfig (wiring); list - called
// for every page and save, its options may come from the database.
func SetPluginBlockSettings(id entity.PluginBlockID, list func(ctx context.Context) ([]entity.PluginBlockSetting, error)) {
	pluginBlockSettingsMu.Lock()
	defer pluginBlockSettingsMu.Unlock()
	pluginBlockSettings[id] = list
}

// PluginBlockSettings - the plugin block's settings (none - a block
// without them).
func PluginBlockSettings(ctx context.Context, id entity.PluginBlockID) ([]entity.PluginBlockSetting, error) {
	pluginBlockSettingsMu.RLock()
	list, ok := pluginBlockSettings[id]
	pluginBlockSettingsMu.RUnlock()
	if !ok {
		return nil, nil
	}
	return list(ctx)
}

// SetPluginBlockData gives the plugin block data for its template: load,
// called by every cache refresh for each of its placements, its result
// the placement's PluginData (entity.BlockWithPlacement), as a View
// block's content is built then (templates can't query the database).
func SetPluginBlockData(id entity.PluginBlockID, load func(ctx context.Context, p entity.BlockPlacementDef) (any, error)) {
	pluginBlockSettingsMu.Lock()
	defer pluginBlockSettingsMu.Unlock()
	pluginBlockData[id] = load
}

// PluginBlockDataLoader - the plugin block's loader (SetPluginBlockData);
// ok=false for a block without one.
func PluginBlockDataLoader(id entity.PluginBlockID) (func(ctx context.Context, p entity.BlockPlacementDef) (any, error), bool) {
	pluginBlockSettingsMu.RLock()
	defer pluginBlockSettingsMu.RUnlock()
	load, ok := pluginBlockData[id]
	return load, ok
}
