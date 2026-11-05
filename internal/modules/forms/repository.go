package forms

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/kosatka-cms/kosatka/internal/modules/forms/ent/generated"
	"github.com/kosatka-cms/kosatka/internal/modules/forms/ent/generated/predicate"
	"github.com/kosatka-cms/kosatka/internal/modules/forms/ent/generated/webform"
	"github.com/kosatka-cms/kosatka/internal/modules/forms/ent/generated/webformelement"
	"github.com/kosatka-cms/kosatka/internal/modules/forms/ent/generated/webformsubmission"
	"github.com/kosatka-cms/kosatka/internal/modules/forms/ent/generated/webformsubmissionvalue"
	"github.com/kosatka-cms/kosatka/internal/modules/forms/manifest"
	"github.com/kosatka-cms/kosatka/modulekit"
)

var _ WebformRepository = (*EntWebformRepository)(nil)

// EntWebformRepository implements WebformRepository on the module's own
// tables; a form's node (the core's NodesRepository), row and elements are
// written in one transaction (modulekit.InTx). A form's node and a
// submission's author are the core's rows, read through its repositories.
type EntWebformRepository struct {
	db    *sql.DB
	nodes modulekit.NodesRepository
	users modulekit.UserRepository
}

// NewWebformRepository creates a EntWebformRepository.
func NewWebformRepository(db *sql.DB, nodes modulekit.NodesRepository, users modulekit.UserRepository) *EntWebformRepository {
	return &EntWebformRepository{db: db, nodes: nodes, users: users}
}

// client - the module's ent client: the context's transaction (InTx),
// else the database.
func (r *EntWebformRepository) client(ctx context.Context) *generated.Client {
	return generated.NewClient(generated.Driver(modulekit.Driver(ctx, r.db)))
}

// one - the form matching where, with its elements, its node and the
// node's SEO.
func (r *EntWebformRepository) one(ctx context.Context, where predicate.Webform, what string) (*Webform, error) {
	res, err := r.client(ctx).Webform.Query().Where(where).WithElements().Only(ctx)
	if err != nil {
		if generated.IsNotFound(err) {
			return nil, fmt.Errorf("%w: form %s", modulekit.ErrEntityNotFound, what)
		}
		return nil, err
	}
	out := ToWebformEntity(res)
	n, err := r.nodes.ByID(ctx, res.Nid)
	if err != nil {
		return nil, fmt.Errorf("the node of form %s: %w", what, err)
	}
	if n.PageSEO, err = r.nodes.PageSEO(ctx, n.ID); err != nil {
		return nil, fmt.Errorf("the SEO of form %s: %w", what, err)
	}
	out.Node = *n
	return out, nil
}

// ByNodeID - the form of node nodeID, with its elements, its node and
// the node's SEO.
func (r *EntWebformRepository) ByNodeID(ctx context.Context, nodeID int) (*Webform, error) {
	return r.one(ctx, webform.Nid(nodeID), fmt.Sprintf("with node ID %d", nodeID))
}

// BySlug - the form whose node has slug (ErrEntityNotFound - none, or a
// node of another type).
func (r *EntWebformRepository) BySlug(ctx context.Context, slug string) (*Webform, error) {
	n, err := r.nodes.BySlug(ctx, slug)
	if err != nil {
		return nil, fmt.Errorf("form with slug %s: %w", slug, err)
	}
	if n.Type != manifest.ContentType {
		return nil, fmt.Errorf("%w: form with slug %s", modulekit.ErrEntityNotFound, slug)
	}
	return r.ByNodeID(ctx, n.ID)
}

