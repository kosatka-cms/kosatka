package forms

import (
	"context"
	"log/slog"
	"math"
	"net/mail"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/kosatka-cms/kosatka/internal/modules/forms/manifest"
	"github.com/kosatka-cms/kosatka/modulekit"

	"github.com/kosatka-cms/kosatka/pkg/filestore"
	"github.com/kosatka-cms/kosatka/pkg/i18n"
	"github.com/kosatka-cms/kosatka/pkg/panicsafe"
	"github.com/kosatka-cms/kosatka/pkg/slug"
)

// Why a form takes no submission now.
var (
	ErrWebformClosed        = i18n.Errorf("The form is closed.")
	ErrWebformUnavailable   = i18n.Errorf("The form is not available.")
	ErrWebformLoginRequired = i18n.Errorf("Log in to fill in the form.")
	ErrWebformAlreadySent   = i18n.Errorf("You have already sent this form.")
)

// Availability - why the form takes no submission from the user now (nil -
// it does): unpublished or on moderation, closed (by hand, by its date
// or by its submission limit), or one submission per user and the user is
// anonymous or has sent it.
func (s *WebformService) Availability(ctx context.Context, w Webform, userID *int, now time.Time) error {
	if w.Node.Status != 1 || w.Node.OnModeration == 1 {
		return ErrWebformUnavailable
	}
	if w.Closed || (w.ClosesAt != nil && !now.Before(*w.ClosesAt)) {
		return ErrWebformClosed
	}
	if w.MaxSubmissions > 0 {
		n, err := s.repo.CountSubmissions(ctx, w.ID)
		if err != nil {
			return err
		}
		if n >= w.MaxSubmissions {
			return ErrWebformClosed
		}
	}
	if w.LimitPerUser {
		if userID == nil {
			return ErrWebformLoginRequired
		}
		sent, err := s.repo.HasUserSubmission(ctx, w.ID, *userID)
		if err != nil {
			return err
		}
		if sent {
			return ErrWebformAlreadySent
		}
	}
	return nil
}

// webformValueMaxLen - the longest value of an element, in characters.
const webformValueMaxLen = 10000

// WebformService - the forms module: forms with their elements and
// their submissions.
type WebformService struct {
	repo   WebformRepository
	nodes  modulekit.NodesService
	logger *slog.Logger
	// notify sends a plain e-mail (MailService.SendPlain); nil - no
	// notifications.
	notify func(to, subject, body string) error
	// wait - set by tests: notifications are sent before Submit returns.
	wait bool
	// files - where sent files are kept (SetFileStore); nil - file
	// elements take nothing.
	files *WebformFileStore
	// uploadLimit - the largest file, in bytes (SetUploadLimit; the upload
	// policy).
	uploadLimit func(ctx context.Context) int64
}

// webformDefaultUploadLimit - the largest file without an upload policy.
const webformDefaultUploadLimit = 10 << 20

// SetFileStore gives the forms where to keep the files sent with them.
func (s *WebformService) SetFileStore(st *WebformFileStore) { s.files = st }

// SetUploadLimit sets how the largest sent file is found (the site's upload
// policy).
func (s *WebformService) SetUploadLimit(limit func(ctx context.Context) int64) { s.uploadLimit = limit }

// UploadLimit - the largest file a form takes, in bytes.
func (s *WebformService) UploadLimit(ctx context.Context) int64 {
	if s.uploadLimit != nil {
		if n := s.uploadLimit(ctx); n > 0 {
			return n
		}
	}
	return webformDefaultUploadLimit
}

// Files - the store of sent files (nil - none).
func (s *WebformService) Files() *WebformFileStore { return s.files }

// SetNotifier makes a submission send an e-mail to the form's notify
// addresses (MailService.SendPlain).
func (s *WebformService) SetNotifier(send func(to, subject, body string) error) {
	s.notify = send
}

