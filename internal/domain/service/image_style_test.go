package service

import (
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

// TestImageStyleVersion - the version follows what the derivative is
// made by: another size, mode, anchor, quality or format changes it, the
// label doesn't; the same settings give the same version.
func TestImageStyleVersion(t *testing.T) {
	base := entity.ImageStyleDef{Name: "cover", Label: "Cover", Width: 600, Height: 200, Mode: "fill", Anchor: "top", Quality: 85, Format: "webp"}
	v := ImageStyleVersion(base)
	if len(v) != 8 || ImageStyleVersion(base) != v {
		t.Fatalf("version %q, want 8 hex characters, stable", v)
	}
	relabelled := base
	relabelled.Label, relabelled.Description = "Gallery cover", "the cards"
	if ImageStyleVersion(relabelled) != v {
		t.Error("the label changed the version")
	}
	for name, change := range map[string]func(*entity.ImageStyleDef){
		"width":   func(d *entity.ImageStyleDef) { d.Width = 300 },
		"height":  func(d *entity.ImageStyleDef) { d.Height = 450 },
		"mode":    func(d *entity.ImageStyleDef) { d.Mode = "fit" },
		"anchor":  func(d *entity.ImageStyleDef) { d.Anchor = "center" },
		"quality": func(d *entity.ImageStyleDef) { d.Quality = 70 },
		"format":  func(d *entity.ImageStyleDef) { d.Format = "jpeg" },
	} {
		d := base
		change(&d)
		if ImageStyleVersion(d) == v {
			t.Errorf("%s: the version didn't change", name)
		}
	}
}
