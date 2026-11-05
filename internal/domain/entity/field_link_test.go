package entity

import (
	"encoding/json"
	"slices"
	"testing"
)

// TestParseHosts - the "Allowed sites" input: separated by spaces, commas
// or lines, an address cut to its host, lower case, without "www.", each
// once; anything with a path left out.
func TestParseHosts(t *testing.T) {
	got := ParseHosts(" YouTube.com, www.vk.example\nhttps://www.Video.example/watch?v=1 youtube.com bad/path .dot. ")
	want := []string{"youtube.com", "vk.example", "video.example", "dot"}
	if !slices.Equal(got, want) {
		t.Errorf("ParseHosts = %v, want %v", got, want)
	}
}

// TestHostAllowed - a field without allowed sites takes any; with them, a
// site or its subdomain only.
func TestHostAllowed(t *testing.T) {
	open := FieldInstanceDef{FieldType: FieldTypeLink}
	if !open.HostAllowed("whatever.example") {
		t.Error("no allowed sites: a site refused")
	}
	f := FieldInstanceDef{FieldType: FieldTypeLink, Config: map[string]any{"allowed_hosts": []any{"video.example"}}}
	for host, want := range map[string]bool{
		"video.example":         true,
		"m.video.example":       true,
		"badvideo.example":      false,
		"video.example.example": false,
		"":                      false,
	} {
		if got := f.HostAllowed(host); got != want {
			t.Errorf("HostAllowed(%q) = %v, want %v", host, got, want)
		}
	}
}

// TestLinkParts - a link's text (its title, else its address) and host
// (lower case, without "www."; none for what isn't an http(s) address);
// a field's title setting (unknown - none); a layout (unknown - all).
func TestLinkParts(t *testing.T) {
	if got := (Link{URL: "https://WWW.Video.Example/x"}).Host(); got != "video.example" {
		t.Errorf("Host = %q", got)
	}
	if got := (Link{URL: "javascript:alert(1)"}).Host(); got != "" {
		t.Errorf("Host of a script = %q", got)
	}
	if got := (Link{URL: "https://a.example", Title: "A"}).Text(); got != "A" {
		t.Errorf("Text = %q", got)
	}
	if got := (FieldInstanceDef{Config: map[string]any{"link_title": "sometimes"}}).LinkTitle(); got != LinkTitleNone {
		t.Errorf("LinkTitle of an unknown setting = %q", got)
	}
	if ParseFieldLayout("featured") != FieldLayoutFeatured || ParseFieldLayout("other") != FieldLayoutAll {
		t.Error("ParseFieldLayout")
	}
}

// TestLinkSnapshot - a link field's links go into a revision's snapshot
// and come back as they were, whatever the cardinality.
func TestLinkSnapshot(t *testing.T) {
	f := FieldInstanceDef{ID: 1, MachineName: "trailer", FieldType: FieldTypeLink}
	links := []Link{{URL: "https://a.example", Title: "A"}, {URL: "https://b.example"}}
	snap := SnapshotOf(DynamicContent{Fields: FieldInstanceDefs{f}, Values: map[int]any{1: links}})
	if raw, _ := json.Marshal(snap.Fields["trailer"]); string(raw) != `[{"url":"https://a.example","title":"A"},{"url":"https://b.example"}]` {
		t.Errorf("snapshot %s", raw)
	}
	values, _ := snap.Values(FieldInstanceDefs{f})
	if got := Links(values[1]); !slices.Equal(got, links) {
		t.Errorf("back %v", got)
	}
	if empty := SnapshotOf(DynamicContent{Fields: FieldInstanceDefs{f}, Values: map[int]any{1: []Link{}}}); len(empty.Fields) != 0 {
		t.Errorf("no links in the snapshot: %v", empty.Fields)
	}
}
