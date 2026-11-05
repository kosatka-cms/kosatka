package service

import (
	"context"
	"strconv"
	"time"

	"github.com/kosatka-cms/kosatka/internal/domain/entity"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
)

// The copyright block (entity.PluginBlockCopyright, blocks/copyright.html):
// "© <year> <holder>" in a theme's footer - a block, so a site edits or
// removes it. The year is the current one (from the first year when
// set), the holder the site's name unless set.

// maxCopyrightHolder - the holder's length.
const maxCopyrightHolder = 200

// RegisterCopyrightBlock gives the copyright block its settings (wiring).
func RegisterCopyrightBlock() {
	SetPluginBlockSettings(entity.PluginBlockCopyright, func(ctx context.Context) ([]entity.PluginBlockSetting, error) {
		return []entity.PluginBlockSetting{
			{Key: entity.CopyrightHolderKey, Label: i18n.T(ctx, "Copyright holder (empty - the site's name)"), Type: entity.PluginBlockSettingText, Max: maxCopyrightHolder},
			{Key: entity.CopyrightSinceKey, Label: i18n.T(ctx, "First year (0 - the current year only)"), Type: entity.PluginBlockSettingNumber,
				Default: "0", Min: 0, Max: time.Now().Year()},
		}, nil
	})
}

// CopyrightYears - the block's years: the current one, "<since>-<now>"
// from an earlier first year.
func CopyrightYears(since string, now time.Time) string {
	year := now.Year()
	if first, err := strconv.Atoi(since); err == nil && first > 0 && first < year {
		return strconv.Itoa(first) + "-" + strconv.Itoa(year)
	}
	return strconv.Itoa(year)
}
