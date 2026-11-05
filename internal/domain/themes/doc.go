// Package themes - the themes: their manifests (theme.yaml, InfoFile;
// a subtheme's merged with its base's, LoadInfoFor), the list the
// Appearance page shows (Available, Invalid), their settings
// (SettingsService), the blocks a theme comes with (PlaceBlocks), the
// catalog of themes kept in their own repositories (Catalog) and
// themes/themes.lock (ReadLock). The template service, the blocks, the
// admin and internal/app use it; it doesn't import package service (the
// versioned static URLs are package assets', a fallback's logging
// package fallback's).
package themes
