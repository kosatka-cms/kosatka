package forms

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/kosatka-cms/kosatka/modulekit"
	"github.com/kosatka-cms/kosatka/modulekit/testkit"
	"github.com/kosatka-cms/kosatka/pkg/filestore"
)

// fakeWebformFilesRepo keeps one form's submission values for the file
// tests.
type fakeWebformFilesRepo struct {
	fakeWebformRepo
	values   map[[3]int]string // submission, element, delta → value
	deleted  []int
	before   []int
	onDelete func(ids []int)
	choices  []int
}

func (f *fakeWebformFilesRepo) AddSubmission(_ context.Context, s WebformSubmission) (int, error) {
	f.sub = &s
	for eid, vs := range s.Values {
		for d, v := range vs {
			f.values[[3]int{1, eid, d}] = v
		}
	}
	return 1, nil
}

func (f *fakeWebformFilesRepo) SubmissionValues(_ context.Context, sids, eids []int) ([]string, error) {
	var out []string
	for k, v := range f.values {
		for _, sid := range sids {
			for _, eid := range eids {
				if k[0] == sid && k[1] == eid {
					out = append(out, v)
				}
			}
		}
	}
	return out, nil
}

func (f *fakeWebformFilesRepo) SubmissionValue(_ context.Context, _, sid, eid, d int) (string, error) {
	return f.values[[3]int{sid, eid, d}], nil
}

func (f *fakeWebformFilesRepo) DeleteSubmissions(_ context.Context, _ int, ids []int) (int, error) {
	f.deleted = ids
	if f.onDelete != nil {
		f.onDelete(ids)
	}
	return len(ids), nil
}

func newWebformFilesService(t *testing.T) (*WebformService, *fakeWebformFilesRepo, string) {
	t.Helper()
	dir := t.TempDir()
	repo := &fakeWebformFilesRepo{values: map[[3]int]string{}}
	svc := NewWebformService(repo, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	svc.SetFileStore(&WebformFileStore{Files: func() filestore.Store { return filestore.NewLocal(dir) }})
	svc.SetUploadLimit(func(context.Context) int64 { return 100 })
	return svc, repo, dir
}

func testFileWebform() Webform {
	return Webform{ID: 3, Node: modulekit.Node{ID: 9, Status: 1}, Elements: WebformElements{
		{ID: 1, MachineName: "kind", Type: WebformRadios, Label: "Kind", Options: []string{"CV", "None"}},
		{ID: 2, Type: WebformFile, Label: "CV", Required: true, Options: []string{"pdf", "txt", "svg"}, VisibleIfElement: "kind", VisibleIfValue: "CV"},
	}}
}

// TestWebformFileStorePaths - saved files land under webform/<node>/<element>;
// paths that leave the store are refused.
func TestWebformFileStorePaths(t *testing.T) {
	dir := t.TempDir()
	st := WebformFileStore{Files: func() filestore.Store { return filestore.NewLocal(dir) }}
	rel, err := st.Save(context.Background(), 9, 2, "pdf", []byte("x"))
	if err != nil || !strings.HasPrefix(rel, "webform/9/2/") || !strings.HasSuffix(rel, ".pdf") {
		t.Fatalf("save: %q %v", rel, err)
	}
	f, _, err := st.Open(context.Background(), rel)
	if err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
	for _, bad := range []string{"../secret", "webform/../../etc/passwd", "/webform/9/2/a.pdf", "other/a.pdf", "webform/9/./2/a.pdf"} {
		if _, _, err := st.Open(context.Background(), bad); !errors.Is(err, errWebformFilePath) {
			t.Errorf("%q: %v", bad, err)
		}
	}
	if err := st.RemoveAll(context.Background(), 9, 2); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "webform", "9", "2")); !os.IsNotExist(err) {
		t.Errorf("element folder left: %v", err)
	}
	if err := st.RemoveAll(context.Background(), 0, 0); err == nil {
		t.Error("RemoveAll without a node must fail")
	}
}

