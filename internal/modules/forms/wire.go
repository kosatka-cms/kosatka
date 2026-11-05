package forms

import (
	"context"

	"github.com/kosatka-cms/kosatka/internal/modules/forms/manifest"
	"github.com/kosatka-cms/kosatka/modulekit"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
	"github.com/kosatka-cms/kosatka/pkg/ratelimit"
)

// Wire - forms, their elements and submissions: the service (a
// submission notifies the form's addresses by e-mail; files sent with
// forms go to the private directory, within the upload policy's size
// limit, and leave with their form), its actions in the action log, the
// "Form" block's choices and [[form:<id>]] in texts, the controller (the form content type's pages)
// and routes (modulekit).
func Wire(d modulekit.Deps) modulekit.Parts {
	webforms := NewWebformService(NewWebformRepository(d.DB, d.NodeRepo, d.Users), d.Nodes, d.Logger)
	webforms.SetNotifier(d.Mail.SendPlain)
	files := &WebformFileStore{Files: modulekit.PrivateFiles}
	webforms.SetFileStore(files)
	webforms.SetUploadLimit(func(ctx context.Context) int64 {
		if s, err := d.UploadSettings.Get(ctx); err == nil && s != nil {
			return int64(s.MaxFileSizeBytes)
		}
		return 0
	})
	d.NodeDeleter.AfterDelete(func(ctx context.Context, n modulekit.Node) {
		if n.Type != manifest.ContentType {
			return
		}
		if err := files.RemoveAll(ctx, n.ID, 0); err != nil {
			d.Logger.Warn("form files not removed", "nodeID", n.ID, "error", err)
		}
	})
	modulekit.SetPluginBlockTargets(manifest.PluginBlockWebform, webforms.BlockTargets)
	modulekit.AddEmbedKind(embedKind(webforms, d.NodeRepo, d.Templates))
	registerActions()
	c := NewWebformController(modulekit.NewBaseController(d.Base), webforms, d.Nodes, d.NodeDeleter)
	return modulekit.Parts{
		// A form takes anonymous submissions: rate limited (CAPTCHA on the
		// CAPTCHA settings page).
		Routes: func(r modulekit.Routes) {
			RegisterWebformRoutes(r.Mux, r.Store, c, ratelimit.NewRateLimiter(10, 3))
		},
		NodeTypes: map[modulekit.ContentType]modulekit.NodeTypeController{manifest.ContentType: c},
		// Files sent with forms go to the private files.
		Requirements: modulekit.CheckWritableStore(i18n.N("Private files (files sent with forms)"), modulekit.PrivateFiles),
	}
}
