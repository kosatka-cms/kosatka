package interfaces

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/internal/domain/service"
	"github.com/kosatka-cms/kosatka/internal/interfaces/form"
)

// typeByName - a content type service knowing one type, without fields.
type typeByName struct {
	service.ContentTypeService
	def entity.ContentTypeDef
}

func (s typeByName) ContentTypeByMachineName(_ context.Context, name string) (*entity.ContentTypeDef, error) {
	if name != s.def.MachineName {
		return nil, entity.ErrEntityNotFound
	}
	def := s.def
	return &def, nil
}

func (typeByName) FieldsByContentTypeID(context.Context, int) (entity.FieldInstanceDefs, error) {
	return nil, nil
}

// TestCreateDisabledType - a disabled type's create form and its submit
// answer 404, as an unknown type's do: the type takes no new content
// whoever may create it.
func TestCreateDisabledType(t *testing.T) {
	prev := errorPageRenderer
	t.Cleanup(func() { errorPageRenderer = prev })
	SetErrorPageRenderer(func(w http.ResponseWriter, _ *http.Request, status int) bool {
		w.WriteHeader(status)
		return true
	})

	c := &DynamicContentController{
		BaseController:     &BaseController{},
		contentTypeService: typeByName{def: entity.ContentTypeDef{ID: 1, MachineName: "event", Status: 0}},
		dynamicRepoFactory: func(entity.ContentTypeDef, entity.FieldInstanceDefs) entity.DynamicContentRepository { return nil },
	}
	for _, h := range []struct {
		name    string
		method  string
		handler http.HandlerFunc
	}{
		{"form", http.MethodGet, c.CreateForm},
		{"submit", http.MethodPost, c.CreateSubmit},
	} {
		r := httptest.NewRequest(h.method, "/create/event", nil)
		r.SetPathValue("type", "event")
		w := httptest.NewRecorder()
		h.handler(w, r)
		if w.Code != http.StatusNotFound {
			t.Errorf("%s: status %d, want 404", h.name, w.Code)
		}
	}
}

// TestSentBack - a form sent back with errors shows what was sent: the
// text typed; a new file kept for the next submit where the field takes
// the media library's, else the saved one and a note to choose it again
// (several images: the saved ones the form kept, in its order); a saved
// file ticked for deleting, ticked.
func TestSentBack(t *testing.T) {
	uploadOnly := map[string]any{"media_source": string(entity.MediaSourceUpload)}
	cf := contentForm{fields: entity.FieldInstanceDefs{
		{ID: 1, FieldType: entity.FieldTypeText},
		{ID: 2, FieldType: entity.FieldTypeImage},
		{ID: 3, FieldType: entity.FieldTypeImage, WidgetConfig: uploadOnly},
		{ID: 4, FieldType: entity.FieldTypeImage, WidgetConfig: uploadOnly, Config: map[string]any{"multiple": true}},
		{ID: 5, FieldType: entity.FieldTypeFile},
	}}
	saved := entity.DynamicContent{Values: map[int]any{1: "saved text", 2: 20, 3: 30, 4: []int{40, 41}, 5: 50}}
	f := &form.DynamicForm{FieldValues: map[int]string{1: "typed text", 2: "21", 3: "31", 4: "41,42", 5: "0"}}

	shown := sentBack(context.Background(), cf, f, saved)
	if shown.Values[1] != "typed text" {
		t.Errorf("text: %v", shown.Values[1])
	}
	if shown.Values[2] != 21 || f.SentFiles[2] != 21 {
		t.Errorf("a library field's new image: shown %v, kept %v", shown.Values[2], f.SentFiles[2])
	}
	if shown.Values[3] != 30 || f.FieldUploadErrors[3] == "" || f.SentFiles[3] != 0 {
		t.Errorf("an upload-only field's new image: shown %v, note %q, kept %v", shown.Values[3], f.FieldUploadErrors[3], f.SentFiles[3])
	}
	if got := entity.FileIDs(shown.Values[4]); len(got) != 1 || got[0] != 41 || f.FieldUploadErrors[4] == "" {
		t.Errorf("several, upload-only: shown %v, note %q", got, f.FieldUploadErrors[4])
	}
	if shown.Values[5] != 50 || !f.FileDeletes[5] {
		t.Errorf("a file ticked for deleting: shown %v, ticked %v", shown.Values[5], f.FileDeletes[5])
	}
}