// WebformSubmissionsPerPage - submissions per page of the list.
const WebformSubmissionsPerPage = 50

// DeleteSubmissions deletes the form's submissions with these IDs (IDs of
// other forms are ignored); returns how many were deleted.
func (s *WebformService) DeleteSubmissions(ctx context.Context, w Webform, ids []int) (int, error) {
	// Their files go too (looked up by this form's file elements, so other
	// forms' submissions add nothing).
	var fileElements []int
	for _, e := range w.Elements {
		if e.Type == WebformFile {
			fileElements = append(fileElements, e.ID)
		}
	}
	files, err := s.repo.SubmissionValues(ctx, ids, fileElements)
	if err != nil {
		return 0, err
	}
	n, err := s.repo.DeleteSubmissions(ctx, w.ID, ids)
	if err != nil {
		return 0, err
	}
	if n > 0 {
		recordSubmissionsDeleted(ctx, w, n)
	}
	if n > 0 {
		s.logger.Info("form submissions deleted", "nodeID", w.Node.ID, "count", n)
		if s.files != nil {
			for _, v := range files {
				rel, _ := ParseWebformFileValue(v)
				if err := s.files.Remove(ctx, rel); err != nil {
					s.logger.Warn("sent file not removed", "path", rel, "error", err)
				}
			}
		}
	}
	return n, nil
}

// webformPurgeBatch - submissions deleted at a time by PurgeSubmissions.
const webformPurgeBatch = 500

// PurgeSubmissions deletes the form's submissions older than days (and
// their files); returns how many.
func (s *WebformService) PurgeSubmissions(ctx context.Context, w Webform, days int, now time.Time) (int, error) {
	if days < 1 {
		return 0, nil
	}
	ids, err := s.repo.SubmissionIDsBefore(ctx, w.ID, now.AddDate(0, 0, -days))
	if err != nil {
		return 0, err
	}
	total := 0
	for len(ids) > 0 {
		batch := ids[:min(webformPurgeBatch, len(ids))]
		ids = ids[len(batch):]
		n, err := s.DeleteSubmissions(ctx, w, batch)
		total += n
		if err != nil {
			return total, err
		}
	}
	return total, nil
}

// WebformAnalysis - a form's answers summed up (its Analysis page).
type WebformAnalysis struct {
	Total    int
	Elements []WebformElementAnalysis
}

// WebformElementAnalysis - one element: how many answered it (of Total)
// and, for a choice, each option's count (of those who answered).
type WebformElementAnalysis struct {
	Element  WebformElement
	Answered int
	Percent  int
	Options  []WebformOptionCount
}

// WebformOptionCount - an option's count; Other - a value no longer among
// the options (changed since it was sent).
type WebformOptionCount struct {
	Value   string
	Count   int
	Percent int
	Other   bool
}

// Analysis counts the form's answers: per element those who answered,
// per choice element each option (the current ones in order, then other
// stored values by count).
func (s *WebformService) Analysis(ctx context.Context, w Webform) (WebformAnalysis, error) {
	total, err := s.repo.CountSubmissions(ctx, w.ID)
	if err != nil {
		return WebformAnalysis{}, err
	}
	var ids, choices []int
	for _, e := range w.Elements {
		ids = append(ids, e.ID)
		if e.Type.HasOptions() {
			choices = append(choices, e.ID)
		}
	}
	counts, err := s.repo.AnswerCounts(ctx, ids, choices)
	if err != nil {
		return WebformAnalysis{}, err
	}
	percent := func(n, of int) int {
		if of == 0 {
			return 0
		}
		return (n*100 + of/2) / of
	}
	out := WebformAnalysis{Total: total}
	for _, e := range w.Elements {
		a := WebformElementAnalysis{Element: e, Answered: counts.Answered[e.ID]}
		a.Percent = percent(a.Answered, total)
		if e.Type.HasOptions() {
			values := counts.Values[e.ID]
			for _, o := range e.Options {
				a.Options = append(a.Options, WebformOptionCount{Value: o, Count: values[o], Percent: percent(values[o], a.Answered)})
			}
			var other []WebformOptionCount
			for v, n := range values {
				if !slices.Contains(e.Options, v) {
					other = append(other, WebformOptionCount{Value: v, Count: n, Percent: percent(n, a.Answered), Other: true})
				}
			}
			slices.SortFunc(other, func(x, y WebformOptionCount) int {
				if x.Count != y.Count {
					return y.Count - x.Count
				}
				return strings.Compare(x.Value, y.Value)
			})
			a.Options = append(a.Options, other...)
		}
		out.Elements = append(out.Elements, a)
	}
	return out, nil
}

