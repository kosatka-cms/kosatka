package testkit

import (
	"github.com/kosatka-cms/kosatka/internal/config"
	"github.com/kosatka-cms/kosatka/internal/infrastructure/filestorage"
	"github.com/kosatka-cms/kosatka/modulekit"
)

// FileStorageConfig - the config a file storage's Public and Private get
// on a site whose public files are in publicStorage ("local", or the
// storage's name), with sections as the config file's files.<section>
// (e.g. {"s3": {"bucket": "site"}}); Decode reads the environment over
// them, as on a site.
func FileStorageConfig(publicStorage string, sections map[string]any) modulekit.FileStorageConfig {
	files := config.FilesConfig{Drivers: sections}
	return filestorage.NewConfig(publicStorage, "public, max-age=31536000", files.DecodeDriver)
}
