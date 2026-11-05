package entity

// ViewSnapshot - the JSON stored in ViewDisplayDef.PublishedSnapshot by
// views.DefaultService.PublishDisplay: the effective settings and
// Filters/Sorts/Fields needed to render the public page.
// URLPath/Status/DisplayType/MachineName are not included - they always
// come from the live row (ViewByURLPath resolves by the live column; the
// rest are structural switches, not page content).
type ViewSnapshot struct {
	Settings ViewSnapshotSettings `json:"settings"`
	Filters  ViewFilterDefs       `json:"filters"`
	Sorts    ViewSortDefs         `json:"sorts"`
	Fields   ViewFieldDefs        `json:"fields"`
}

// ViewSnapshotSettings - the Settings bundle EffectiveSettings resolves,
// plus Label/MetaTitle/MetaDescription: never inherited from Default,
// but still published page content. Stored as resolved values so a
// snapshot never depends on a live Default row that may change later.
type ViewSnapshotSettings struct {
	DisplayStyle               ViewDisplayStyle `json:"display_style"`
	GridColumns                int              `json:"grid_columns"`
	OverflowAfter              int              `json:"overflow_after"`
	CarouselSlidesPerView      int              `json:"carousel_slides_per_view"`
	CarouselAutoplay           bool             `json:"carousel_autoplay"`
	CarouselLoop               bool             `json:"carousel_loop"`
	CarouselShowNavigation     bool             `json:"carousel_show_navigation"`
	CarouselShowPagination     bool             `json:"carousel_show_pagination"`
	CarouselLinkFieldStorageID *int             `json:"carousel_link_field_storage_id"`
	CarouselEffect             string           `json:"carousel_effect"`
	ShowPagination             bool             `json:"show_pagination"`
	PaginationAjax             bool             `json:"pagination_ajax"`
	ShowGlossary               bool             `json:"show_glossary"`
	GlossaryFieldStorageID     *int             `json:"glossary_field_storage_id"`
	PageSize                   int              `json:"page_size"`
	HeaderText                 string           `json:"header_text"`
	FooterText                 string           `json:"footer_text"`
	OverflowHeaderText         string           `json:"overflow_header_text"`
	CustomCSS                  string           `json:"custom_css"`
	RowTemplate                string           `json:"row_template"`
	GroupByField               string           `json:"group_by_field"`
	GroupDateFormat            string           `json:"group_date_format"`
	Label                      string           `json:"label"`
	MetaTitle                  string           `json:"meta_title"`
	MetaDescription            string           `json:"meta_description"`
}

// ApplyTo returns a copy of display with its Settings bundle (and
// Label/meta) replaced by s; all other fields pass through untouched.
// The snapshot counterpart of EffectiveSettings.
func (s ViewSnapshotSettings) ApplyTo(display ViewDisplayDef) ViewDisplayDef {
	display.DisplayStyle = s.DisplayStyle
	display.GridColumns = s.GridColumns
	display.OverflowAfter = s.OverflowAfter
	display.CarouselSlidesPerView = s.CarouselSlidesPerView
	display.CarouselAutoplay = s.CarouselAutoplay
	display.CarouselLoop = s.CarouselLoop
	display.CarouselShowNavigation = s.CarouselShowNavigation
	display.CarouselShowPagination = s.CarouselShowPagination
	display.CarouselLinkFieldStorageID = s.CarouselLinkFieldStorageID
	display.CarouselEffect = s.CarouselEffect
	display.ShowPagination = s.ShowPagination
	display.PaginationAjax = s.PaginationAjax
	display.ShowGlossary = s.ShowGlossary
	display.GlossaryFieldStorageID = s.GlossaryFieldStorageID
	display.PageSize = s.PageSize
	display.HeaderText = s.HeaderText
	display.FooterText = s.FooterText
	display.OverflowHeaderText = s.OverflowHeaderText
	display.CustomCSS = s.CustomCSS
	display.RowTemplate = s.RowTemplate
	display.GroupByField = s.GroupByField
	display.GroupDateFormat = s.GroupDateFormat
	display.Label = s.Label
	display.MetaTitle = s.MetaTitle
	display.MetaDescription = s.MetaDescription
	return display
}

// SnapshotSettingsOf - display's Settings bundle and Label/meta as a
// snapshot stores them (ApplyTo's counterpart).
func SnapshotSettingsOf(display ViewDisplayDef) ViewSnapshotSettings {
	return ViewSnapshotSettings{
		DisplayStyle:               display.DisplayStyle,
		GridColumns:                display.GridColumns,
		OverflowAfter:              display.OverflowAfter,
		CarouselSlidesPerView:      display.CarouselSlidesPerView,
		CarouselAutoplay:           display.CarouselAutoplay,
		CarouselLoop:               display.CarouselLoop,
		CarouselShowNavigation:     display.CarouselShowNavigation,
		CarouselShowPagination:     display.CarouselShowPagination,
		CarouselLinkFieldStorageID: display.CarouselLinkFieldStorageID,
		CarouselEffect:             display.CarouselEffect,
		ShowPagination:             display.ShowPagination,
		PaginationAjax:             display.PaginationAjax,
		ShowGlossary:               display.ShowGlossary,
		GlossaryFieldStorageID:     display.GlossaryFieldStorageID,
		PageSize:                   display.PageSize,
		HeaderText:                 display.HeaderText,
		FooterText:                 display.FooterText,
		OverflowHeaderText:         display.OverflowHeaderText,
		CustomCSS:                  display.CustomCSS,
		RowTemplate:                display.RowTemplate,
		GroupByField:               display.GroupByField,
		GroupDateFormat:            display.GroupDateFormat,
		Label:                      display.Label,
		MetaTitle:                  display.MetaTitle,
		MetaDescription:            display.MetaDescription,
	}
}

// WithSettingsOf - display with from's Settings bundle (inherited from a
// View's Default, seeded into a copy): the snapshot's fields but
// Label/meta, which are the display's own.
func (display ViewDisplayDef) WithSettingsOf(from ViewDisplayDef) ViewDisplayDef {
	s := SnapshotSettingsOf(from)
	s.Label, s.MetaTitle, s.MetaDescription = display.Label, display.MetaTitle, display.MetaDescription
	return s.ApplyTo(display)
}