// WebformListItem - a form in the admin list.
type WebformListItem struct {
	Webform Webform
	WebformStats
}

// AdminList - every form with its submissions count and last time, by
// title.
func (s *WebformService) AdminList(ctx context.Context) ([]WebformListItem, error) {
	forms, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	stats, err := s.repo.SubmissionStats(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]WebformListItem, 0, len(forms))
	for _, w := range forms {
		out = append(out, WebformListItem{Webform: w, WebformStats: stats[w.ID]})
	}
	return out, nil
}

// Stats - the submissions count and last time of w.
func (s *WebformService) Stats(ctx context.Context, w Webform) (WebformStats, error) {
	stats, err := s.repo.SubmissionStats(ctx)
	if err != nil {
		return WebformStats{}, err
	}
	return stats[w.ID], nil
}

// BlockTargets - the forms the "Form" block can show (its placement page):
// node ID → title.
func (s *WebformService) BlockTargets(ctx context.Context) ([]modulekit.PluginBlockTarget, error) {
	forms, err := s.repo.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]modulekit.PluginBlockTarget, 0, len(forms))
	for _, w := range forms {
		out = append(out, modulekit.PluginBlockTarget{Value: strconv.Itoa(w.Node.ID), Label: w.Node.Title})
	}
	return out, nil
}

// CountSubmissions - the form's submissions.
func (s *WebformService) CountSubmissions(ctx context.Context, w Webform) (int, error) {
	return s.repo.CountSubmissions(ctx, w.ID)
}

// Submissions - a page (from 1) of the form's submissions, newest first.
func (s *WebformService) Submissions(ctx context.Context, w Webform, page int) ([]WebformSubmission, error) {
	if page < 1 {
		page = 1
	}
	return s.repo.Submissions(ctx, w.ID, WebformSubmissionsPerPage, (page-1)*WebformSubmissionsPerPage)
}

// Submission - one submission of the form with the newer and older ones'
// IDs (0 - none); nil when the form has no such submission.
func (s *WebformService) Submission(ctx context.Context, w Webform, id int) (sub *WebformSubmission, newer, older int, err error) {
	sub, err = s.repo.Submission(ctx, w.ID, id)
	if err != nil || sub == nil {
		return nil, 0, 0, err
	}
	newer, older, err = s.repo.SubmissionNeighbors(ctx, *sub)
	return sub, newer, older, err
}

// EachSubmission calls fn for every submission of the form, newest first,
// reading them in batches (the CSV export).
func (s *WebformService) EachSubmission(ctx context.Context, w Webform, fn func(WebformSubmission) error) error {
	const batch = 500
	for offset := 0; ; offset += batch {
		subs, err := s.repo.Submissions(ctx, w.ID, batch, offset)
		if err != nil {
			return err
		}
		for _, sub := range subs {
			if err := fn(sub); err != nil {
				return err
			}
		}
		if len(subs) < batch {
			return nil
		}
	}
}

// NewWebformService creates a WebformService.
func NewWebformService(repo WebformRepository, nodes modulekit.NodesService, baseLogger *slog.Logger) *WebformService {
	return &WebformService{repo: repo, nodes: nodes, logger: baseLogger.With("component", "webform.service")}
}

