package filestore

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"path"
	"sort"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// S3Config - an S3-compatible bucket: AWS S3, Yandex Object Storage,
// Google Cloud Storage (interoperability), Cloudflare R2, MinIO…
type S3Config struct {
	// Endpoint - the storage's address, e.g. https://storage.yandexcloud.net
	// (http:// - without TLS, for a local MinIO).
	Endpoint string
	// Region - the bucket's region ("" - the endpoint's default).
	Region string
	Bucket string
	// Prefix - the folder of the bucket the store keeps its files in
	// ("" - the bucket's root): several sites can share a bucket.
	Prefix string
	// AccessKey and SecretKey - the credentials.
	AccessKey, SecretKey string
	// PathStyle - address the bucket as <endpoint>/<bucket> instead of
	// <bucket>.<endpoint> (MinIO and some providers need it).
	PathStyle bool
	// CacheControl - the files' Cache-Control when the bucket serves them
	// itself ("" - none).
	CacheControl string
}

// S3 keeps the files as objects of a bucket: the key is the object's
// name after the prefix. A folder is a name prefix ending in "/"; Mkdir
// stores an empty object of that name so that an empty folder exists.
type S3 struct {
	client       *minio.Client
	bucket       string
	prefix       string // "" or ending in "/"
	cacheControl string
}

// NewS3 returns the store of a bucket. It doesn't contact the storage;
// the first operation does.
func NewS3(cfg S3Config) (*S3, error) {
	u, err := url.Parse(cfg.Endpoint)
	if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") {
		return nil, fmt.Errorf("filestore: the S3 endpoint %q isn't an http(s) address", cfg.Endpoint)
	}
	if cfg.Bucket == "" {
		return nil, errors.New("filestore: no S3 bucket")
	}
	if cfg.AccessKey == "" || cfg.SecretKey == "" {
		return nil, errors.New("filestore: no S3 access key or secret key")
	}
	lookup := minio.BucketLookupAuto
	if cfg.PathStyle {
		lookup = minio.BucketLookupPath
	}
	client, err := minio.New(u.Host, &minio.Options{
		Creds:        credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure:       u.Scheme == "https",
		Region:       cfg.Region,
		BucketLookup: lookup,
	})
	if err != nil {
		return nil, fmt.Errorf("filestore: S3 client: %w", err)
	}
	prefix := strings.Trim(cfg.Prefix, "/")
	if prefix != "" {
		if !validPath(prefix) {
			return nil, fmt.Errorf("filestore: the S3 prefix %q isn't a folder name", cfg.Prefix)
		}
		prefix += "/"
	}
	return &S3{client: client, bucket: cfg.Bucket, prefix: prefix, cacheControl: cfg.CacheControl}, nil
}

// String - where the store keeps the files: s3://<bucket>/<prefix>.
func (s *S3) String() string { return "s3://" + s.bucket + "/" + s.prefix }

func (s *S3) object(key string) string { return s.prefix + key }

// folder - the name prefix of the folder dir ("" - the root).
func (s *S3) folder(dir string) string {
	if dir == "" {
		return s.prefix
	}
	return s.prefix + dir + "/"
}

func (s *S3) key(object string) string { return strings.TrimPrefix(object, s.prefix) }

// notFound reports whether err is the storage's "no such object".
func notFound(err error) bool {
	resp := minio.ToErrorResponse(err)
	return resp.StatusCode == http.StatusNotFound || resp.Code == "NoSuchKey"
}

// wrap turns the storage's "no such object" into ErrNotExist.
func wrap(key string, err error) error {
	if notFound(err) {
		return fmt.Errorf("%s: %w", key, ErrNotExist)
	}
	return err
}

// inline - the media types a browser may show in place; the bucket
// serves everything else as a download (Content-Disposition).
func inline(contentType string) bool {
	return strings.HasPrefix(contentType, "image/") || contentType == "application/pdf"
}

// countingReader counts the bytes read through it.
type countingReader struct {
	r io.Reader
	n int64
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += int64(n)
	return n, err
}

// remaining - the bytes left in r when it can tell (-1 otherwise).
func remaining(r io.Reader) int64 {
	sk, ok := r.(io.Seeker)
	if !ok {
		return -1
	}
	cur, err := sk.Seek(0, io.SeekCurrent)
	if err != nil {
		return -1
	}
	end, err := sk.Seek(0, io.SeekEnd)
	if err != nil {
		return -1
	}
	if _, err := sk.Seek(cur, io.SeekStart); err != nil {
		return -1
	}
	return end - cur
}

