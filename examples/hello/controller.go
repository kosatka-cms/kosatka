package hello

import (
	"net/http"

	"github.com/kosatka-cms/kosatka/modulekit"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// Controller - the greetings page.
type Controller struct {
	*modulekit.BaseController
	greetings *Repository
}

// Page handles GET /hello: the greetings, newest first.
func (c *Controller) Page(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := c.DBContext(r)
	defer cancel()
	list, err := c.greetings.List(ctx)
	if err != nil {
		c.RenderLoadFailure(w, r, err, "error listing greetings")
		return
	}
	title := i18n.T(r.Context(), "Hello")
	data := c.GetCommonData(r)
	data["title"] = title
	data["greetings"] = list
	data["pageSEO"] = modulekit.PageSEOForList(title, r.URL.Path)
	c.RenderOrFail(w, r, "hello/page.html", data)
}
