package paths

import "testing"

// TestPublicFiles - defaults without a config; configured values are
// normalised (no trailing slash on the directory, slashes around the URL).
func TestPublicFiles(t *testing.T) {
	t.Cleanup(func() { SetPublicFiles("", "") })
	SetPublicFiles("", "")
	if PublicFilesDir() != DefaultPublicFilesDir || PublicFilesURL() != DefaultPublicFilesURL || PublicFilesRel() != "uploads/" {
		t.Errorf("defaults: %q %q %q", PublicFilesDir(), PublicFilesURL(), PublicFilesRel())
	}
	SetPublicFiles(" /srv/site/files/ ", "files")
	if PublicFilesDir() != "/srv/site/files" || PublicFilesURL() != "/files/" || PublicFilesRel() != "files/" {
		t.Errorf("configured: %q %q %q", PublicFilesDir(), PublicFilesURL(), PublicFilesRel())
	}
}