// ByNodeID - the form of node nodeID, with its elements and its node.
func (s *WebformService) ByNodeID(ctx context.Context, nodeID int) (*Webform, error) {
	w, err := s.repo.ByNodeID(ctx, nodeID)
	return w, err
}

// BySlug - the form whose node has slug (ErrEntityNotFound - none).
func (s *WebformService) BySlug(ctx context.Context, slug string) (*Webform, error) {
	w, err := s.repo.BySlug(ctx, slug)
	return w, err
}

// Create stores a new form (a slug from its title when it has none);
// returns its node ID. The action log gets "Content created".
func (s *WebformService) Create(ctx context.Context, w Webform) (int, error) {
	if w.Node.Slug == "" {
		generated, err := s.nodes.GenerateUniqueSlug(ctx, w.Node.Title, manifest.ContentType)
		if err != nil {
			return 0, err
		}
		w.Node.Slug = generated
	}
	prepareWebformElements(w.Elements, nil)
	id, err := s.repo.Create(ctx, w)
	if err != nil {
		return 0, err
	}
	if created, err := s.repo.ByNodeID(ctx, id); err == nil {
		modulekit.RecordNodeCreated(ctx, created.Node)
	} else {
		modulekit.LogFallback(s.logger, err, "the created form for the action log", "nodeID", id)
	}
	return id, nil
}

// Update stores the form; existing elements keep their machine names, new
// ones get names unique among them. What it changed of the content (status,
// author, address) goes to the action log.
func (s *WebformService) Update(ctx context.Context, w Webform) error {
	existing, err := s.repo.ByNodeID(ctx, w.Node.ID)
	if err != nil {
		return err
	}
	prepareWebformElements(w.Elements, existing.Elements)
	if err := s.repo.Update(ctx, w); err != nil {
		return err
	}
	if saved, err := s.repo.ByNodeID(ctx, w.Node.ID); err == nil {
		modulekit.RecordNodeSaved(ctx, existing.Node, saved.Node)
	} else {
		modulekit.LogFallback(s.logger, err, "the saved form for the action log", "nodeID", w.Node.ID)
	}
	// A removed file element's files go with it.
	kept := map[int]bool{}
	for _, e := range w.Elements {
		kept[e.ID] = true
	}
	for _, e := range existing.Elements {
		if e.Type == WebformFile && !kept[e.ID] && s.files != nil {
			if err := s.files.RemoveAll(ctx, w.Node.ID, e.ID); err != nil {
				s.logger.Warn("files of a removed element not removed", "nodeID", w.Node.ID, "elementID", e.ID, "error", err)
			}
		}
	}
	return nil
}

// prepareWebformElements readies the editor's elements for storing: the
// saved ones (by ID, among saved) keep their machine names, the new ones
// get theirs (assignWebformMachineNames), the conditions' references
// become names (resolveWebformConditions).
func prepareWebformElements(elements, saved WebformElements) {
	names := map[int]string{}
	for _, e := range saved {
		names[e.ID] = e.MachineName
	}
	for i := range elements {
		if elements[i].ID > 0 {
			elements[i].MachineName = names[elements[i].ID]
		}
	}
	assignWebformMachineNames(elements)
	resolveWebformConditions(elements)
}

// resolveWebformConditions turns the editor's references (VisibleIfRef:
// an earlier element's position plus one) into machine names; a reference
// to the element itself or a later one is dropped.
func resolveWebformConditions(elements WebformElements) {
	for i := range elements {
		ref := elements[i].VisibleIfRef
		if ref > 0 && ref-1 < i {
			elements[i].VisibleIfElement = elements[ref-1].MachineName
		} else if ref != 0 || elements[i].VisibleIfElement == "" {
			elements[i].VisibleIfElement, elements[i].VisibleIfValue = "", ""
		}
		elements[i].VisibleIfRef = 0
	}
}