// Put uploads the object. With Exclusive it checks the key first and asks
// the storage to refuse an existing object (If-None-Match); a storage
// without conditional writes gets a plain upload after the check.
func (s *S3) Put(ctx context.Context, key string, r io.Reader, opts PutOptions) (int64, error) {
	key, err := checkKey(key)
	if err != nil {
		return 0, err
	}
	if opts.Exclusive {
		if taken, err := Exists(ctx, s, key); err != nil {
			return 0, err
		} else if taken {
			return 0, fmt.Errorf("%s: %w", key, ErrExist)
		}
	}
	ct := opts.ContentType
	if ct == "" {
		ct = mime.TypeByExtension(path.Ext(key))
	}
	if ct == "" {
		ct = "application/octet-stream"
	}
	po := minio.PutObjectOptions{ContentType: ct, CacheControl: s.cacheControl}
	if !inline(ct) {
		po.ContentDisposition = "attachment"
	}
	size := remaining(r)
	start := int64(-1)
	if sk, ok := r.(io.Seeker); ok && size >= 0 {
		start, _ = sk.Seek(0, io.SeekCurrent)
	}
	put := func(po minio.PutObjectOptions) (int64, error) {
		cr := &countingReader{r: r}
		_, err := s.client.PutObject(ctx, s.bucket, s.object(key), cr, size, po)
		return cr.n, err
	}
	if opts.Exclusive {
		po.SetMatchETagExcept("*")
		n, err := put(po)
		resp := minio.ToErrorResponse(err)
		switch {
		case resp.StatusCode == http.StatusPreconditionFailed:
			return 0, fmt.Errorf("%s: %w", key, ErrExist)
		case resp.StatusCode == http.StatusNotImplemented && (n == 0 || start >= 0):
			// No conditional writes: upload again without the condition.
			if n > 0 {
				if _, err := r.(io.Seeker).Seek(start, io.SeekStart); err != nil {
					return 0, err
				}
			}
			po = minio.PutObjectOptions{ContentType: po.ContentType, ContentDisposition: po.ContentDisposition, CacheControl: po.CacheControl}
		default:
			if err != nil {
				return 0, err
			}
			return n, nil
		}
	}
	n, err := put(po)
	if err != nil {
		return 0, err
	}
	return n, nil
}

// Open opens the object; reading and seeking fetch its bytes as needed.
func (s *S3) Open(ctx context.Context, key string) (File, Info, error) {
	key, err := checkKey(key)
	if err != nil {
		return nil, Info{}, err
	}
	obj, err := s.client.GetObject(ctx, s.bucket, s.object(key), minio.GetObjectOptions{})
	if err != nil {
		return nil, Info{}, wrap(key, err)
	}
	oi, err := obj.Stat()
	if err != nil {
		_ = obj.Close()
		return nil, Info{}, wrap(key, err)
	}
	return obj, s.fileInfo(oi), nil
}

func (s *S3) fileInfo(oi minio.ObjectInfo) Info {
	key := s.key(oi.Key)
	return Info{Key: key, Name: path.Base(key), Size: oi.Size, ModTime: oi.LastModified}
}

func dirInfo(key string) Info {
	return Info{Key: key, Name: path.Base(key), IsDir: true}
}

// Stat describes the object key, else the folder key: one holding an
// object (or its own empty one).
func (s *S3) Stat(ctx context.Context, key string) (Info, error) {
	key, err := checkDir(key)
	if err != nil {
		return Info{}, err
	}
	if key == "" {
		return Info{IsDir: true}, nil
	}
	oi, err := s.client.StatObject(ctx, s.bucket, s.object(key), minio.StatObjectOptions{})
	if err == nil {
		return s.fileInfo(oi), nil
	}
	if !notFound(err) {
		return Info{}, err
	}
	ok, err := s.folderExists(ctx, key)
	if err != nil {
		return Info{}, err
	}
	if !ok {
		return Info{}, fmt.Errorf("%s: %w", key, ErrNotExist)
	}
	return dirInfo(key), nil
}

func (s *S3) folderExists(ctx context.Context, dir string) (bool, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	// Recursive: some storages keep the folders themselves and list an
	// emptied one as a common prefix.
	oi, ok := <-s.client.ListObjects(ctx, s.bucket, minio.ListObjectsOptions{Prefix: s.folder(dir), Recursive: true, MaxKeys: 1})
	if !ok {
		return false, nil
	}
	return oi.Err == nil, oi.Err
}

