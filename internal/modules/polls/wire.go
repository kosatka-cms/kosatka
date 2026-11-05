package polls

import (
	"github.com/kosatka-cms/kosatka/internal/modules/polls/manifest"
	"github.com/kosatka-cms/kosatka/modulekit"
	"github.com/kosatka-cms/kosatka/pkg/ratelimit"
)

// Wire - the module's service, controller (the poll content type's
// pages), vote routes, the "Poll" block's polls and [[poll:<id>]] in texts
// (modulekit).
func Wire(d modulekit.Deps) modulekit.Parts {
	polls := NewPollService(NewPollRepository(d.DB, d.NodeRepo, d.Users), d.Nodes, d.Logger)
	modulekit.SetPluginBlockTargets(manifest.PluginBlockPoll, polls.BlockTargets)
	modulekit.AddEmbedKind(embedKind(polls, d.NodeRepo, d.Templates))
	c := NewPollController(d.Base, PollDeps{
		PollService:  polls,
		NodesService: d.Nodes,
		NodeDeleter:  d.NodeDeleter,
	})
	return modulekit.Parts{
		Routes: func(r modulekit.Routes) {
			RegisterPollVoteRoutes(r.Mux, r.Store, c, ratelimit.NewRateLimiter(30, 10))
		},
		NodeTypes: map[modulekit.ContentType]modulekit.NodeTypeController{manifest.ContentType: c},
	}
}