// Create - see WebformRepository.Create.
func (r *EntWebformRepository) Create(ctx context.Context, w Webform) (int, error) {
	var nodeID int
	err := modulekit.InTx(ctx, r.db, func(ctx context.Context) error {
		n := w.Node
		n.Type = manifest.ContentType
		id, err := r.nodes.CreateRow(ctx, n)
		if err != nil {
			return err
		}
		nodeID = id
		c := r.client(ctx)
		form, err := c.Webform.Create().
			SetNid(id).
			SetDescription(w.Description).
			SetConfirmation(w.Confirmation).SetConfirmationURL(w.ConfirmationURL).
			SetSubmitLabel(w.SubmitLabel).
			SetClosed(w.Closed).
			SetNotifyEmail(w.NotifyEmail).
			SetLimitPerUser(w.LimitPerUser).
			SetMaxSubmissions(w.MaxSubmissions).
			SetNillableClosesAt(w.ClosesAt).
			Save(ctx)
		if err != nil {
			return fmt.Errorf("create form: %w", err)
		}
		return saveWebformElements(ctx, c, form.ID, w.Elements)
	})
	return nodeID, err
}

// Update - see WebformRepository.Update.
func (r *EntWebformRepository) Update(ctx context.Context, w Webform) error {
	return modulekit.InTx(ctx, r.db, func(ctx context.Context) error {
		c := r.client(ctx)
		form, err := c.Webform.Query().Where(webform.Nid(w.Node.ID)).Only(ctx)
		if err != nil {
			return err
		}
		upd := c.Webform.UpdateOneID(form.ID)
		if w.ClosesAt == nil {
			upd = upd.ClearClosesAt()
		}
		if err := upd.
			SetDescription(w.Description).
			SetConfirmation(w.Confirmation).SetConfirmationURL(w.ConfirmationURL).
			SetSubmitLabel(w.SubmitLabel).
			SetClosed(w.Closed).
			SetNotifyEmail(w.NotifyEmail).
			SetLimitPerUser(w.LimitPerUser).
			SetMaxSubmissions(w.MaxSubmissions).
			SetNillableClosesAt(w.ClosesAt).
			Exec(ctx); err != nil {
			return err
		}
		if err := r.nodes.UpdateRow(ctx, w.Node); err != nil {
			return err
		}
		return saveWebformElements(ctx, c, form.ID, w.Elements)
	})
}

// saveWebformElements makes the form's elements those given, in their
// order: an element with an ID is updated (its machine name kept), one
// without is added, an existing one not given is deleted with its values.
func saveWebformElements(ctx context.Context, c *generated.Client, formID int, elements WebformElements) error {
	keep := map[int]bool{}
	for _, e := range elements {
		if e.ID > 0 {
			keep[e.ID] = true
		}
	}
	existing, err := c.WebformElement.Query().Where(webformelement.WebformID(formID)).All(ctx)
	if err != nil {
		return err
	}
	for _, e := range existing {
		if !keep[e.ID] {
			if err := c.WebformElement.DeleteOneID(e.ID).Exec(ctx); err != nil {
				return err
			}
		}
	}
	for i, e := range elements {
		options := strings.Join(e.Options, "\n")
		if e.ID > 0 {
			err = c.WebformElement.UpdateOneID(e.ID).
				Where(webformelement.WebformID(formID)).
				SetType(string(e.Type)).SetLabel(e.Label).SetDescription(e.Description).
				SetRequired(e.Required).SetOptions(options).SetWeight(i).
				SetVisibleIfElement(e.VisibleIfElement).SetVisibleIfValue(e.VisibleIfValue).
				SetPrefillParam(e.PrefillParam).
				Exec(ctx)
		} else {
			err = c.WebformElement.Create().
				SetWebformID(formID).SetMachineName(e.MachineName).
				SetType(string(e.Type)).SetLabel(e.Label).SetDescription(e.Description).
				SetRequired(e.Required).SetOptions(options).SetWeight(i).
				SetVisibleIfElement(e.VisibleIfElement).SetVisibleIfValue(e.VisibleIfValue).
				SetPrefillParam(e.PrefillParam).
				Exec(ctx)
		}
		if err != nil {
			return fmt.Errorf("element %q: %w", e.Label, err)
		}
	}
	return nil
}

