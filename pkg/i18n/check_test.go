package i18n

import (
	"errors"
	"testing"
)

// TestCheckTranslation - a translation keeps the source's placeholders
// (@count may go; ":name" after a letter or "@host" in an address aren't
// placeholders) and only its tags, as they are; a "<" or ">" a browser
// reads as text is text, a tag not closed isn't.
func TestCheckTranslation(t *testing.T) {
	link := `<a href=":login">Log in</a> to post comments`
	for _, c := range []struct {
		source, text, placeholder, tag string
	}{
		{"Hello, @name!", "Привет, @name!", "", ""},
		{"Hello, @name!", "Привет!", "@name", ""},
		{"@count comment", "один комментарий", "", ""},
		{link, `<a href=":login">Войдите</a>, чтобы писать`, "", ""},
		{link, `<a  href=":login" >Войдите</a>, чтобы писать`, "", ""},
		{link, `<a href="/login">Войдите</a> :login`, "", `<a href="/login">`},
		{link, `<a href=":login" onclick="x()">Войдите</a>`, "", `<a href=":login" onclick="x()">`},
		{"Save", "<script>alert(1)</script>Сохранить", "", "<script>"},
		{"Save", "Сохранить <b", "", "<"},
		{"Save", "Сохранить </b", "", "<"},
		{"Save", "<!-- x -->Сохранить", "", "<!-- x -->"},
		{"Ratio", "a > b, b < c", "", ""},
		{`Not "<" or ">"`, `Не "<" и не ">"`, "", ""},
		{"[[block:<name>]] a block", "[[block:<имя>]] блок", "", ""},
		{"Format: <code>from -> to</code>", "Формат: <code>откуда -> куда</code>", "", ""},
		{"Tokens: <code>{{field:machine_name}}</code>", "Токены: <code>{{field:машинное_имя}}</code>", "", ""},
		{"Write to admin@example.com", "Пишите на почту сайта", "", ""},
		{"@count of :total", "@count из", ":total", ""},
		{"See %title", "См. %title", "", ""},
	} {
		err := CheckTranslation(c.source, c.text)
		var te *TranslationError
		switch {
		case c.placeholder == "" && c.tag == "":
			if err != nil {
				t.Errorf("%q → %q: %v", c.source, c.text, err)
			}
		case !errors.As(err, &te) || te.Placeholder != c.placeholder || te.Tag != c.tag:
			t.Errorf("%q → %q: %v, want placeholder %q tag %q", c.source, c.text, err, c.placeholder, c.tag)
		}
	}
}
