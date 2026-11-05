// Package subscriptions - the subscriptions module: users follow a
// content item's comments or a term's new content; what happens there
// becomes their notifications, on the site (the profile's Notifications
// tab, the user menu) and by e-mail (at once, a daily digest or never).
package subscriptions

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/kosatka-cms/kosatka/internal/modules/subscriptions/ent/generated"
	"github.com/kosatka-cms/kosatka/internal/modules/subscriptions/ent/generated/subscription"
	"github.com/kosatka-cms/kosatka/internal/modules/subscriptions/ent/generated/subscriptionnotification"
	"github.com/kosatka-cms/kosatka/internal/modules/subscriptions/ent/generated/subscriptionsettings"
	"github.com/kosatka-cms/kosatka/internal/modules/subscriptions/ent/generated/subscriptionusersettings"
	"github.com/kosatka-cms/kosatka/modulekit"
	"github.com/kosatka-cms/kosatka/pkg/randtoken"
)

// The kinds of notifications.
const (
	// KindReply - a reply to the user's comment.
	KindReply = "reply"
	// KindComment - a comment on content the user follows.
	KindComment = "comment"
	// KindContent - new content with a term the user follows.
	KindContent = "content"
)

// The e-mail states of a notification.
const (
	mailWaiting int8 = 0
	mailSent    int8 = 1
	mailSkipped int8 = 2
)

// The users' e-mail modes.
const (
	MailNow   = "now"
	MailDaily = "daily"
	MailNever = "never"
)

// Target - what a subscription follows: a content item's comments
// (NodeID) or a term's new content (TermID).
type Target struct {
	NodeID int
	TermID int
}

// Subscription - a user following a Target.
type Subscription struct {
	ID     int
	UserID int
	Target
	Token string
}

// Notification - something told to a user.
type Notification struct {
	ID        int
	UserID    int
	Kind      string
	NodeID    int
	CommentID int
	ActorID   int
	TermID    int
	CreatedAt time.Time
	Read      bool
}

// UserSettings - a user's settings of the module; the zero row's are the
// defaults (DefaultUserSettings).
type UserSettings struct {
	Mail          string
	Replies       bool
	AutoOwn       bool
	AutoCommented bool
}

// DefaultUserSettings - the settings of a user who never saved theirs.
func DefaultUserSettings() UserSettings {
	return UserSettings{Mail: MailNow, Replies: true, AutoOwn: true, AutoCommented: true}
}

// Repo - the module's tables.
type Repo struct{ db *sql.DB }

// NewRepo - the module's tables on db.
func NewRepo(db *sql.DB) *Repo { return &Repo{db: db} }

func (r *Repo) client(ctx context.Context) *generated.Client {
	return generated.NewClient(generated.Driver(modulekit.Driver(ctx, r.db)))
}

// find - userID's row for t; nil without one.
func (r *Repo) find(ctx context.Context, userID int, t Target) (*generated.Subscription, error) {
	q := r.client(ctx).Subscription.Query().Where(subscription.UserID(userID))
	if t.NodeID > 0 {
		q = q.Where(subscription.NodeID(t.NodeID))
	} else {
		q = q.Where(subscription.TermID(t.TermID))
	}
	row, err := q.Only(ctx)
	if generated.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("the subscription of user %d to %+v: %w", userID, t, err)
	}
	return row, nil
}

// Subscribed - userID follows t (and didn't unsubscribe).
func (r *Repo) Subscribed(ctx context.Context, userID int, t Target) (bool, error) {
	row, err := r.find(ctx, userID, t)
	return row != nil && !row.Muted, err
}

// set makes userID follow t (on) or not; auto - only when they never
// subscribed nor unsubscribed (the automatic subscriptions).
func (r *Repo) set(ctx context.Context, userID int, t Target, on, auto bool) error {
	row, err := r.find(ctx, userID, t)
	if err != nil {
		return err
	}
	c := r.client(ctx)
	if row != nil {
		if auto || row.Muted == !on {
			return nil
		}
		if err := c.Subscription.UpdateOneID(row.ID).SetMuted(!on).Exec(ctx); err != nil {
			return fmt.Errorf("updating the subscription of user %d to %+v: %w", userID, t, err)
		}
		return nil
	}
	token, err := randtoken.Generate(16)
	if err != nil {
		return fmt.Errorf("a subscription's token: %w", err)
	}
	create := c.Subscription.Create().SetUserID(userID).SetMuted(!on).SetToken(token).SetCreatedAt(time.Now())
	if t.NodeID > 0 {
		create = create.SetNodeID(t.NodeID)
	} else {
		create = create.SetTermID(t.TermID)
	}
	if err := create.Exec(ctx); err != nil {
		return fmt.Errorf("subscribing user %d to %+v: %w", userID, t, err)
	}
	return nil
}

