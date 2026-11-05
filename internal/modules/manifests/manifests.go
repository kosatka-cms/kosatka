// Package manifests registers every module of the engine (its manifest:
// entity.Module and install steps), then those from outside it
// (modulekit.Register) - the one list of the modules the binary
// carries. Call Register before anything reads the registry: at
// the start of main, in a tool's main, in a test that needs the real
// modules.
package manifests

import (
	"sync"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	actionlog "github.com/kosatka-cms/kosatka/internal/modules/action_log/manifest"
	complaints "github.com/kosatka-cms/kosatka/internal/modules/complaints/manifest"
	errorreports "github.com/kosatka-cms/kosatka/internal/modules/error_reports/manifest"
	feeds "github.com/kosatka-cms/kosatka/internal/modules/feeds/manifest"
	forms "github.com/kosatka-cms/kosatka/internal/modules/forms/manifest"
	galleries "github.com/kosatka-cms/kosatka/internal/modules/galleries/manifest"
	languages "github.com/kosatka-cms/kosatka/internal/modules/languages/manifest"
	notfoundlog "github.com/kosatka-cms/kosatka/internal/modules/not_found_log/manifest"
	polls "github.com/kosatka-cms/kosatka/internal/modules/polls/manifest"
	privatemessages "github.com/kosatka-cms/kosatka/internal/modules/private_messages/manifest"
	ratings "github.com/kosatka-cms/kosatka/internal/modules/ratings/manifest"
	subscriptions "github.com/kosatka-cms/kosatka/internal/modules/subscriptions/manifest"
	videoembed "github.com/kosatka-cms/kosatka/internal/modules/video_embed/manifest"
	"github.com/kosatka-cms/kosatka/modulekit/manifestkit"
)

// All - the modules' entries, in the order of the Modules page.
func All() []entity.Module {
	return []entity.Module{
		polls.Module(),
		feeds.Module(),
		galleries.Module(),
		errorreports.Module(),
		complaints.Module(),
		forms.Module(),
		privatemessages.Module(),
		ratings.Module(),
		subscriptions.Module(),
		videoembed.Module(),
		actionlog.Module(),
		notfoundlog.Module(),
		languages.Module(),
	}
}

var once sync.Once

// Register registers every module, its install and data steps and its
// demo step (once).
func Register() {
	once.Do(func() {
		for _, m := range All() {
			entity.RegisterModule(m)
		}
		galleries.RegisterSteps()
		polls.RegisterSteps()
		forms.RegisterSteps()
		errorreports.RegisterSteps()
		complaints.RegisterSteps()
		feeds.RegisterSteps()
		privatemessages.RegisterSteps()
		ratings.RegisterSteps()
		subscriptions.RegisterSteps()
		videoembed.RegisterSteps()
		actionlog.RegisterSteps()
		notfoundlog.RegisterSteps()
		languages.RegisterSteps()
		// The modules from outside the engine (modulekit.Register, from
		// their packages' init), after the engine's own.
		external, steps := manifestkit.Registered()
		for _, m := range external {
			m.External = true
			entity.RegisterModule(m)
		}
		for _, s := range steps {
			s()
		}
	})
}
