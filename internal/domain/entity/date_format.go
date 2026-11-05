package entity

import (
	"context"
	"strconv"
	"time"
)

// DateFormat - a named date format of the date formats catalog
// (/admin/config/date-formats): a pattern (internal/lib/dateformat) that date fields
// and Views fields pick by MachineName.
type DateFormat struct {
	ID          int
	MachineName string
	Label       string
	Pattern     string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// DefaultDateFormat - the format of a date with none chosen; can't be
// deleted.
const DefaultDateFormat = "short"

// DateTimeFormat - the format of a date with its time where the engine
// shows one (comments, messages, the admin's lists); without it the
// default format is used.
const DateTimeFormat = "datetime"

// DateFormatRepository - the date_formats table.
type DateFormatRepository interface {
	DateFormats(ctx context.Context) ([]DateFormat, error)
	DateFormatByID(ctx context.Context, id int) (*DateFormat, error)
	CreateDateFormat(ctx context.Context, f DateFormat) (int, error)
	UpdateDateFormat(ctx context.Context, f DateFormat) error
	DeleteDateFormat(ctx context.Context, id int) error
	// DateFormatUsage - the fields and Views fields that chose the
	// format, as readable labels.
	DateFormatUsage(ctx context.Context, machineName string) ([]string, error)
	// InstanceDateFormats - the formats chosen on date field instances:
	// "<content type>:<field storage id>" → format machine name.
	InstanceDateFormats(ctx context.Context) (map[string]string, error)
}

// InstanceDateFormatKey - the key of DateFormatRepository.
// InstanceDateFormats: a content type and a field storage.
func InstanceDateFormatKey(contentType string, fieldStorageID int) string {
	return contentType + ":" + strconv.Itoa(fieldStorageID)
}
