// Package modules lists what the modules (internal/modules/<name>) give
// the core at startup: their Wire and their files - one entry per module
// here, besides its manifest in manifests.All() (TestEveryModuleListed
// checks both lists agree); a module from outside the engine gives the
// same with modulekit.Register, and its interface catalogs too (the
// engine's modules are translated by the downloaded languages).
package modules

import (
	"io/fs"
	"sort"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	actionlog "github.com/kosatka-cms/kosatka/internal/modules/action_log"
	actionlogm "github.com/kosatka-cms/kosatka/internal/modules/action_log/manifest"
	"github.com/kosatka-cms/kosatka/internal/modules/complaints"
	complaintsm "github.com/kosatka-cms/kosatka/internal/modules/complaints/manifest"
	errorreports "github.com/kosatka-cms/kosatka/internal/modules/error_reports"
	errorreportsm "github.com/kosatka-cms/kosatka/internal/modules/error_reports/manifest"
	"github.com/kosatka-cms/kosatka/internal/modules/feeds"
	feedsm "github.com/kosatka-cms/kosatka/internal/modules/feeds/manifest"
	"github.com/kosatka-cms/kosatka/internal/modules/forms"
	formsm "github.com/kosatka-cms/kosatka/internal/modules/forms/manifest"
	"github.com/kosatka-cms/kosatka/internal/modules/galleries"
	galleriesm "github.com/kosatka-cms/kosatka/internal/modules/galleries/manifest"
	"github.com/kosatka-cms/kosatka/internal/modules/languages"
	languagesm "github.com/kosatka-cms/kosatka/internal/modules/languages/manifest"
	notfoundlog "github.com/kosatka-cms/kosatka/internal/modules/not_found_log"
	notfoundlogm "github.com/kosatka-cms/kosatka/internal/modules/not_found_log/manifest"
	"github.com/kosatka-cms/kosatka/internal/modules/polls"
	pollsm "github.com/kosatka-cms/kosatka/internal/modules/polls/manifest"
	privatemessages "github.com/kosatka-cms/kosatka/internal/modules/private_messages"
	privatemessagesm "github.com/kosatka-cms/kosatka/internal/modules/private_messages/manifest"
	"github.com/kosatka-cms/kosatka/internal/modules/ratings"
	ratingsm "github.com/kosatka-cms/kosatka/internal/modules/ratings/manifest"
	"github.com/kosatka-cms/kosatka/internal/modules/subscriptions"
	subscriptionsm "github.com/kosatka-cms/kosatka/internal/modules/subscriptions/manifest"
	videoembed "github.com/kosatka-cms/kosatka/internal/modules/video_embed"
	videoembedm "github.com/kosatka-cms/kosatka/internal/modules/video_embed/manifest"
	"github.com/kosatka-cms/kosatka/modulekit"
)

// module - what a module gives the core: its catalogs (a module from
// outside the engine: its translations/), its Wire and its files (its
// templates/ and static/, embedded).
type module struct {
	catalog fs.FS
	wire    func(modulekit.Deps) modulekit.Parts
	assets  fs.FS
}

// registry - every module, by name.
var registry = map[string]module{
	pollsm.Name:           {wire: polls.Wire, assets: polls.Assets},
	feedsm.Name:           {wire: feeds.Wire, assets: feeds.Assets},
	galleriesm.Name:       {wire: galleries.Wire, assets: galleries.Assets},
	errorreportsm.Name:    {wire: errorreports.Wire, assets: errorreports.Assets},
	complaintsm.Name:      {wire: complaints.Wire, assets: complaints.Assets},
	formsm.Name:           {wire: forms.Wire, assets: forms.Assets},
	privatemessagesm.Name: {wire: privatemessages.Wire, assets: privatemessages.Assets},
	ratingsm.Name:         {wire: ratings.Wire, assets: ratings.Assets},
	subscriptionsm.Name:   {wire: subscriptions.Wire, assets: subscriptions.Assets},
	videoembedm.Name:      {wire: videoembed.Wire, assets: videoembed.Assets},
	actionlogm.Name:       {wire: actionlog.Wire, assets: actionlog.Assets},
	notfoundlogm.Name:     {wire: notfoundlog.Wire, assets: notfoundlog.Assets},
	languagesm.Name:       {wire: languages.Wire, assets: languages.Assets},
}

// Wired - a module's name and the Parts its Wire returned.
type Wired struct {
	Name string
	modulekit.Parts
}

// all - the engine's modules (registry) and those from outside it
// (modulekit.Register), by name.
func all() map[string]module {
	out := make(map[string]module, len(registry))
	for name, m := range registry {
		out[name] = m
	}
	for _, r := range modulekit.Registered() {
		out[r.Module.Name] = module{catalog: r.Catalog, wire: r.Wire, assets: r.Assets}
	}
	return out
}

// Wire - every registered module (entity.Modules) wired from d, in the
// registry's order.
func Wire(d modulekit.Deps) []Wired {
	mods := all()
	out := make([]Wired, 0, len(entity.Modules))
	for _, m := range entity.Modules {
		if e, ok := mods[m.Name]; ok && e.wire != nil {
			out = append(out, Wired{Name: m.Name, Parts: e.wire(d)})
		}
	}
	return out
}

// Catalogs - the catalogs of the modules that have them (those from
// outside the engine), for the translator's chain.
func Catalogs() []fs.FS {
	mods := all()
	out := make([]fs.FS, 0, len(mods))
	for _, name := range Names() {
		if c := mods[name].catalog; c != nil {
			out = append(out, c)
		}
	}
	return out
}

// Assets - every module's embedded files (templates/, static/) by name,
// for paths.SetModuleAssets.
func Assets() map[string]fs.FS {
	mods := all()
	out := make(map[string]fs.FS, len(mods))
	for name, m := range mods {
		if m.assets != nil {
			out[name] = m.assets
		}
	}
	return out
}

// Catalog - a module's catalogs (nil - it has none).
func Catalog(name string) fs.FS { return all()[name].catalog }

// Names - the listed modules (the engine's and those from outside it),
// sorted.
func Names() []string {
	mods := all()
	names := make([]string, 0, len(mods))
	for n := range mods {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}
