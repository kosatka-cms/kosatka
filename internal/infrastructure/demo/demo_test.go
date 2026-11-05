package demo

import (
	"bytes"
	"image/jpeg"
	"testing"
	"testing/fstest"
)

func TestPicture(t *testing.T) {
	data, err := Picture(320, 180, len(palettes)) // past the palettes: wraps around
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := jpeg.DecodeConfig(bytes.NewReader(data))
	if err != nil || cfg.Width != 320 || cfg.Height != 180 {
		t.Errorf("picture: %+v %v", cfg, err)
	}
}

// TestLoad - a language's own file, English for one without it, an error
// for broken JSON.
func TestLoad(t *testing.T) {
	fsys := fstest.MapFS{
		"demo/en.json": {Data: []byte(`{"title":"Hello"}`)},
		"demo/ru.json": {Data: []byte(`{"title":"Привет"}`)},
		"demo/xx.json": {Data: []byte(`{`)},
	}
	for lang, want := range map[string]string{"ru": "Привет", "en": "Hello", "de": "Hello"} {
		var v struct{ Title string }
		if err := Load(fsys, lang, &v); err != nil || v.Title != want {
			t.Errorf("%s: %q %v", lang, v.Title, err)
		}
	}
	var v struct{ Title string }
	if err := Load(fsys, "xx", &v); err == nil {
		t.Error("broken JSON must fail")
	}
}

// TestContent - a demo set's content over the module's file, strictly: a
// key the step doesn't know fails.
func TestContent(t *testing.T) {
	fsys := fstest.MapFS{"demo/en.json": {Data: []byte(`{"title":"Hello"}`)}}
	type content struct {
		Title string `json:"title"`
	}
	var v content
	if err := Content(Env{Lang: "en"}, fsys, &v); err != nil || v.Title != "Hello" {
		t.Errorf("the module's file: %q %v", v.Title, err)
	}
	v = content{}
	if err := Content(Env{Lang: "en", Data: []byte(`{"title":"Set"}`)}, fsys, &v); err != nil || v.Title != "Set" {
		t.Errorf("the set's: %q %v", v.Title, err)
	}
	if err := Content(Env{Data: []byte(`{"title":"Set","titel":"typo"}`)}, fsys, &v); err == nil {
		t.Error("an unknown key: no error")
	}
}
