package migrations

// DropOrder - dropOrder for the external tests (sets_test.go).
var DropOrder = dropOrder

// DropRemovedTables, AtlasDriver - dropRemovedTables and atlasDriver for
// the external tests (generate_integration_test.go).
var (
	DropRemovedTables = dropRemovedTables
	AtlasDriver       = atlasDriver
)

// MoveResendKey, ShowLastCommentsTitle, ClearStarterLogo - moveResendKey,
// showLastCommentsTitle and clearStarterLogo for the external tests
// (update_core_integration_test.go).
var (
	MoveResendKey         = moveResendKey
	ShowLastCommentsTitle = showLastCommentsTitle
	ClearStarterLogo      = clearStarterLogo
)

// KeepNodePageReferences - keepNodePageReferences for the external tests
// (update_core_references_integration_test.go).
var KeepNodePageReferences = keepNodePageReferences