// AddSubmission - see WebformRepository.AddSubmission.
func (r *EntWebformRepository) AddSubmission(ctx context.Context, s WebformSubmission) (int, error) {
	var id int
	err := modulekit.InTx(ctx, r.db, func(ctx context.Context) error {
		c := r.client(ctx)
		create := c.WebformSubmission.Create().SetWebformID(s.WebformID).SetRemoteIP(s.RemoteIP).SetSourceURL(s.SourceURL)
		if s.UserID != nil {
			create = create.SetUserID(*s.UserID)
		}
		if !s.CreatedAt.IsZero() {
			create = create.SetCreatedAt(s.CreatedAt)
		}
		sub, err := create.Save(ctx)
		if err != nil {
			return err
		}
		id = sub.ID
		for elementID, values := range s.Values {
			for delta, v := range values {
				if err := c.WebformSubmissionValue.Create().
					SetSubmissionID(sub.ID).SetElementID(elementID).SetDelta(delta).SetValue(v).
					Exec(ctx); err != nil {
					return err
				}
			}
		}
		return nil
	})
	return id, err
}

// CountSubmissions - see WebformRepository.CountSubmissions.
func (r *EntWebformRepository) CountSubmissions(ctx context.Context, webformID int) (int, error) {
	return r.client(ctx).WebformSubmission.Query().Where(webformsubmission.WebformID(webformID)).Count(ctx)
}

// Submissions - see WebformRepository.Submissions.
func (r *EntWebformRepository) Submissions(ctx context.Context, webformID, limit, offset int) ([]WebformSubmission, error) {
	rows, err := r.client(ctx).WebformSubmission.Query().
		Where(webformsubmission.WebformID(webformID)).
		Order(generated.Desc(webformsubmission.FieldCreatedAt), generated.Desc(webformsubmission.FieldID)).
		Limit(limit).Offset(offset).
		WithValues(func(q *generated.WebformSubmissionValueQuery) {
			q.Order(generated.Asc(webformsubmissionvalue.FieldDelta), generated.Asc(webformsubmissionvalue.FieldID))
		}).
		All(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]WebformSubmission, 0, len(rows))
	for _, s := range rows {
		out = append(out, toWebformSubmission(s))
	}
	return out, r.withAuthors(ctx, out)
}

// toWebformSubmission - a submission loaded with its values.
func toWebformSubmission(s *generated.WebformSubmission) WebformSubmission {
	sub := WebformSubmission{ID: s.ID, WebformID: s.WebformID, UserID: s.UserID, RemoteIP: s.RemoteIP, SourceURL: s.SourceURL, CreatedAt: s.CreatedAt, Values: map[int][]string{}}
	for _, v := range s.Edges.Values {
		sub.Values[v.ElementID] = append(sub.Values[v.ElementID], v.Value)
	}
	return sub
}

