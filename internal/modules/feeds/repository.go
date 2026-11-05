package feeds

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/kosatka-cms/kosatka/internal/modules/feeds/ent/generated"
	"github.com/kosatka-cms/kosatka/internal/modules/feeds/ent/generated/feed"
	"github.com/kosatka-cms/kosatka/internal/modules/feeds/ent/generated/feedfieldmap"
	"github.com/kosatka-cms/kosatka/internal/modules/feeds/ent/generated/predicate"
	"github.com/kosatka-cms/kosatka/modulekit"
)

// EntFeedRepository - feeds / feed_field_maps, on the module's own ent
// schema (ent/).
type EntFeedRepository struct {
	db *sql.DB
}

// NewFeedRepository - the feeds' repository on db.
func NewFeedRepository(db *sql.DB) *EntFeedRepository {
	return &EntFeedRepository{db: db}
}

// client - the module's client for ctx: in its transaction, if any.
func (r *EntFeedRepository) client(ctx context.Context) *generated.Client {
	return generated.NewClient(generated.Driver(modulekit.Driver(ctx, r.db)))
}

func toFeed(row *generated.Feed) Feed {
	f := Feed{
		ID: row.ID, Label: row.Label, Path: row.Path, Format: ParseFeedFormat(row.Format), Status: row.Status,
		TermIDs: ParseFeedTermIDs(row.TermIds), ItemLimit: row.ItemLimit, Days: row.Days,
		Title: row.Title, Description: row.Description, Language: row.Language, ImageStyle: row.ImageStyle,
		CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}
	for _, t := range strings.Split(row.ContentTypes, ",") {
		if t = strings.TrimSpace(t); t != "" {
			f.ContentTypes = append(f.ContentTypes, t)
		}
	}
	return f
}

// Feeds - every feed with its field maps, by label.
func (r *EntFeedRepository) Feeds(ctx context.Context) ([]Feed, error) {
	client := r.client(ctx)
	rows, err := client.Feed.Query().Order(generated.Asc(feed.FieldLabel), generated.Asc(feed.FieldID)).All(ctx)
	if err != nil {
		return nil, err
	}
	var out []Feed
	for _, row := range rows {
		f := toFeed(row)
		if f.FieldMaps, err = fieldMaps(ctx, client, f.ID); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, nil
}

// FeedByID - the feed with its field maps (ErrEntityNotFound - none).
func (r *EntFeedRepository) FeedByID(ctx context.Context, id int) (*Feed, error) {
	return r.one(ctx, feed.ID(id))
}

// FeedByPath - the feed at path, with its field maps (ErrEntityNotFound -
// none).
func (r *EntFeedRepository) FeedByPath(ctx context.Context, path string) (*Feed, error) {
	return r.one(ctx, feed.Path(path))
}

func (r *EntFeedRepository) one(ctx context.Context, where ...predicate.Feed) (*Feed, error) {
	client := r.client(ctx)
	row, err := client.Feed.Query().Where(where...).Only(ctx)
	if err != nil {
		if generated.IsNotFound(err) {
			return nil, fmt.Errorf("%w: feed", modulekit.ErrEntityNotFound)
		}
		return nil, err
	}
	f := toFeed(row)
	if f.FieldMaps, err = fieldMaps(ctx, client, f.ID); err != nil {
		return nil, err
	}
	return &f, nil
}

func fieldMaps(ctx context.Context, client *generated.Client, feedID int) (FeedFieldMaps, error) {
	rows, err := client.FeedFieldMap.Query().
		Where(feedfieldmap.FeedID(feedID)).
		Order(generated.Asc(feedfieldmap.FieldContentType)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	var out FeedFieldMaps
	for _, m := range rows {
		out = append(out, FeedFieldMap{ContentType: m.ContentType, BodyField: m.BodyField, ImageField: m.ImageField, CategoryField: m.CategoryField})
	}
	return out, nil
}

// CreateFeed stores f with its field maps; returns its ID.
func (r *EntFeedRepository) CreateFeed(ctx context.Context, f Feed) (id int, err error) {
	err = modulekit.InTx(ctx, r.db, func(ctx context.Context) error {
		c := r.client(ctx)
		now := time.Now()
		row, err := c.Feed.Create().
			SetLabel(f.Label).SetPath(f.Path).SetFormat(string(f.Format)).SetStatus(f.Status).
			SetContentTypes(strings.Join(f.ContentTypes, ",")).SetTermIds(f.TermIDsText()).SetItemLimit(f.ItemLimit).SetDays(f.Days).
			SetTitle(f.Title).SetDescription(f.Description).SetLanguage(f.Language).SetImageStyle(f.ImageStyle).
			SetCreatedAt(now).SetUpdatedAt(now).
			Save(ctx)
		if err != nil {
			return err
		}
		id = row.ID
		return replaceFieldMaps(ctx, c, id, f.FieldMaps)
	})
	if err != nil {
		return 0, err
	}
	return id, nil
}

// UpdateFeed - see FeedRepository.UpdateFeed.
func (r *EntFeedRepository) UpdateFeed(ctx context.Context, f Feed) error {
	return modulekit.InTx(ctx, r.db, func(ctx context.Context) error {
		c := r.client(ctx)
		_, err := c.Feed.Update().Where(feed.ID(f.ID)).
			SetLabel(f.Label).SetPath(f.Path).SetFormat(string(f.Format)).SetStatus(f.Status).
			SetContentTypes(strings.Join(f.ContentTypes, ",")).SetTermIds(f.TermIDsText()).SetItemLimit(f.ItemLimit).SetDays(f.Days).
			SetTitle(f.Title).SetDescription(f.Description).SetLanguage(f.Language).SetImageStyle(f.ImageStyle).
			SetUpdatedAt(time.Now()).
			Save(ctx)
		if err != nil {
			return err
		}
		return replaceFieldMaps(ctx, c, f.ID, f.FieldMaps)
	})
}

func replaceFieldMaps(ctx context.Context, c *generated.Client, feedID int, maps FeedFieldMaps) error {
	if _, err := c.FeedFieldMap.Delete().Where(feedfieldmap.FeedID(feedID)).Exec(ctx); err != nil {
		return err
	}
	for _, m := range maps {
		err := c.FeedFieldMap.Create().
			SetFeedID(feedID).SetContentType(m.ContentType).SetBodyField(m.BodyField).SetImageField(m.ImageField).SetCategoryField(m.CategoryField).
			Exec(ctx)
		if err != nil {
			return err
		}
	}
	return nil
}

// DeleteFeed deletes the feed with its field maps.
func (r *EntFeedRepository) DeleteFeed(ctx context.Context, id int) error {
	return modulekit.InTx(ctx, r.db, func(ctx context.Context) error {
		c := r.client(ctx)
		if _, err := c.FeedFieldMap.Delete().Where(feedfieldmap.FeedID(id)).Exec(ctx); err != nil {
			return err
		}
		_, err := c.Feed.Delete().Where(feed.ID(id)).Exec(ctx)
		return err
	})
}
