package interfaces

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

// TestScalarFieldsOnNodePage - a number by its settings (an old one as
// stored), an e-mail address hidden from harvesters (or plain), a phone
// as a tel: link.
func TestScalarFieldsOnNodePage(t *testing.T) {
	root, _ := filepath.Abs(filepath.Join("..", ".."))
	t.Chdir(root)
	fields := entity.FieldInstanceDefs{
		{ID: 1, MachineName: "price", Label: "Price", FieldType: entity.FieldTypeNumber, ShowOnDetail: true, ShowLabel: true,
			Config: map[string]any{"number_kind": "integer", "suffix": " $"}},
		{ID: 2, MachineName: "rating", Label: "Rating", FieldType: entity.FieldTypeNumber, ShowOnDetail: true, ShowLabel: true},
		{ID: 3, MachineName: "mail", Label: "Mail", FieldType: entity.FieldTypeEmail, ShowOnDetail: true, ShowLabel: true},
		{ID: 4, MachineName: "public_mail", Label: "Public mail", FieldType: entity.FieldTypeEmail, ShowOnDetail: true, ShowLabel: true,
			Config: map[string]any{"email_plain": true}},
		{ID: 5, MachineName: "phone", Label: "Phone", FieldType: entity.FieldTypePhone, ShowOnDetail: true, ShowLabel: true},
	}
	dc := &entity.DynamicContent{Fields: fields, Values: map[int]any{1: 12500.0, 2: 4.7, 3: "info@example.com", 4: "press@example.com", 5: "+7 (495) 123-45-67"}}
	out := renderNodeSections(t, dc)
	for _, want := range []string{
		`Price:</span> 12,500 $</p>`,
		`Rating:</span> 4.7</p>`,
		`<a href="#" class="field-email" data-email="moc.elpmaxe@ofni">info [at] example.com</a>`,
		`<a href="mailto:press@example.com" class="field-email">press@example.com</a>`,
		`<a href="tel:&#43;74951234567" class="field-phone">&#43;7 (495) 123-45-67</a>`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("no %s in %s", want, out)
		}
	}
}
