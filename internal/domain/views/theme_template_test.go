package views

import (
	"html/template"
	"testing"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
)

// TestWithViewTexts - a theme template's list gets the display's header
// and footer texts around it, as on the View's page; none - the list as
// it is.
func TestWithViewTexts(t *testing.T) {
	list := template.HTML(`<ul class="cards"></ul>`)
	if got := WithViewTexts(entity.ViewDisplayDef{}, list); got != list {
		t.Errorf("no texts: %s", got)
	}
	got := WithViewTexts(entity.ViewDisplayDef{HeaderText: "<h2>Services</h2>", FooterText: "<p><a href=\"/all\">All</a></p>"}, list)
	want := `<div class="view-text view-header-text"><h2>Services</h2></div><ul class="cards"></ul><div class="view-text view-footer-text"><p><a href="/all">All</a></p></div>`
	if string(got) != want {
		t.Errorf("got %s\nwant %s", got, want)
	}
}