// withAuthors fills the submissions' authors' names and addresses (the
// core's users; a deleted one leaves them empty).
func (r *EntWebformRepository) withAuthors(ctx context.Context, subs []WebformSubmission) error {
	var ids []int
	for _, s := range subs {
		if s.UserID != nil {
			ids = append(ids, *s.UserID)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	users, err := r.users.ByIDs(ctx, ids)
	if err != nil {
		return fmt.Errorf("the submissions' authors: %w", err)
	}
	for i, s := range subs {
		if s.UserID == nil {
			continue
		}
		if u, ok := users[*s.UserID]; ok {
			subs[i].Username, subs[i].UserSlug = u.Username, u.Slug
		}
	}
	return nil
}

// Submission - see WebformRepository.Submission.
func (r *EntWebformRepository) Submission(ctx context.Context, webformID, id int) (*WebformSubmission, error) {
	s, err := r.client(ctx).WebformSubmission.Query().
		Where(webformsubmission.WebformID(webformID), webformsubmission.ID(id)).
		WithValues(func(q *generated.WebformSubmissionValueQuery) {
			q.Order(generated.Asc(webformsubmissionvalue.FieldDelta), generated.Asc(webformsubmissionvalue.FieldID))
		}).
		Only(ctx)
	if generated.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	sub := []WebformSubmission{toWebformSubmission(s)}
	if err := r.withAuthors(ctx, sub); err != nil {
		return nil, err
	}
	return &sub[0], nil
}

// SubmissionNeighbors - see WebformRepository.SubmissionNeighbors.
func (r *EntWebformRepository) SubmissionNeighbors(ctx context.Context, s WebformSubmission) (newer, older int, err error) {
	q := func() *generated.WebformSubmissionQuery {
		return r.client(ctx).WebformSubmission.Query().Where(webformsubmission.WebformID(s.WebformID))
	}
	// The list's order: created_at, then ID, both descending.
	newer, err = q().Where(webformsubmission.Or(
		webformsubmission.CreatedAtGT(s.CreatedAt),
		webformsubmission.And(webformsubmission.CreatedAtEQ(s.CreatedAt), webformsubmission.IDGT(s.ID)),
	)).Order(generated.Asc(webformsubmission.FieldCreatedAt), generated.Asc(webformsubmission.FieldID)).FirstID(ctx)
	if err != nil && !generated.IsNotFound(err) {
		return 0, 0, err
	}
	older, err = q().Where(webformsubmission.Or(
		webformsubmission.CreatedAtLT(s.CreatedAt),
		webformsubmission.And(webformsubmission.CreatedAtEQ(s.CreatedAt), webformsubmission.IDLT(s.ID)),
	)).Order(generated.Desc(webformsubmission.FieldCreatedAt), generated.Desc(webformsubmission.FieldID)).FirstID(ctx)
	if err != nil && !generated.IsNotFound(err) {
		return 0, 0, err
	}
	return newer, older, nil
}

// HasUserSubmission - see WebformRepository.HasUserSubmission.
func (r *EntWebformRepository) HasUserSubmission(ctx context.Context, webformID, userID int) (bool, error) {
	return r.client(ctx).WebformSubmission.Query().
		Where(webformsubmission.WebformID(webformID), webformsubmission.UserID(userID)).Exist(ctx)
}

// List - see WebformRepository.List.
func (r *EntWebformRepository) List(ctx context.Context) ([]Webform, error) {
	rows, err := r.client(ctx).Webform.Query().Select(webform.FieldID, webform.FieldNid).All(ctx)
	if err != nil {
		return nil, err
	}
	nids := make([]int, len(rows))
	for i, w := range rows {
		nids[i] = w.Nid
	}
	nodes, err := r.nodes.ByIDs(ctx, nids)
	if err != nil {
		return nil, fmt.Errorf("the forms' nodes: %w", err)
	}
	out := make([]Webform, 0, len(rows))
	for _, w := range rows {
		n, ok := nodes[w.Nid]
		if !ok {
			continue
		}
		out = append(out, Webform{ID: w.ID, Node: modulekit.Node{ID: n.ID, Title: n.Title, Slug: n.Slug, Status: n.Status}})
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].Node.Title) < strings.ToLower(out[j].Node.Title) })
	return out, nil
}

// DeleteSubmissions - see WebformRepository.DeleteSubmissions.
func (r *EntWebformRepository) DeleteSubmissions(ctx context.Context, webformID int, ids []int) (int, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	// Values go with them (ON DELETE CASCADE).
	return r.client(ctx).WebformSubmission.Delete().
		Where(webformsubmission.WebformID(webformID), webformsubmission.IDIn(ids...)).Exec(ctx)
}

