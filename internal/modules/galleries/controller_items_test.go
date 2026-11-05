package galleries

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"testing"

	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// TestGalleryUploadErrorResponse - a check the upload failed is 400 with
// its message; any other failure is 500, its text kept from the visitor.
func TestGalleryUploadErrorResponse(t *testing.T) {
	ctx := context.Background()
	status, msg := galleryUploadErrorResponse(ctx, fmt.Errorf("uploading: %w", i18n.Errorf("The file is too large.")))
	if status != http.StatusBadRequest || msg != "The file is too large." {
		t.Errorf("a failed check: %d %q", status, msg)
	}
	status, msg = galleryUploadErrorResponse(ctx, errors.New("open /srv/files/galleries/1: disk full"))
	if status != http.StatusInternalServerError || msg != "Could not upload the image" {
		t.Errorf("an internal failure: %d %q", status, msg)
	}
}

// TestMainImageFirst - the image set as the main one goes first, the
// others keep their order after it, within the field's limit.
func TestMainImageFirst(t *testing.T) {
	for _, tc := range []struct {
		file    int
		current []int
		max     int
		want    []int
	}{
		{5, nil, 0, []int{5}},
		{5, []int{1, 2}, 0, []int{5, 1, 2}},
		{2, []int{1, 2, 3}, 0, []int{2, 1, 3}},
		{5, []int{1, 2, 3}, 3, []int{5, 1, 2}},
	} {
		if got := mainImageFirst(tc.file, tc.current, tc.max); !slices.Equal(got, tc.want) {
			t.Errorf("%d over %v (max %d): %v, want %v", tc.file, tc.current, tc.max, got, tc.want)
		}
	}
}