// assignWebformMachineNames gives the elements without a machine name one
// from the label (transliterated, "_" separated), unique among them.
func assignWebformMachineNames(elements WebformElements) {
	used := map[string]bool{}
	for _, e := range elements {
		if e.MachineName != "" {
			used[e.MachineName] = true
		}
	}
	for i := range elements {
		if elements[i].MachineName != "" {
			continue
		}
		base := strings.ReplaceAll(slug.Generate(elements[i].Label), "-", "_")
		if len(base) > 48 {
			base = strings.TrimRight(base[:48], "_")
		}
		if base == "" || base[0] < 'a' || base[0] > 'z' {
			base = "element_" + base
		}
		name := base
		for n := 2; used[name]; n++ {
			name = base + "_" + strconv.Itoa(n)
		}
		used[name] = true
		elements[i].MachineName = name
	}
}

// Submit validates the values (by element ID; one per checked choice of a
// checkboxes element) and stores the submission. Field errors (by element
// ID, untranslated sources) come back with a nil error; a form that takes
// no submission from the user gives Availability's error. The sender's
// page (from) is kept with it and named in the notification.
func (s *WebformService) Submit(ctx context.Context, w Webform, input map[int][]string, uploads map[int]WebformUpload, from WebformSender) (map[int]string, error) {
	userID := from.UserID
	if err := s.Availability(ctx, w, userID, time.Now()); err != nil {
		return nil, err
	}
	values, errs := ValidateWebformInput(w.Elements, input)
	for id, msg := range s.checkUploads(ctx, w.Elements, input, uploads) {
		errs[id] = msg
	}
	if len(errs) > 0 {
		return errs, nil
	}
	saved, err := s.saveUploads(ctx, w, input, uploads, values)
	if err != nil {
		return nil, err
	}
	if _, err := s.repo.AddSubmission(ctx, WebformSubmission{
		WebformID: w.ID, UserID: userID, RemoteIP: from.RemoteIP, SourceURL: from.Source, CreatedAt: time.Now(), Values: values,
	}); err != nil {
		for _, rel := range saved {
			_ = s.files.Remove(ctx, rel)
		}
		return nil, err
	}
	s.logger.Info("form submitted", "nodeID", w.Node.ID, "userID", userID)
	if to := w.NotifyAddresses(); s.notify != nil && len(to) > 0 {
		subject, body := WebformNotification(w, values, from.Source)
		send := func() {
			defer panicsafe.Recover(s.logger, "webform.notify")
			for _, addr := range to {
				if err := s.notify(addr, subject, body); err != nil {
					s.logger.Warn("form notification not sent", "nodeID", w.Node.ID, "to", addr, "error", err)
				}
			}
		}
		if s.wait {
			send()
		} else {
			// The visitor doesn't wait for the mail server.
			go send()
		}
	}
	return nil, nil
}

// OpenSubmissionFile opens a file sent with the form (submission, element,
// delta); returns it with its original name.
func (s *WebformService) OpenSubmissionFile(ctx context.Context, w Webform, submissionID, elementID, delta int) (filestore.File, filestore.Info, string, error) {
	isFile := false
	for _, e := range w.Elements {
		isFile = isFile || (e.ID == elementID && e.Type == WebformFile)
	}
	if !isFile || s.files == nil {
		return nil, filestore.Info{}, "", os.ErrNotExist
	}
	v, err := s.repo.SubmissionValue(ctx, w.ID, submissionID, elementID, delta)
	if err != nil {
		return nil, filestore.Info{}, "", err
	}
	if v == "" {
		return nil, filestore.Info{}, "", os.ErrNotExist
	}
	rel, name := ParseWebformFileValue(v)
	f, info, err := s.files.Open(ctx, rel)
	return f, info, name, err
}