// Subscribe makes userID follow t.
func (r *Repo) Subscribe(ctx context.Context, userID int, t Target) error {
	return r.set(ctx, userID, t, true, false)
}

// Unsubscribe makes userID stop following t: the row stays muted, so an
// automatic subscription doesn't come back.
func (r *Repo) Unsubscribe(ctx context.Context, userID int, t Target) error {
	return r.set(ctx, userID, t, false, false)
}

// AutoSubscribe makes userID follow t unless they ever subscribed to it
// or unsubscribed from it.
func (r *Repo) AutoSubscribe(ctx context.Context, userID int, t Target) error {
	return r.set(ctx, userID, t, true, true)
}

// Followers - the users following t, with their subscriptions (by user).
func (r *Repo) Followers(ctx context.Context, t Target) (map[int]Subscription, error) {
	q := r.client(ctx).Subscription.Query().Where(subscription.Muted(false))
	if t.NodeID > 0 {
		q = q.Where(subscription.NodeID(t.NodeID))
	} else {
		q = q.Where(subscription.TermID(t.TermID))
	}
	rows, err := q.All(ctx)
	if err != nil {
		return nil, fmt.Errorf("the followers of %+v: %w", t, err)
	}
	out := make(map[int]Subscription, len(rows))
	for _, row := range rows {
		out[row.UserID] = toSubscription(row)
	}
	return out, nil
}

// TermFollowers - the users following any of termIDs, with one of the
// subscriptions each (the first term's of the list they follow).
func (r *Repo) TermFollowers(ctx context.Context, termIDs []int) (map[int]Subscription, error) {
	out := map[int]Subscription{}
	if len(termIDs) == 0 {
		return out, nil
	}
	rows, err := r.client(ctx).Subscription.Query().Where(subscription.Muted(false), subscription.TermIDIn(termIDs...)).All(ctx)
	if err != nil {
		return nil, fmt.Errorf("the followers of terms %v: %w", termIDs, err)
	}
	order := make(map[int]int, len(termIDs))
	for i, id := range termIDs {
		order[id] = i
	}
	for _, row := range rows {
		s := toSubscription(row)
		if prev, ok := out[row.UserID]; !ok || order[s.TermID] < order[prev.TermID] {
			out[row.UserID] = s
		}
	}
	return out, nil
}

// UserSubscriptions - what userID follows, newest first.
func (r *Repo) UserSubscriptions(ctx context.Context, userID int) ([]Subscription, error) {
	rows, err := r.client(ctx).Subscription.Query().
		Where(subscription.UserID(userID), subscription.Muted(false)).
		Order(generated.Desc(subscription.FieldID)).All(ctx)
	if err != nil {
		return nil, fmt.Errorf("the subscriptions of user %d: %w", userID, err)
	}
	out := make([]Subscription, len(rows))
	for i, row := range rows {
		out[i] = toSubscription(row)
	}
	return out, nil
}

// SubscriptionByID - userID's subscription id; nil - none of theirs.
func (r *Repo) SubscriptionByID(ctx context.Context, userID, id int) (*Subscription, error) {
	row, err := r.client(ctx).Subscription.Query().Where(subscription.ID(id), subscription.UserID(userID)).Only(ctx)
	if generated.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("subscription %d: %w", id, err)
	}
	s := toSubscription(row)
	return &s, nil
}

// SubscriptionByToken - the subscription of an e-mail's unsubscribe link;
// nil - none.
func (r *Repo) SubscriptionByToken(ctx context.Context, token string) (*Subscription, error) {
	row, err := r.client(ctx).Subscription.Query().Where(subscription.Token(token)).Only(ctx)
	if generated.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("the subscription of a token: %w", err)
	}
	s := toSubscription(row)
	return &s, nil
}

func toSubscription(row *generated.Subscription) Subscription {
	s := Subscription{ID: row.ID, UserID: row.UserID, Token: row.Token}
	if row.NodeID != nil {
		s.NodeID = *row.NodeID
	}
	if row.TermID != nil {
		s.TermID = *row.TermID
	}
	return s
}