// TestWebformSubmitFiles - a shown file element checks required, the
// extension, web pages under any name and the size; a hidden one is
// skipped; a valid file is stored privately and opened by its submission.
func TestWebformSubmitFiles(t *testing.T) {
	svc, repo, dir := newWebformFilesService(t)
	ctx := context.Background()
	w := testFileWebform()
	shown := map[int][]string{1: {"CV"}}
	cases := map[string]struct {
		up   WebformUpload
		want string
	}{
		"missing":   {WebformUpload{}, "This field is required."},
		"extension": {WebformUpload{Name: "cv.exe", Data: []byte("MZ")}, "This file type is not allowed."},
		"svg":       {WebformUpload{Name: "cv.svg", Data: []byte("<svg/>")}, "This file type is not allowed."},
		"html":      {WebformUpload{Name: "cv.txt", Data: []byte("<html><script>x</script>")}, "This file type is not allowed."},
		"too large": {WebformUpload{Name: "cv.txt", Data: []byte(strings.Repeat("a", 101))}, "The file is too large."},
	}
	for name, c := range cases {
		errs, err := svc.Submit(ctx, w, shown, map[int]WebformUpload{2: c.up}, WebformSender{})
		if err != nil || errs[2] != c.want {
			t.Errorf("%s: %v %v", name, errs, err)
		}
	}
	if repo.sub != nil {
		t.Fatal("nothing must be stored after errors")
	}
	if errs, err := svc.Submit(ctx, w, map[int][]string{1: {"None"}}, nil, WebformSender{}); err != nil || len(errs) != 0 {
		t.Errorf("hidden element: %v %v", errs, err)
	}

	repo.values = map[[3]int]string{}
	errs, err := svc.Submit(ctx, w, shown, map[int]WebformUpload{2: {Name: `C:\docs\my cv.pdf`, Data: []byte("%PDF-1.4")}}, WebformSender{})
	if err != nil || len(errs) != 0 {
		t.Fatalf("valid: %v %v", errs, err)
	}
	rel, name := ParseWebformFileValue(repo.sub.Values[2][0])
	if name != "my cv.pdf" || !strings.HasPrefix(rel, "webform/9/2/") {
		t.Fatalf("stored value: %q %q", rel, name)
	}
	if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(rel))); err != nil {
		t.Fatal(err)
	}
	f, info, got, err := svc.OpenSubmissionFile(ctx, w, 1, 2, 0)
	if err != nil || got != "my cv.pdf" || info.Size == 0 {
		t.Fatalf("open: %q %v", got, err)
	}
	_ = f.Close()
	if _, _, _, err := svc.OpenSubmissionFile(ctx, w, 1, 1, 0); !os.IsNotExist(err) {
		t.Errorf("a non-file element: %v", err)
	}

	actions := testkit.CaptureActions(t)
	if n, err := svc.DeleteSubmissions(ctx, w, []int{1}); err != nil || n != 1 {
		t.Fatalf("delete: %d %v", n, err)
	}
	if len(*actions) != 1 || (*actions)[0].Type != ActionSubmissionsDelete || (*actions)[0].Changes[0].To != "1" {
		t.Errorf("the action log: %+v", *actions)
	}
	if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(rel))); !os.IsNotExist(err) {
		t.Errorf("file left after the submission was deleted: %v", err)
	}
}