// checkUploads - the errors of the shown file elements' uploads: required,
// an allowed extension, the size limit, and the core's check of
// documents (modulekit.DocumentAllowed: no page, script or program under
// any name - it could run in the browser of whoever opens it).
func (s *WebformService) checkUploads(ctx context.Context, elements WebformElements, input map[int][]string, uploads map[int]WebformUpload) map[int]string {
	errs := map[int]string{}
	visible := elements.Visible(input)
	limit := s.UploadLimit(ctx)
	for _, e := range elements {
		if e.Type != WebformFile || !visible[e.ID] {
			continue
		}
		up, ok := uploads[e.ID]
		if !ok || len(up.Data) == 0 {
			if e.Required {
				errs[e.ID] = i18n.N("This field is required.")
			}
			continue
		}
		ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(up.Name), "."))
		switch {
		case s.files == nil:
			errs[e.ID] = i18n.N("Files can't be sent with this form now.")
		case !slices.Contains(e.FileExtensions(), ext) || !modulekit.DocumentAllowed(up.Name, up.Data):
			errs[e.ID] = i18n.N("This file type is not allowed.")
		case int64(len(up.Data)) > limit:
			errs[e.ID] = i18n.N("The file is too large.")
		}
	}
	return errs
}

// saveUploads keeps the shown file elements' uploads and records them in
// values; returns the saved paths (to remove if the submission fails).
func (s *WebformService) saveUploads(ctx context.Context, w Webform, input map[int][]string, uploads map[int]WebformUpload, values map[int][]string) ([]string, error) {
	var saved []string
	visible := w.Elements.Visible(input)
	for _, e := range w.Elements {
		up, ok := uploads[e.ID]
		if e.Type != WebformFile || !visible[e.ID] || !ok || len(up.Data) == 0 || s.files == nil {
			continue
		}
		ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(up.Name), "."))
		rel, err := s.files.Save(ctx, w.Node.ID, e.ID, ext, up.Data)
		if err != nil {
			for _, r := range saved {
				_ = s.files.Remove(ctx, r)
			}
			return nil, err
		}
		saved = append(saved, rel)
		values[e.ID] = []string{WebformFileValue(rel, webformFileName(up.Name))}
	}
	return saved, nil
}

// webformFileName - a sent file's name as shown: no path, no line breaks,
// at most 200 characters.
func webformFileName(name string) string {
	name = filepath.Base(strings.ReplaceAll(name, "\\", "/"))
	name = strings.Map(func(r rune) rune {
		if r < 32 {
			return -1
		}
		return r
	}, name)
	if r := []rune(name); len(r) > 200 {
		name = string(r[:200])
	}
	if name == "" || name == "." || name == "/" {
		name = "file"
	}
	return name
}

// WebformNotification - the e-mail about a submission, in the site's
// language: the form's title, every answered element as "Label: value"
// (several values joined by ", "), the page it was sent from (source; "" -
// unknown) and the link to its submissions.
func WebformNotification(w Webform, values map[int][]string, source string) (subject, body string) {
	lang := i18n.Default()
	tr := func(s string, args ...any) string { return i18n.Current().Text(lang, "", s, args...) }
	subject = tr(i18n.N("New submission: @form"), "@form", w.Node.Title)
	var b strings.Builder
	b.WriteString(tr(i18n.N("The form \"@form\" has been sent."), "@form", w.Node.Title))
	b.WriteString("\n\n")
	for _, e := range w.Elements {
		v := values[e.ID]
		switch {
		case len(v) == 0:
		case e.Type == WebformConsent:
			b.WriteString(e.Label + ": " + tr("Yes") + "\n")
		case e.Type == WebformFile:
			_, name := ParseWebformFileValue(v[0])
			b.WriteString(e.Label + ": " + name + "\n")
		default:
			b.WriteString(e.Label + ": " + strings.Join(v, ", ") + "\n")
		}
	}
	if source != "" {
		b.WriteString("\n" + tr(i18n.N("Sent from the page:")) + " " + modulekit.SiteDomain() + source + "\n")
	}
	b.WriteString("\n" + tr(i18n.N("All submissions:")) + " " + modulekit.SiteDomain() + "/node/" + strconv.Itoa(w.Node.ID) + "/form/submissions\n")
	return subject, b.String()
}

