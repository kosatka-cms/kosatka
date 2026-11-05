package manifest

import (
	"context"
	"embed"
	"fmt"
	"strings"
	"time"

	"github.com/kosatka-cms/kosatka/internal/modules/forms/ent"
	"github.com/kosatka-cms/kosatka/internal/modules/forms/ent/generated"
	"github.com/kosatka-cms/kosatka/modulekit/demokit"
	"github.com/kosatka-cms/kosatka/modulekit/manifestkit"
)

// demoFS - the module's demo content by language: demo/<lang>.json.
//
//go:embed demo/*.json
var demoFS embed.FS

// demoContent - the form: its title, description and confirmation (plain
// text),
// the submit button's label, its elements, the demo users' submissions
// (values by element machine name) and the title of its link in the
// footer menu.
type demoContent struct {
	Title        string           `json:"title"`
	Description  string           `json:"description"`
	Confirmation string           `json:"confirmation"`
	Submit       string           `json:"submit"`
	Elements     []demoElement    `json:"elements"`
	Submissions  []demoSubmission `json:"submissions"`
	Menu         string           `json:"menu"`
}

type demoElement struct {
	Name     string   `json:"name"` // the machine name
	Type     string   `json:"type"`
	Label    string   `json:"label"`
	Required bool     `json:"required"`
	Options  []string `json:"options"`
}

type demoSubmission struct {
	Author string            `json:"author"`
	Values map[string]string `json:"values"`
}

// demoSubmissionStep - the time between two demo submissions.
const demoSubmissionStep = 9 * time.Hour

// RegisterSteps registers the module's schema and its demo step
// (manifests.Register).
func RegisterSteps() {
	manifestkit.RegisterSchema(Name, ent.Schema)
	demokit.Register(Name, createDemo)
}

// createDemo adds the feedback form with a few submissions from the demo
// users, linked from the footer menu. It notifies nobody (no e-mail).
func createDemo(ctx context.Context, env demokit.Env) error {
	var content demoContent
	if err := demokit.Content(env, demoFS, &content); err != nil {
		return err
	}
	c := generated.NewClient(generated.Driver(demokit.Driver(env)))
	at := env.Now.Add(-time.Duration(len(content.Submissions)+1) * demoSubmissionStep)
	nodeID, err := demokit.CreateNode(ctx, env, demokit.Node{Type: string(ContentType), Title: content.Title, Author: env.CreatedBy, Comments: manifestkit.CommentModeHidden, At: at})
	if err != nil {
		return err
	}
	form, err := c.Webform.Create().
		SetNid(nodeID).
		SetDescription(content.Description).
		SetConfirmation(content.Confirmation).
		SetSubmitLabel(content.Submit).
		Save(ctx)
	if err != nil {
		return err
	}
	elements := map[string]int{}
	for i, e := range content.Elements {
		el, err := c.WebformElement.Create().
			SetWebformID(form.ID).
			SetMachineName(e.Name).
			SetType(e.Type).
			SetLabel(e.Label).
			SetRequired(e.Required).
			SetOptions(strings.Join(e.Options, "\n")).
			SetWeight(i).
			Save(ctx)
		if err != nil {
			return fmt.Errorf("element %q: %w", e.Name, err)
		}
		elements[e.Name] = el.ID
	}
	for i, s := range content.Submissions {
		user, err := env.User(s.Author)
		if err != nil {
			return err
		}
		sub, err := c.WebformSubmission.Create().
			SetWebformID(form.ID).
			SetUserID(user).
			SetCreatedAt(at.Add(time.Duration(i+1) * demoSubmissionStep)).
			Save(ctx)
		if err != nil {
			return err
		}
		for name, value := range s.Values {
			element, ok := elements[name]
			if !ok {
				return fmt.Errorf("submission of %s: no element %q", s.Author, name)
			}
			if err := c.WebformSubmissionValue.Create().SetSubmissionID(sub.ID).SetElementID(element).SetValue(value).Exec(ctx); err != nil {
				return err
			}
		}
	}
	return demokit.MenuLink(ctx, env, env.FooterMenu, content.Menu, "", nodeID, 30)
}
