package forms

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"path"
	"strconv"
	"strings"

	"github.com/kosatka-cms/kosatka/pkg/filestore"
)

// WebformFileStore keeps the files sent with forms in the private files'
// store (modulekit.PrivateFiles, never served as it is):
// webform/<node id>/<element id>/<random>.<ext>. Paths it takes back are
// checked to stay under webform/.
type WebformFileStore struct {
	// Files - the private files' store.
	Files func() filestore.Store
}

// webformFileRoot - the store's part of the private files.
const webformFileRoot = "webform"

var errWebformFilePath = errors.New("webform file: bad path")

// Save writes data as a new file of the form's element; returns its path
// relative to the private files.
func (st WebformFileStore) Save(ctx context.Context, nodeID, elementID int, ext string, data []byte) (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	rel := path.Join(webformFileRoot, strconv.Itoa(nodeID), strconv.Itoa(elementID), hex.EncodeToString(b)+"."+ext)
	if _, err := st.Files().Put(ctx, rel, bytes.NewReader(data), filestore.PutOptions{Exclusive: true}); err != nil {
		return "", err
	}
	return rel, nil
}

// check - rel, if it is one of the store's paths.
func (st WebformFileStore) check(rel string) (string, error) {
	clean := path.Clean("/" + rel)[1:]
	if clean != rel || !strings.HasPrefix(clean, webformFileRoot+"/") {
		return "", errWebformFilePath
	}
	return clean, nil
}

// Open opens a stored file.
func (st WebformFileStore) Open(ctx context.Context, rel string) (filestore.File, filestore.Info, error) {
	key, err := st.check(rel)
	if err != nil {
		return nil, filestore.Info{}, err
	}
	return st.Files().Open(ctx, key)
}

// Remove deletes a stored file (a missing one is no error).
func (st WebformFileStore) Remove(ctx context.Context, rel string) error {
	key, err := st.check(rel)
	if err != nil {
		return err
	}
	return st.Files().Delete(ctx, key)
}

// RemoveAll deletes the files of a form's element, or of the whole form
// when elementID is 0.
func (st WebformFileStore) RemoveAll(ctx context.Context, nodeID, elementID int) error {
	if nodeID <= 0 {
		return fmt.Errorf("webform files: node %d", nodeID)
	}
	dir := path.Join(webformFileRoot, strconv.Itoa(nodeID))
	if elementID > 0 {
		dir = path.Join(dir, strconv.Itoa(elementID))
	}
	return st.Files().DeleteDir(ctx, dir)
}
