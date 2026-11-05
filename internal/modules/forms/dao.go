package forms

import (
	"sort"

	"github.com/kosatka-cms/kosatka/internal/modules/forms/ent/generated"
)

// ToWebformEntity maps a form with its elements (ordered by weight); its
// node is the core's (the repository adds it).
func ToWebformEntity(w *generated.Webform) *Webform {
	if w == nil {
		return nil
	}
	out := &Webform{
		ID:              w.ID,
		Description:     w.Description,
		Confirmation:    w.Confirmation,
		ConfirmationURL: w.ConfirmationURL,
		SubmitLabel:     w.SubmitLabel,
		Closed:          w.Closed,
		NotifyEmail:     w.NotifyEmail,
		LimitPerUser:    w.LimitPerUser,
		MaxSubmissions:  w.MaxSubmissions,
		ClosesAt:        w.ClosesAt,
	}
	for _, e := range w.Edges.Elements {
		out.Elements = append(out.Elements, WebformElement{
			ID:               e.ID,
			MachineName:      e.MachineName,
			Type:             WebformElementType(e.Type),
			Label:            e.Label,
			Description:      e.Description,
			Required:         e.Required,
			Options:          ParseWebformOptions(e.Options),
			Weight:           e.Weight,
			VisibleIfElement: e.VisibleIfElement,
			VisibleIfValue:   e.VisibleIfValue,
			PrefillParam:     e.PrefillParam,
		})
	}
	sort.SliceStable(out.Elements, func(i, j int) bool {
		if out.Elements[i].Weight != out.Elements[j].Weight {
			return out.Elements[i].Weight < out.Elements[j].Weight
		}
		return out.Elements[i].ID < out.Elements[j].ID
	})
	return out
}
