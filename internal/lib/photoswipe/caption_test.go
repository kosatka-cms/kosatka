package photoswipe

import (
	"strings"
	"testing"
)

func TestCaptionHTMLBothFields(t *testing.T) {
	got := CaptionHTML("Title", "Description")
	if !strings.Contains(got, `<div class="pswp-caption-title">Title</div>`) {
		t.Fatalf("missing title block: %s", got)
	}
	if !strings.Contains(got, `<div class="pswp-caption-description">Description</div>`) {
		t.Fatalf("missing description block: %s", got)
	}
}

func TestCaptionHTMLEmptyWhenBothBlank(t *testing.T) {
	if got := CaptionHTML("  ", ""); got != "" {
		t.Fatalf("expected empty result, got %q", got)
	}
}

func TestCaptionHTMLDedupesIdenticalTitleAndDescription(t *testing.T) {
	got := CaptionHTML("Same text", "Same text")
	if strings.Count(got, "Same text") != 1 {
		t.Fatalf("expected description to be dropped when equal to title, got %q", got)
	}
	if !strings.Contains(got, "pswp-caption-title") || strings.Contains(got, "pswp-caption-description") {
		t.Fatalf("expected only the title block, got %q", got)
	}
}

func TestCaptionHTMLEscapesInput(t *testing.T) {
	got := CaptionHTML(`<script>alert(1)</script>`, "")
	if strings.Contains(got, "<script>") {
		t.Fatalf("expected HTML to be escaped, got %q", got)
	}
}

func TestCaptionHTMLOnlyDescription(t *testing.T) {
	got := CaptionHTML("", "Just a description")
	if strings.Contains(got, "pswp-caption-title") {
		t.Fatalf("did not expect a title block, got %q", got)
	}
	if !strings.Contains(got, "pswp-caption-description") {
		t.Fatalf("expected description block, got %q", got)
	}
}