// SubmissionStats - see WebformRepository.SubmissionStats.
func (r *EntWebformRepository) SubmissionStats(ctx context.Context) (map[int]WebformStats, error) {
	var rows []struct {
		WebformID int       `json:"webform_id"`
		Count     int       `json:"count"`
		Last      time.Time `json:"max"`
	}
	err := r.client(ctx).WebformSubmission.Query().
		GroupBy(webformsubmission.FieldWebformID).
		Aggregate(generated.Count(), generated.Max(webformsubmission.FieldCreatedAt)).
		Scan(ctx, &rows)
	if err != nil {
		return nil, err
	}
	out := make(map[int]WebformStats, len(rows))
	for _, row := range rows {
		out[row.WebformID] = WebformStats{Count: row.Count, Last: row.Last}
	}
	return out, nil
}

// AnswerCounts - see WebformRepository.AnswerCounts.
func (r *EntWebformRepository) AnswerCounts(ctx context.Context, elementIDs, choiceIDs []int) (WebformAnswerCounts, error) {
	out := WebformAnswerCounts{Answered: map[int]int{}, Values: map[int]map[string]int{}}
	if len(elementIDs) == 0 {
		return out, nil
	}
	client := r.client(ctx)
	// An answer's first value (several only for checkboxes) - one per
	// submission.
	var answered []struct {
		ElementID int `json:"element_id"`
		Count     int `json:"count"`
	}
	if err := client.WebformSubmissionValue.Query().
		Where(webformsubmissionvalue.ElementIDIn(elementIDs...), webformsubmissionvalue.Delta(0)).
		GroupBy(webformsubmissionvalue.FieldElementID).Aggregate(generated.Count()).Scan(ctx, &answered); err != nil {
		return out, err
	}
	for _, a := range answered {
		out.Answered[a.ElementID] = a.Count
	}
	if len(choiceIDs) == 0 {
		return out, nil
	}
	var values []struct {
		ElementID int    `json:"element_id"`
		Value     string `json:"value"`
		Count     int    `json:"count"`
	}
	if err := client.WebformSubmissionValue.Query().
		Where(webformsubmissionvalue.ElementIDIn(choiceIDs...)).
		GroupBy(webformsubmissionvalue.FieldElementID, webformsubmissionvalue.FieldValue).Aggregate(generated.Count()).Scan(ctx, &values); err != nil {
		return out, err
	}
	for _, v := range values {
		if out.Values[v.ElementID] == nil {
			out.Values[v.ElementID] = map[string]int{}
		}
		out.Values[v.ElementID][v.Value] = v.Count
	}
	return out, nil
}

// SubmissionIDsBefore - see WebformRepository.SubmissionIDsBefore.
func (r *EntWebformRepository) SubmissionIDsBefore(ctx context.Context, webformID int, t time.Time) ([]int, error) {
	return r.client(ctx).WebformSubmission.Query().
		Where(webformsubmission.WebformID(webformID), webformsubmission.CreatedAtLT(t)).IDs(ctx)
}

// SubmissionValues - see WebformRepository.SubmissionValues.
func (r *EntWebformRepository) SubmissionValues(ctx context.Context, submissionIDs, elementIDs []int) ([]string, error) {
	if len(submissionIDs) == 0 || len(elementIDs) == 0 {
		return nil, nil
	}
	return r.client(ctx).WebformSubmissionValue.Query().
		Where(webformsubmissionvalue.SubmissionIDIn(submissionIDs...), webformsubmissionvalue.ElementIDIn(elementIDs...)).
		Select(webformsubmissionvalue.FieldValue).Strings(ctx)
}

// SubmissionValue - see WebformRepository.SubmissionValue.
func (r *EntWebformRepository) SubmissionValue(ctx context.Context, webformID, submissionID, elementID, delta int) (string, error) {
	v, err := r.client(ctx).WebformSubmissionValue.Query().
		Where(
			webformsubmissionvalue.SubmissionID(submissionID), webformsubmissionvalue.ElementID(elementID), webformsubmissionvalue.Delta(delta),
			webformsubmissionvalue.HasSubmissionWith(webformsubmission.WebformID(webformID)),
		).Only(ctx)
	if generated.IsNotFound(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return v.Value, nil
}