// TestWebformSubmitFilesWithoutStore - with no file store a file is refused
// rather than lost.
func TestWebformSubmitFilesWithoutStore(t *testing.T) {
	repo := &fakeWebformFilesRepo{values: map[[3]int]string{}}
	svc := NewWebformService(repo, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	errs, err := svc.Submit(context.Background(), testFileWebform(), map[int][]string{1: {"CV"}},
		map[int]WebformUpload{2: {Name: "cv.pdf", Data: []byte("%PDF")}}, WebformSender{})
	if err != nil || errs[2] != "Files can't be sent with this form now." {
		t.Errorf("%v %v", errs, err)
	}
}

// TestWebformFileName - a sent name loses its path and control characters.
func TestWebformFileName(t *testing.T) {
	for in, want := range map[string]string{
		`C:\a\b.pdf`: "b.pdf", "../../x.txt": "x.txt", "a\nb.pdf": "ab.pdf", "": "file", "/": "file",
	} {
		if got := webformFileName(in); got != want {
			t.Errorf("%q: %q, want %q", in, got, want)
		}
	}
}

// TestWebformUpdateRemovesFiles - removing a file element removes its
// files; the kept elements' files stay.
func TestWebformUpdateRemovesFiles(t *testing.T) {
	svc, repo, dir := newWebformFilesService(t)
	w := testFileWebform()
	repo.existing = &w
	st := WebformFileStore{Files: func() filestore.Store { return filestore.NewLocal(dir) }}
	gone, _ := st.Save(context.Background(), 9, 2, "pdf", []byte("x"))
	kept, _ := st.Save(context.Background(), 9, 1, "pdf", []byte("x"))
	upd := Webform{Node: modulekit.Node{ID: 9}, Elements: WebformElements{w.Elements[0]}}
	if err := svc.Update(context.Background(), upd); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, gone)); !os.IsNotExist(err) {
		t.Errorf("removed element's file left: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, kept)); err != nil {
		t.Errorf("kept element's file: %v", err)
	}
}

func (f *fakeWebformFilesRepo) SubmissionIDsBefore(context.Context, int, time.Time) ([]int, error) {
	return f.before, nil
}

func (f *fakeWebformFilesRepo) CountSubmissions(context.Context, int) (int, error) { return 4, nil }

func (f *fakeWebformFilesRepo) AnswerCounts(_ context.Context, ids, choices []int) (WebformAnswerCounts, error) {
	f.choices = choices
	return WebformAnswerCounts{
		Answered: map[int]int{1: 3, 2: 1},
		Values:   map[int]map[string]int{1: {"CV": 2, "Old": 1}},
	}, nil
}

// TestWebformPurgeSubmissions - submissions older than the days go in
// batches, with their files; zero days delete nothing.
func TestWebformPurgeSubmissions(t *testing.T) {
	svc, repo, dir := newWebformFilesService(t)
	w := testFileWebform()
	st := WebformFileStore{Files: func() filestore.Store { return filestore.NewLocal(dir) }}
	rel, _ := st.Save(context.Background(), 9, 2, "pdf", []byte("x"))
	repo.values[[3]int{1, 2, 0}] = WebformFileValue(rel, "cv.pdf")
	for i := 1; i <= webformPurgeBatch+20; i++ {
		repo.before = append(repo.before, i)
	}
	var batches []int
	repo.onDelete = func(ids []int) { batches = append(batches, len(ids)) }
	n, err := svc.PurgeSubmissions(context.Background(), w, 30, time.Now())
	if err != nil || n != webformPurgeBatch+20 || len(batches) != 2 || batches[1] != 20 {
		t.Fatalf("purge: %d %v %v", n, err, batches)
	}
	if _, err := os.Stat(filepath.Join(dir, rel)); !os.IsNotExist(err) {
		t.Errorf("file left: %v", err)
	}
	if n, _ := svc.PurgeSubmissions(context.Background(), w, 0, time.Now()); n != 0 {
		t.Errorf("zero days: %d", n)
	}
}

// TestWebformAnalysis - answered share of all submissions; a choice's
// options in order with zero counts kept, other stored values after them;
// shares of those who answered.
func TestWebformAnalysis(t *testing.T) {
	svc, repo, _ := newWebformFilesService(t)
	w := testFileWebform()
	a, err := svc.Analysis(context.Background(), w)
	if err != nil {
		t.Fatal(err)
	}
	if len(repo.choices) != 1 || repo.choices[0] != 1 {
		t.Errorf("choices asked: %v", repo.choices)
	}
	kind, file := a.Elements[0], a.Elements[1]
	if a.Total != 4 || kind.Answered != 3 || kind.Percent != 75 || file.Answered != 1 || file.Percent != 25 || file.Options != nil {
		t.Fatalf("analysis: %+v", a)
	}
	want := []WebformOptionCount{{Value: "CV", Count: 2, Percent: 67}, {Value: "None", Count: 0, Percent: 0}, {Value: "Old", Count: 1, Percent: 33, Other: true}}
	if !reflect.DeepEqual(kind.Options, want) {
		t.Errorf("options: %+v", kind.Options)
	}
}
