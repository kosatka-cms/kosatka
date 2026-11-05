package entity

import (
	"testing"
)

// TestContentTypeDefCreatable - only an enabled type takes new content.
func TestContentTypeDefCreatable(t *testing.T) {
	if !(ContentTypeDef{Status: 1}).Creatable() {
		t.Error("an enabled type isn't creatable")
	}
	if (ContentTypeDef{Status: 0}).Creatable() {
		t.Error("a disabled type is creatable")
	}
}
