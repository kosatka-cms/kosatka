// Package views - Views: lists of content or comments
// configured in the admin (filters, sorts, fields; displays: a page, a
// block, an attachment, a term's page, a profile tab), published as
// snapshots and run against the database. Service is the configuration and the queries;
// the field instances and rows they render with (BuildContent*,
// BuildComment*) and the theme's View templates (RenderThemeTemplate)
// are here too. The template service and the block cache (package
// service) render them: this package doesn't import service.
package views