// List lists the folder dir a level deep: its objects and folders, the
// folders' own empty objects left out.
func (s *S3) List(ctx context.Context, dir string) ([]Info, error) {
	dir, err := checkDir(dir)
	if err != nil {
		return nil, err
	}
	folder := s.folder(dir)
	var out []Info
	found := dir == ""
	for oi := range s.client.ListObjects(ctx, s.bucket, minio.ListObjectsOptions{Prefix: folder}) {
		if oi.Err != nil {
			return nil, oi.Err
		}
		found = true
		switch {
		case oi.Key == folder:
			// The folder's own empty object (Mkdir).
		case strings.HasSuffix(oi.Key, "/"):
			// A storage keeping folders lists an emptied one too.
			key := strings.TrimSuffix(s.key(oi.Key), "/")
			ok, err := s.folderExists(ctx, key)
			if err != nil {
				return nil, err
			}
			if ok {
				out = append(out, dirInfo(key))
			}
		default:
			out = append(out, s.fileInfo(oi))
		}
	}
	if !found {
		return nil, fmt.Errorf("%s: %w", dir, ErrNotExist)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// Walk calls fn for every object under the folder dir, the folders'
// empty ones left out, in the bucket's order.
func (s *S3) Walk(ctx context.Context, dir string, fn func(Info) error) error {
	dir, err := checkDir(dir)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	for oi := range s.client.ListObjects(ctx, s.bucket, minio.ListObjectsOptions{Prefix: s.folder(dir), Recursive: true}) {
		if oi.Err != nil {
			return oi.Err
		}
		if strings.HasSuffix(oi.Key, "/") {
			continue
		}
		if err := fn(s.fileInfo(oi)); err != nil {
			return err
		}
	}
	return nil
}

// Delete removes the object key; a missing one is no error.
func (s *S3) Delete(ctx context.Context, key string) error {
	key, err := checkKey(key)
	if err != nil {
		return err
	}
	err = s.client.RemoveObject(ctx, s.bucket, s.object(key), minio.RemoveObjectOptions{})
	if err != nil && !notFound(err) {
		return err
	}
	return nil
}

// DeleteDir removes every object under the folder, its own empty one
// included.
func (s *S3) DeleteDir(ctx context.Context, dir string) error {
	dir, err := checkKey(dir)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	objects := make(chan minio.ObjectInfo)
	listErr := make(chan error, 1)
	go func() {
		defer close(objects)
		for oi := range s.client.ListObjects(ctx, s.bucket, minio.ListObjectsOptions{Prefix: s.folder(dir), Recursive: true}) {
			if oi.Err != nil {
				listErr <- oi.Err
				return
			}
			select {
			case objects <- oi:
			case <-ctx.Done():
				return
			}
		}
	}()
	var firstErr error
	for re := range s.client.RemoveObjects(ctx, s.bucket, objects, minio.RemoveObjectsOptions{}) {
		if re.Err != nil && !notFound(re.Err) && firstErr == nil {
			firstErr = fmt.Errorf("removing %s: %w", re.ObjectName, re.Err)
		}
	}
	select {
	case err := <-listErr:
		return err
	default:
	}
	return firstErr
}

// Mkdir stores the folder's empty object (dir/); its parents exist
// through it.
func (s *S3) Mkdir(ctx context.Context, dir string) error {
	dir, err := checkDir(dir)
	if err != nil || dir == "" {
		return err
	}
	_, err = s.client.PutObject(ctx, s.bucket, s.folder(dir), strings.NewReader(""), 0, minio.PutObjectOptions{})
	return err
}

// Rename copies the object (or every object of the folder) to the new
// name and removes the old one: not atomic, a failure midway leaves both.
func (s *S3) Rename(ctx context.Context, from, to string) error {
	from, err := checkKey(from)
	if err != nil {
		return err
	}
	if to, err = checkKey(to); err != nil {
		return err
	}
	src, err := s.Stat(ctx, from)
	if err != nil {
		return err
	}
	if taken, err := Exists(ctx, s, to); err != nil {
		return err
	} else if taken {
		return fmt.Errorf("%s: %w", to, ErrExist)
	}
	if !src.IsDir {
		if err := s.copy(ctx, s.object(from), s.object(to)); err != nil {
			return err
		}
		return s.Delete(ctx, from)
	}
	var objects []string
	for oi := range s.client.ListObjects(ctx, s.bucket, minio.ListObjectsOptions{Prefix: s.folder(from), Recursive: true}) {
		if oi.Err != nil {
			return oi.Err
		}
		objects = append(objects, oi.Key)
	}
	for _, o := range objects {
		dst := s.folder(to) + strings.TrimPrefix(o, s.folder(from))
		if strings.HasSuffix(o, "/") {
			// A folder's empty object: made anew (not every storage copies
			// one).
			if _, err := s.client.PutObject(ctx, s.bucket, dst, strings.NewReader(""), 0, minio.PutObjectOptions{}); err != nil {
				return err
			}
			continue
		}
		if err := s.copy(ctx, o, dst); err != nil {
			return err
		}
	}
	return s.DeleteDir(ctx, from)
}

func (s *S3) copy(ctx context.Context, from, to string) error {
	_, err := s.client.CopyObject(ctx,
		minio.CopyDestOptions{Bucket: s.bucket, Object: to},
		minio.CopySrcOptions{Bucket: s.bucket, Object: from})
	return wrap(s.key(from), err)
}
