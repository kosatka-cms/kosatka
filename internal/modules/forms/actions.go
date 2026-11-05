package forms

import (
	"context"
	"strconv"

	"github.com/kosatka-cms/kosatka/internal/modules/forms/manifest"
	"github.com/kosatka-cms/kosatka/modulekit"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// The forms' actions in the action log: their group and types. A form
// created, published, deleted… is the core's "Content" group
// (modulekit.RecordNodeCreated, RecordNodeSaved, the core's deletion).
const (
	ActionGroup             = "forms"
	ActionSubmissionsDelete = "forms.submissions_delete"
)

// registerActions adds the forms' group and actions to the action log.
func registerActions() {
	modulekit.AddActionGroup(modulekit.ActionGroup{Name: ActionGroup, Module: manifest.Name, Label: i18n.N("Forms"),
		Description: i18n.N("Deleting a form's submissions. Forms created, published or deleted are in the Content group.")})
	modulekit.AddActionType(modulekit.ActionType{Name: ActionSubmissionsDelete, Group: ActionGroup, Label: i18n.N("Form submissions deleted"), Tone: modulekit.ActionToneDelete})
}

// recordSubmissionsDeleted records that n of the form's submissions were
// deleted.
func recordSubmissionsDeleted(ctx context.Context, w Webform, n int) {
	modulekit.RecordAction(ctx, modulekit.Action{Type: ActionSubmissionsDelete, Object: modulekit.NodeActionObject(w.Node),
		Changes: []modulekit.ActionChange{{Field: i18n.T(ctx, "Deleted"), To: strconv.Itoa(n)}}})
}