// ValidateWebformInput - the cleaned values of the elements (trimmed,
// empty dropped) and the errors by element ID (untranslated sources):
// required, e-mail format, a choice among the options, one value for a
// single-value element, a length limit.
func ValidateWebformInput(elements WebformElements, input map[int][]string) (map[int][]string, map[int]string) {
	values := map[int][]string{}
	errs := map[int]string{}
	// An element hidden by its condition is neither checked nor stored.
	visible := elements.Visible(input)
	for _, e := range elements {
		// A file comes as an upload (checkUploads), never as a value.
		if !visible[e.ID] || e.Type == WebformFile {
			continue
		}
		var got []string
		for _, v := range input[e.ID] {
			if v = strings.TrimSpace(v); v != "" {
				got = append(got, v)
			}
		}
		switch {
		case e.Type == WebformConsent:
			// Always required; ticked is the one value.
			if len(got) != 1 || got[0] != ConsentValue {
				errs[e.ID] = i18n.N("Tick the checkbox to send the form.")
				continue
			}
		case len(got) == 0:
			if e.Required {
				errs[e.ID] = i18n.N("This field is required.")
			}
			continue
		case len(got) > 1 && !e.Type.Multiple():
			errs[e.ID] = i18n.N("Choose one value.")
			continue
		}
		for _, v := range got {
			switch {
			case utf8.RuneCountInString(v) > webformValueMaxLen:
				errs[e.ID] = i18n.N("The value is too long.")
			case e.Type == WebformEmail && !validWebformEmail(v):
				errs[e.ID] = i18n.N("Enter a valid e-mail address.")
			case e.Type == WebformNumber && !validWebformNumber(e, v):
				errs[e.ID] = i18n.N("Enter a number within the allowed range.")
			case e.Type == WebformDate && !validWebformDate(v):
				errs[e.ID] = i18n.N("Enter a valid date.")
			case e.Type == WebformPhone && !validWebformPhone(v):
				errs[e.ID] = i18n.N("Enter a valid phone number.")
			case e.Type.HasOptions() && !slices.Contains(e.Options, v):
				errs[e.ID] = i18n.N("Choose one of the options.")
			}
		}
		if _, bad := errs[e.ID]; !bad {
			values[e.ID] = got
		}
	}
	return values, errs
}

// ParseWebformNumber - a number as typed ("1,5" too); ok false for
// anything else (or not finite).
func ParseWebformNumber(v string) (float64, bool) {
	f, err := strconv.ParseFloat(strings.Replace(strings.TrimSpace(v), ",", ".", 1), 64)
	return f, err == nil && !math.IsInf(f, 0) && !math.IsNaN(f)
}

func validWebformNumber(e WebformElement, v string) bool {
	f, ok := ParseWebformNumber(v)
	if !ok {
		return false
	}
	lowest, highest := e.NumberRange()
	if m, ok := ParseWebformNumber(lowest); ok && f < m {
		return false
	}
	if m, ok := ParseWebformNumber(highest); ok && f > m {
		return false
	}
	return true
}

func validWebformDate(v string) bool {
	_, err := time.Parse(time.DateOnly, v)
	return err == nil
}

// validWebformPhone - digits with the usual separators (+ ( ) - . and
// spaces), 5 to 15 digits (E.164's longest).
func validWebformPhone(v string) bool {
	digits := 0
	for _, r := range v {
		switch {
		case r >= '0' && r <= '9':
			digits++
		case strings.ContainsRune("+()-. ", r):
		default:
			return false
		}
	}
	return digits >= 5 && digits <= 15
}

func validWebformEmail(v string) bool {
	a, err := mail.ParseAddress(v)
	return err == nil && a.Address == v
}