// AddNotifications stores ns; mailed - whether each user's are to be
// e-mailed (their mode isn't "never").
func (r *Repo) AddNotifications(ctx context.Context, ns []Notification, mailed map[int]bool) error {
	if len(ns) == 0 {
		return nil
	}
	c := r.client(ctx)
	builders := make([]*generated.SubscriptionNotificationCreate, len(ns))
	for i, n := range ns {
		state := mailSkipped
		if mailed[n.UserID] {
			state = mailWaiting
		}
		b := c.SubscriptionNotification.Create().SetUserID(n.UserID).SetKind(n.Kind).SetNodeID(n.NodeID).
			SetCreatedAt(n.CreatedAt).SetMailState(state)
		if n.CommentID > 0 {
			b = b.SetCommentID(n.CommentID)
		}
		if n.ActorID > 0 {
			b = b.SetActorID(n.ActorID)
		}
		if n.TermID > 0 {
			b = b.SetTermID(n.TermID)
		}
		builders[i] = b
	}
	if err := c.SubscriptionNotification.CreateBulk(builders...).Exec(ctx); err != nil {
		return fmt.Errorf("storing %d notifications: %w", len(ns), err)
	}
	return nil
}

// ToldOfContent - those of userIDs already told of the new content
// nodeID (a node published again tells nobody twice).
func (r *Repo) ToldOfContent(ctx context.Context, nodeID int, userIDs []int) (map[int]bool, error) {
	out := map[int]bool{}
	if len(userIDs) == 0 {
		return out, nil
	}
	ids, err := r.client(ctx).SubscriptionNotification.Query().
		Where(subscriptionnotification.NodeID(nodeID), subscriptionnotification.Kind(KindContent), subscriptionnotification.UserIDIn(userIDs...)).
		Select(subscriptionnotification.FieldUserID).Ints(ctx)
	if err != nil {
		return nil, fmt.Errorf("who was told of node %d: %w", nodeID, err)
	}
	for _, id := range ids {
		out[id] = true
	}
	return out, nil
}

// Notifications - a page of userID's notifications, newest first, and
// how many they have.
func (r *Repo) Notifications(ctx context.Context, userID, offset, limit int) ([]Notification, int, error) {
	c := r.client(ctx)
	total, err := c.SubscriptionNotification.Query().Where(subscriptionnotification.UserID(userID)).Count(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("counting the notifications of user %d: %w", userID, err)
	}
	rows, err := c.SubscriptionNotification.Query().Where(subscriptionnotification.UserID(userID)).
		Order(generated.Desc(subscriptionnotification.FieldID)).Offset(offset).Limit(limit).All(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("the notifications of user %d: %w", userID, err)
	}
	return toNotifications(rows), total, nil
}

// Notification - userID's notification id; nil - none of theirs.
func (r *Repo) Notification(ctx context.Context, userID, id int) (*Notification, error) {
	row, err := r.client(ctx).SubscriptionNotification.Query().
		Where(subscriptionnotification.ID(id), subscriptionnotification.UserID(userID)).Only(ctx)
	if generated.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("notification %d: %w", id, err)
	}
	return &toNotifications([]*generated.SubscriptionNotification{row})[0], nil
}

func toNotifications(rows []*generated.SubscriptionNotification) []Notification {
	out := make([]Notification, len(rows))
	for i, row := range rows {
		n := Notification{ID: row.ID, UserID: row.UserID, Kind: row.Kind, NodeID: row.NodeID, CreatedAt: row.CreatedAt, Read: row.ReadAt != nil}
		if row.CommentID != nil {
			n.CommentID = *row.CommentID
		}
		if row.ActorID != nil {
			n.ActorID = *row.ActorID
		}
		if row.TermID != nil {
			n.TermID = *row.TermID
		}
		out[i] = n
	}
	return out
}

// UnreadCount - userID's notifications not opened yet.
func (r *Repo) UnreadCount(ctx context.Context, userID int) (int, error) {
	n, err := r.client(ctx).SubscriptionNotification.Query().
		Where(subscriptionnotification.UserID(userID), subscriptionnotification.ReadAtIsNil()).Count(ctx)
	if err != nil {
		return 0, fmt.Errorf("counting the unread notifications of user %d: %w", userID, err)
	}
	return n, nil
}

// MarkRead marks n read with the unread ones of the same content and
// kind (a group of comments is read at once).
func (r *Repo) MarkRead(ctx context.Context, n Notification) error {
	if _, err := r.client(ctx).SubscriptionNotification.Update().
		Where(subscriptionnotification.UserID(n.UserID), subscriptionnotification.NodeID(n.NodeID),
			subscriptionnotification.Kind(n.Kind), subscriptionnotification.ReadAtIsNil()).
		SetReadAt(time.Now()).Save(ctx); err != nil {
		return fmt.Errorf("marking notification %d read: %w", n.ID, err)
	}
	return nil
}

