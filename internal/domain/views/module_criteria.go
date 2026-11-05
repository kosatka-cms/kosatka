package views

import (
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

// The sort criteria, filters and fields modules add to content Views
// (entity.ViewModuleSort…), registered at startup from the modules'
// Wire. A View keeps one by its key; while its module is off it is
// neither offered nor applied (a stored one is skipped).
var (
	moduleCriteriaMu sync.RWMutex
	moduleSorts      []entity.ViewModuleSort
	moduleFilters    []entity.ViewModuleFilter
	moduleFields     []entity.ViewModuleField
	// moduleEnabled - whether a module is on (the app's wiring sets it);
	// nil - every module is.
	moduleEnabled func(name string) bool
)

// SetModuleEnabled sets how the registry tells a module is on.
func SetModuleEnabled(fn func(name string) bool) {
	moduleCriteriaMu.Lock()
	defer moduleCriteriaMu.Unlock()
	moduleEnabled = fn
}

// checkModuleKey panics on a key that isn't "<module>.<name>" or is too
// long for the column, or is registered already: a module's mistake,
// found at startup.
func checkModuleKey(module, key string, taken bool) {
	name, ok := strings.CutPrefix(key, module+".")
	if module == "" || !ok || name == "" || len(key) > entity.ViewModuleKeyMaxLen {
		panic(fmt.Sprintf("views: module %q: criterion key %q must be \"%s.<name>\", at most %d characters", module, key, module, entity.ViewModuleKeyMaxLen))
	}
	if taken {
		panic(fmt.Sprintf("views: criterion key %q is registered twice", key))
	}
}

// AddModuleSort registers a module's sort criterion.
func AddModuleSort(s entity.ViewModuleSort) {
	moduleCriteriaMu.Lock()
	defer moduleCriteriaMu.Unlock()
	checkModuleKey(s.Module, s.Key, slices.ContainsFunc(moduleSorts, func(o entity.ViewModuleSort) bool { return o.Key == s.Key }))
	moduleSorts = append(moduleSorts, s)
}

// AddModuleFilter registers a module's filter.
func AddModuleFilter(f entity.ViewModuleFilter) {
	moduleCriteriaMu.Lock()
	defer moduleCriteriaMu.Unlock()
	checkModuleKey(f.Module, f.Key, slices.ContainsFunc(moduleFilters, func(o entity.ViewModuleFilter) bool { return o.Key == f.Key }))
	moduleFilters = append(moduleFilters, f)
}

// AddModuleField registers a module's field.
func AddModuleField(f entity.ViewModuleField) {
	moduleCriteriaMu.Lock()
	defer moduleCriteriaMu.Unlock()
	checkModuleKey(f.Module, f.Key, slices.ContainsFunc(moduleFields, func(o entity.ViewModuleField) bool { return o.Key == f.Key }))
	moduleFields = append(moduleFields, f)
}

// ResetModuleCriteria drops every registered criterion (a new wiring of
// the app registers them again).
func ResetModuleCriteria() {
	moduleCriteriaMu.Lock()
	defer moduleCriteriaMu.Unlock()
	moduleSorts, moduleFilters, moduleFields = nil, nil, nil
}

// enabledLocked - whether module is on (moduleCriteriaMu held).
func enabledLocked(module string) bool {
	return moduleEnabled == nil || moduleEnabled(module)
}

// enabledOf - the items of list whose module is on.
func enabledOf[T any](list []T, module func(T) string) []T {
	moduleCriteriaMu.RLock()
	defer moduleCriteriaMu.RUnlock()
	var out []T
	for _, item := range list {
		if enabledLocked(module(item)) {
			out = append(out, item)
		}
	}
	return out
}

// ModuleSorts (and ModuleFilters/ModuleFields) - the criteria of the
// modules that are on, in the order registered.
func ModuleSorts() []entity.ViewModuleSort {
	return enabledOf(moduleSorts, func(s entity.ViewModuleSort) string { return s.Module })
}

// ModuleFilters - the filters of the modules that are on (see
// ModuleSorts).
func ModuleFilters() []entity.ViewModuleFilter {
	return enabledOf(moduleFilters, func(f entity.ViewModuleFilter) string { return f.Module })
}

// ModuleFields - the fields of the modules that are on (see ModuleSorts).
func ModuleFields() []entity.ViewModuleField {
	return enabledOf(moduleFields, func(f entity.ViewModuleField) string { return f.Module })
}

// ModuleSort (and ModuleFilter/ModuleField) - the criterion of key, false
// when none is registered or its module is off.
func ModuleSort(key string) (entity.ViewModuleSort, bool) {
	return findByKey(ModuleSorts(), key, func(s entity.ViewModuleSort) string { return s.Key })
}

// ModuleFilter - the module's filter of key (see ModuleSort).
func ModuleFilter(key string) (entity.ViewModuleFilter, bool) {
	return findByKey(ModuleFilters(), key, func(f entity.ViewModuleFilter) string { return f.Key })
}

// ModuleField - the module's field of key (see ModuleSort).
func ModuleField(key string) (entity.ViewModuleField, bool) {
	return findByKey(ModuleFields(), key, func(f entity.ViewModuleField) string { return f.Key })
}

func findByKey[T any](list []T, key string, keyOf func(T) string) (T, bool) {
	for _, item := range list {
		if keyOf(item) == key {
			return item, true
		}
	}
	var zero T
	return zero, false
}

// IsModuleKey - whether a View row's target is a module's criterion
// ("<module>.<name>"; the core's builtins have no dot), on or off.
func IsModuleKey(key string) bool {
	return strings.Contains(key, ".")
}

// ModuleKeyPrefix - the prefix of every criterion key of module.
func ModuleKeyPrefix(module string) string {
	return module + "."
}
