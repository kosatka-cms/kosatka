package files

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSaveUploadedDocument - documents of the policy's types are stored
// under their transliterated name; other types and disguised pages,
// scripts and programs are rejected.
func TestSaveUploadedDocument(t *testing.T) {
	s, tmp := newTestUserFiles(t)
	ctx := t.Context()

	f, h := uploadBody("Annual Report.PDF", []byte("%PDF-1.7\n1 0 obj\n"))
	url, kind, err := s.SaveUploadedFile(ctx, 7, "", f, h, 0, true)
	if err != nil {
		t.Fatal(err)
	}
	if kind != UserFileKindFile || url != "/uploads/u7/annual-report.pdf" {
		t.Fatalf("url %q kind %q", url, kind)
	}
	if b, _ := os.ReadFile(filepath.Join(tmp, "u7", "annual-report.pdf")); !strings.HasPrefix(string(b), "%PDF") {
		t.Fatal("file content not stored")
	}
	// Same name again: a unique one.
	f, h = uploadBody("Annual Report.pdf", []byte("%PDF-1.7\n"))
	if url, _, err := s.SaveUploadedFile(ctx, 7, "", f, h, 0, true); err != nil || url != "/uploads/u7/annual-report_1.pdf" {
		t.Fatalf("second upload: %q %v", url, err)
	}

	for name, content := range map[string]string{
		"page.html":   "<html><body>hi</body></html>",
		"evil.pdf":    "<!DOCTYPE html><html><script>alert(1)</script>",
		"image.txt":   "<svg xmlns='http://www.w3.org/2000/svg'><script/></svg>",
		"tool.pdf":    "MZ\x90\x00\x03",
		"run.txt":     "#!/bin/sh\nrm -rf /",
		"sheet.xlsx":  "PK\x03\x04",
		"noextension": "plain",
	} {
		f, h := uploadBody(name, []byte(content))
		if _, _, err := s.SaveUploadedFile(ctx, 7, "", f, h, 0, true); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	entries, _ := s.List(ctx, 7, "")
	if len(entries) != 2 {
		t.Errorf("rejected uploads left files: %+v", entries)
	}
	for _, e := range entries {
		if e.Kind != UserFileKindFile {
			t.Errorf("%s: kind %q", e.Name, e.Kind)
		}
	}
}