// MarkAllRead marks every notification of userID read.
func (r *Repo) MarkAllRead(ctx context.Context, userID int) error {
	if _, err := r.client(ctx).SubscriptionNotification.Update().
		Where(subscriptionnotification.UserID(userID), subscriptionnotification.ReadAtIsNil()).
		SetReadAt(time.Now()).Save(ctx); err != nil {
		return fmt.Errorf("marking the notifications of user %d read: %w", userID, err)
	}
	return nil
}

// WaitingMail - the notifications waiting for their e-mail, by user,
// oldest first.
func (r *Repo) WaitingMail(ctx context.Context) (map[int][]Notification, error) {
	rows, err := r.client(ctx).SubscriptionNotification.Query().
		Where(subscriptionnotification.MailState(mailWaiting)).
		Order(generated.Asc(subscriptionnotification.FieldID)).All(ctx)
	if err != nil {
		return nil, fmt.Errorf("the notifications waiting for e-mail: %w", err)
	}
	out := map[int][]Notification{}
	for _, n := range toNotifications(rows) {
		out[n.UserID] = append(out[n.UserID], n)
	}
	return out, nil
}

// SetMailState sets the e-mail state of the notifications ids.
func (r *Repo) setMailState(ctx context.Context, ids []int, state int8) error {
	if len(ids) == 0 {
		return nil
	}
	if _, err := r.client(ctx).SubscriptionNotification.Update().
		Where(subscriptionnotification.IDIn(ids...)).SetMailState(state).Save(ctx); err != nil {
		return fmt.Errorf("the e-mail state of notifications %v: %w", ids, err)
	}
	return nil
}

// DeleteOlder deletes the notifications from before t; how many.
func (r *Repo) DeleteOlder(ctx context.Context, t time.Time) (int, error) {
	n, err := r.client(ctx).SubscriptionNotification.Delete().
		Where(subscriptionnotification.CreatedAtLT(t)).Exec(ctx)
	if err != nil {
		return 0, fmt.Errorf("deleting the notifications before %v: %w", t, err)
	}
	return n, nil
}

// UserSettings - the settings of userIDs (the defaults for those who
// never saved theirs).
func (r *Repo) UserSettings(ctx context.Context, userIDs []int) (map[int]UserSettings, error) {
	out := make(map[int]UserSettings, len(userIDs))
	for _, id := range userIDs {
		out[id] = DefaultUserSettings()
	}
	if len(userIDs) == 0 {
		return out, nil
	}
	rows, err := r.client(ctx).SubscriptionUserSettings.Query().Where(subscriptionusersettings.UserIDIn(userIDs...)).All(ctx)
	if err != nil {
		return nil, fmt.Errorf("the notification settings of users %v: %w", userIDs, err)
	}
	for _, row := range rows {
		out[row.UserID] = UserSettings{Mail: row.Mail, Replies: row.Replies, AutoOwn: row.AutoOwn, AutoCommented: row.AutoCommented}
	}
	return out, nil
}

// SaveUserSettings stores userID's settings (the row made on the first
// save).
func (r *Repo) SaveUserSettings(ctx context.Context, userID int, s UserSettings) error {
	c := r.client(ctx)
	n, err := c.SubscriptionUserSettings.Update().Where(subscriptionusersettings.UserID(userID)).
		SetMail(s.Mail).SetReplies(s.Replies).SetAutoOwn(s.AutoOwn).SetAutoCommented(s.AutoCommented).Save(ctx)
	if err == nil && n == 0 {
		err = c.SubscriptionUserSettings.Create().SetUserID(userID).
			SetMail(s.Mail).SetReplies(s.Replies).SetAutoOwn(s.AutoOwn).SetAutoCommented(s.AutoCommented).Exec(ctx)
	}
	if err != nil {
		return fmt.Errorf("saving the notification settings of user %d: %w", userID, err)
	}
	return nil
}

// settingsRow - the module's settings row; nil - never saved.
func (r *Repo) settingsRow(ctx context.Context) (*generated.SubscriptionSettings, error) {
	row, err := r.client(ctx).SubscriptionSettings.Get(ctx, 1)
	if generated.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("the subscriptions' settings: %w", err)
	}
	return row, nil
}

// updateSettings applies set to the settings row (made on the first
// save).
func (r *Repo) updateSettings(ctx context.Context, set func(*generated.SubscriptionSettingsMutation)) error {
	c := r.client(ctx)
	u := c.SubscriptionSettings.Update().Where(subscriptionsettings.ID(1))
	set(u.Mutation())
	n, err := u.Save(ctx)
	if err == nil && n == 0 {
		create := c.SubscriptionSettings.Create().SetID(1)
		set(create.Mutation())
		err = create.Exec(ctx)
	}
	if err != nil {
		return fmt.Errorf("saving the subscriptions' settings: %w", err)
	}
	return nil
}
