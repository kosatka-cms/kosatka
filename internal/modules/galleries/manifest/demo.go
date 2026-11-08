package manifest

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"path"
	"strconv"
	"strings"

	"github.com/kosatka-cms/kosatka/internal/modules/galleries/ent/generated"
	"github.com/kosatka-cms/kosatka/modulekit/demokit"
)

// demoFS - the module's demo content by language: demo/<lang>.json.
//
//go:embed demo/*.json
var demoFS embed.FS

// demoContent - the gallery of the newest demo article: its title,
// description and its pictures (the first is the cover).
type demoContent struct {
	Title       string        `json:"title"`
	Description string        `json:"description"`
	Pictures    []demoPicture `json:"pictures"`
	// Node - a demo set's node (its key) the gallery is on; empty - the
	// newest demo article.
	Node string `json:"node"`
}

// demoPicture - a picture of the demo gallery: its title - a generated
// picture's ("Choosing a doctor") - or its title and a file of the demo
// set's own ({title, file}, demokit.SetPicture).
type demoPicture struct {
	Title string `json:"title"`
	File  string `json:"file"`
}

// UnmarshalJSON takes a title or {title, file}.
func (p *demoPicture) UnmarshalJSON(b []byte) error {
	if len(b) > 0 && b[0] == '"' {
		return json.Unmarshal(b, &p.Title)
	}
	type plain demoPicture
	return json.Unmarshal(b, (*plain)(p))
}

// The demo gallery's generated pictures, 1600×1000.
const demoPictureWidth, demoPictureHeight = 1600, 1000

// createDemo adds a gallery of sea pictures to the newest demo article (a
// demo set's: to its node) and switches galleries on for it. The pictures
// go where uploads do (galleries/<gallery ID>/ under the public files
// directory).
func createDemo(ctx context.Context, env demokit.Env) error {
	var content demoContent
	if err := demokit.Content(env, demoFS, &content); err != nil {
		return err
	}
	var nodeID int
	switch {
	case content.Node != "":
		id, err := env.Node(content.Node)
		if err != nil {
			return err
		}
		nodeID = id
	case len(env.Articles) > 0:
		nodeID = env.Articles[0]
	default:
		return nil
	}
	// The module's own client, in the install transaction.
	c := generated.NewClient(generated.Driver(demokit.Driver(env)))
	gallery, err := c.Gallery.Create().
		SetNid(nodeID).
		SetTitle(content.Title).
		SetDescription(content.Description).
		SetCreatedAt(env.Now).
		SetUpdatedAt(env.Now).
		Save(ctx)
	if err != nil {
		return err
	}
	dir := path.Join("galleries", strconv.Itoa(gallery.ID))
	for i, pic := range content.Pictures {
		title := pic.Title
		var fileID int
		var err error
		if pic.File != "" {
			fileID, err = demokit.SetPicture(ctx, env, path.Join(dir, fmt.Sprintf("demo-%d%s", i+1, strings.ToLower(path.Ext(pic.File)))), pic.File, title, env.Now)
		} else {
			// The palettes from the sixth on (demokit.ImageFile), named in demo/*.json.
			fileID, err = demokit.ImageFile(ctx, env, path.Join(dir, fmt.Sprintf("demo-%d.jpg", i+1)), demoPictureWidth, demoPictureHeight, i+5, title, env.Now)
		}
		if err != nil {
			return fmt.Errorf("picture %q: %w", title, err)
		}
		if err := c.GalleryItem.Create().
			SetGalleryID(gallery.ID).
			SetFid(fileID).
			SetTitle(title).
			SetWeight(i).
			SetIsCover(i == 0).
			SetCreatedAt(env.Now).
			Exec(ctx); err != nil {
			return err
		}
	}
	return c.GalleryNode.Create().SetNid(nodeID).Exec(ctx)
}
