package privatemessages

import (
	"context"
	"database/sql"
	"fmt"
	"slices"
	"time"

	"github.com/kosatka-cms/kosatka/internal/modules/private_messages/ent/generated"
	"github.com/kosatka-cms/kosatka/internal/modules/private_messages/ent/generated/pmmessage"
	"github.com/kosatka-cms/kosatka/internal/modules/private_messages/ent/generated/pmthread"
	"github.com/kosatka-cms/kosatka/internal/modules/private_messages/ent/generated/pmthreadmember"
	"github.com/kosatka-cms/kosatka/internal/modules/private_messages/ent/generated/predicate"
	"github.com/kosatka-cms/kosatka/modulekit"
)

// The repositories of the threads, their messages and members: the
// module's own tables (pm_threads, pm_messages, pm_thread_members). A
// message's author and a member are the core's users, read through its
// repository.

var (
	_ ThreadRepository  = (*ThreadRepo)(nil)
	_ MessageRepository = (*MessageRepo)(nil)
	_ MemberRepository  = (*MemberRepo)(nil)
)

// store - the database and the core's users the repositories share.
type store struct {
	db    *sql.DB
	users modulekit.UserRepository
}

// client - the module's ent client: the context's transaction
// (modulekit.InTx), else the database.
func (s store) client(ctx context.Context) *generated.Client {
	return generated.NewClient(generated.Driver(modulekit.Driver(ctx, s.db)))
}

// attachUsers sets the messages' authors and the members' users, loaded
// in one query; a deleted user's stay nil.
func (s store) attachUsers(ctx context.Context, msgs []*Message, members []*Member) error {
	var ids []int
	for _, m := range msgs {
		if m.AuthorID != 0 {
			ids = append(ids, m.AuthorID)
		}
	}
	for _, m := range members {
		ids = append(ids, m.UserID)
	}
	if len(ids) == 0 {
		return nil
	}
	slices.Sort(ids)
	users, err := s.users.ByIDs(ctx, slices.Compact(ids))
	if err != nil {
		return fmt.Errorf("loading the users: %w", err)
	}
	for _, m := range msgs {
		if u, ok := users[m.AuthorID]; ok {
			m.Author = &u
		}
	}
	for _, m := range members {
		if u, ok := users[m.UserID]; ok {
			m.User = &u
		}
	}
	return nil
}

// threadsUsers - attachUsers for the threads' messages (the last one too)
// and members.
func (s store) threadsUsers(ctx context.Context, threads []Thread) error {
	var msgs []*Message
	var members []*Member
	for i := range threads {
		t := &threads[i]
		for j := range t.Messages {
			msgs = append(msgs, &t.Messages[j])
		}
		if t.LastMessage != nil {
			msgs = append(msgs, t.LastMessage)
		}
		for j := range t.Members {
			members = append(members, &t.Members[j])
		}
	}
	return s.attachUsers(ctx, msgs, members)
}

// ThreadRepo - ThreadRepository on pm_threads.
type ThreadRepo struct{ store }

// NewThreadRepo - the threads (ThreadRepository) of db; users - the
// core's.
func NewThreadRepo(db *sql.DB, users modulekit.UserRepository) *ThreadRepo {
	return &ThreadRepo{store{db, users}}
}

// ByID - the thread with its members (who may open it); its messages are
// read a page at a time (MessageRepo.List), not here.
func (r *ThreadRepo) ByID(ctx context.Context, id int) (*Thread, error) {
	res, err := r.client(ctx).PmThread.Query().
		Where(pmthread.ID(id)).
		WithMembers().
		First(ctx)

	if err != nil {
		return nil, fmt.Errorf("loading thread %d: %w", id, err)
	}

	threads := []Thread{toThread(res)}
	if err := r.threadsUsers(ctx, threads); err != nil {
		return nil, fmt.Errorf("loading thread %d: %w", id, err)
	}
	return &threads[0], nil
}

// Create stores the thread and its members, each once and with news for
// them; its messages are added by MessageRepo.Create.
func (r *ThreadRepo) Create(ctx context.Context, thread Thread, memberIDs []int) (*Thread, error) {
	now := time.Now()
	c := r.client(ctx)

	res, err := c.PmThread.Create().
		SetSubject(thread.Subject).
		SetCreatedAt(now).
		SetUpdatedAt(now).
		Save(ctx)

	if err != nil {
		return nil, err
	}

	// Add members, each once.
	added := make(map[int]bool, len(memberIDs))
	for _, uid := range memberIDs {
		if added[uid] {
			continue
		}
		added[uid] = true
		_, err := c.PmThreadMember.Create().
			SetThreadID(res.ID).
			SetUserID(uid).
			SetIsNew(true).
			SetDeleted(false).
			SetJoinedAt(now).
			Save(ctx)
		if err != nil {
			return nil, err
		}
	}

	created := toThread(res)
	return &created, nil
}

// TotalCount - how many threads match filter (its Limit and Offset
// aside).
func (r *ThreadRepo) TotalCount(ctx context.Context, filter ThreadFilter) (int, error) {
	return r.client(ctx).PmThread.Query().Where(threadsWhere(filter)...).Count(ctx)
}

// threadsWhere - filter's conditions, the same for List and TotalCount:
// threads of UserID (with their Deleted and IsNew marks - their own) and
// of ParticipantID.
func threadsWhere(filter ThreadFilter) []predicate.PmThread {
	var where []predicate.PmThread
	if filter.UserID != 0 {
		memberPreds := []predicate.PmThreadMember{pmthreadmember.UserID(filter.UserID)}
		if filter.Deleted != nil {
			memberPreds = append(memberPreds, pmthreadmember.Deleted(*filter.Deleted))
		}
		if filter.IsNew != nil {
			memberPreds = append(memberPreds, pmthreadmember.IsNew(*filter.IsNew))
		}
		where = append(where, pmthread.HasMembersWith(memberPreds...))
	}
	if filter.ParticipantID != 0 {
		where = append(where, pmthread.HasMembersWith(pmthreadmember.UserID(filter.ParticipantID)))
	}
	return where
}

// List - the threads matching filter, most recently updated first, with
// their members and last messages and those users.
func (r *ThreadRepo) List(ctx context.Context, filter ThreadFilter) (*Threads, error) {
	query := r.client(ctx).PmThread.Query().
		Where(threadsWhere(filter)...).
		WithMembers().
		Order(generated.Desc(pmthread.FieldUpdatedAt))

	if filter.Limit > 0 {
		query = query.Limit(filter.Limit).Offset(filter.Offset)
	}

	res, err := query.All(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing threads: %w", err)
	}

	var threads Threads
	for _, t := range res {
		threads = append(threads, toThread(t))
	}
	if err := r.lastMessages(ctx, threads); err != nil {
		return nil, fmt.Errorf("listing threads: %w", err)
	}
	if err := r.threadsUsers(ctx, threads); err != nil {
		return nil, fmt.Errorf("listing threads: %w", err)
	}

	return &threads, nil
}

// lastMessages sets each thread's LastMessage: its newest message (the
// newest IDs in one query, those messages in another - a limit on an
// eager-loaded edge limits all the threads' messages together).
func (r *ThreadRepo) lastMessages(ctx context.Context, threads Threads) error {
	if len(threads) == 0 {
		return nil
	}
	ids := make([]int, len(threads))
	for i, t := range threads {
		ids[i] = t.ID
	}
	var newest []struct {
		ThreadID int `json:"thread_id"`
		Max      int `json:"max"`
	}
	if err := r.client(ctx).PmMessage.Query().Where(pmmessage.ThreadIDIn(ids...)).
		GroupBy(pmmessage.FieldThreadID).Aggregate(generated.Max(pmmessage.FieldID)).Scan(ctx, &newest); err != nil {
		return fmt.Errorf("finding the threads' last messages: %w", err)
	}
	msgIDs := make([]int, len(newest))
	for i, n := range newest {
		msgIDs[i] = n.Max
	}
	rows, err := r.client(ctx).PmMessage.Query().Where(pmmessage.IDIn(msgIDs...)).All(ctx)
	if err != nil {
		return fmt.Errorf("loading the threads' last messages: %w", err)
	}
	last := make(map[int]Message, len(rows))
	for _, row := range rows {
		last[row.ThreadID] = toMessage(row)
	}
	for i := range threads {
		if m, ok := last[threads[i].ID]; ok {
			threads[i].LastMessage = &m
		}
	}
	return nil
}

// MessageRepo - MessageRepository on pm_messages.
type MessageRepo struct{ store }

// NewMessageRepo - the messages (MessageRepository) of db; users - the
// core's.
func NewMessageRepo(db *sql.DB, users modulekit.UserRepository) *MessageRepo {
	return &MessageRepo{store{db, users}}
}

// List - a page of the messages matching filter, oldest first, with
// their authors and the total count.
func (r *MessageRepo) List(ctx context.Context, filter MessageFilter, pagination Pagination) (*MessagePage, error) {
	query := r.client(ctx).PmMessage.Query().
		Order(generated.Asc(pmmessage.FieldCreatedAt))

	if filter.ThreadID != 0 {
		query = query.Where(pmmessage.ThreadID(filter.ThreadID))
	}
	if filter.AuthorID != nil {
		query = query.Where(pmmessage.AuthorID(*filter.AuthorID))
	}

	total, err := query.Count(ctx)
	if err != nil {
		return nil, fmt.Errorf("counting messages: %w", err)
	}

	res, err := query.
		Limit(pagination.Limit).
		Offset(pagination.Offset).
		All(ctx)
	if err != nil {
		return nil, err
	}

	msgs := make(Messages, 0, len(res))
	for _, m := range res {
		msgs = append(msgs, toMessage(m))
	}
	ptrs := make([]*Message, len(msgs))
	for i := range msgs {
		ptrs[i] = &msgs[i]
	}
	if err := r.attachUsers(ctx, ptrs, nil); err != nil {
		return nil, fmt.Errorf("listing messages: %w", err)
	}

	return &MessagePage{
		Messages: msgs,
		Total:    total,
		Limit:    pagination.Limit,
		Offset:   pagination.Offset,
	}, nil
}

// Create stores the message, bumps its thread's update time and marks
// the thread new for its members but the author.
func (r *MessageRepo) Create(ctx context.Context, msg Message) (*Message, error) {
	c := r.client(ctx)
	q := c.PmMessage.Create().
		SetThreadID(msg.ThreadID).
		SetAuthorID(msg.AuthorID).
		SetBody(msg.Body).
		SetCreatedAt(time.Now())

	res, err := q.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("creating message: %w", err)
	}

	// Bump the thread's updated_at.
	err = c.PmThread.UpdateOneID(msg.ThreadID).
		SetUpdatedAt(time.Now()).
		Exec(ctx)
	if err != nil {
		return nil, err
	}

	// Mark the thread as new for every member except the author.
	err = c.PmThreadMember.Update().
		Where(
			pmthreadmember.ThreadID(msg.ThreadID),
			pmthreadmember.Not(pmthreadmember.UserID(msg.AuthorID)),
		).
		SetIsNew(true).
		Exec(ctx)
	if err != nil {
		return nil, err
	}

	// The author has read their own message.
	err = c.PmThreadMember.Update().
		Where(
			pmthreadmember.ThreadID(msg.ThreadID),
			pmthreadmember.UserID(msg.AuthorID),
		).
		SetIsNew(false).
		Exec(ctx)
	if err != nil {
		return nil, err
	}

	created := toMessage(res)
	return &created, nil
}

// TotalCount - how many messages match filter.
func (r *MessageRepo) TotalCount(ctx context.Context, filter MessageFilter) (int, error) {
	query := r.client(ctx).PmMessage.Query()

	if filter.ThreadID != 0 {
		query = query.Where(pmmessage.ThreadID(filter.ThreadID))
	}
	if filter.AuthorID != nil {
		query = query.Where(pmmessage.AuthorID(*filter.AuthorID))
	}

	return query.Count(ctx)
}

// DeleteByAuthor deletes the messages the user wrote, their memberships
// and the threads left with no messages, in one transaction. Other
// members' messages stay; their replies to the deleted ones lose the link.
func (r *MessageRepo) DeleteByAuthor(ctx context.Context, userID int) error {
	return modulekit.InTx(ctx, r.db, func(ctx context.Context) error {
		c := r.client(ctx)
		// Remember the user's threads so that only those left without
		// messages are removed afterwards.
		memberOf, err := c.PmThreadMember.Query().Where(pmthreadmember.UserID(userID)).
			Select(pmthreadmember.FieldThreadID).Ints(ctx)
		if err != nil {
			return fmt.Errorf("the user's threads: %w", err)
		}
		wroteIn, err := c.PmMessage.Query().Where(pmmessage.AuthorID(userID)).
			Select(pmmessage.FieldThreadID).Ints(ctx)
		if err != nil {
			return fmt.Errorf("the threads the user wrote in: %w", err)
		}
		threadIDs := append(memberOf, wroteIn...)
		slices.Sort(threadIDs)
		threadIDs = slices.Compact(threadIDs)

		if _, err := c.PmMessage.Delete().Where(pmmessage.AuthorID(userID)).Exec(ctx); err != nil {
			return fmt.Errorf("deleting the messages: %w", err)
		}
		if _, err := c.PmThreadMember.Delete().Where(pmthreadmember.UserID(userID)).Exec(ctx); err != nil {
			return fmt.Errorf("deleting the memberships: %w", err)
		}
		if len(threadIDs) == 0 {
			return nil
		}
		empty := pmthread.And(pmthread.IDIn(threadIDs...), pmthread.Not(pmthread.HasMessages()))
		if _, err := c.PmThreadMember.Delete().Where(pmthreadmember.HasThreadWith(empty)).Exec(ctx); err != nil {
			return fmt.Errorf("deleting the empty threads' members: %w", err)
		}
		if _, err := c.PmThread.Delete().Where(empty).Exec(ctx); err != nil {
			return fmt.Errorf("deleting the empty threads: %w", err)
		}
		return nil
	})
}

// MemberRepo - MemberRepository on pm_thread_members.
type MemberRepo struct{ store }

// NewMemberRepo - the thread members (MemberRepository) of db; users -
// the core's.
func NewMemberRepo(db *sql.DB, users modulekit.UserRepository) *MemberRepo {
	return &MemberRepo{store{db, users}}
}

// MarkRead marks the thread read for the user: no news for them.
func (r *MemberRepo) MarkRead(ctx context.Context, threadID int, userID int) error {
	return r.client(ctx).PmThreadMember.Update().
		Where(
			pmthreadmember.ThreadID(threadID),
			pmthreadmember.UserID(userID),
		).
		SetIsNew(false).
		Exec(ctx)
}

// Archive moves the thread to the user's archive (Member.Deleted).
func (r *MemberRepo) Archive(ctx context.Context, threadID int, userID int) error {
	return r.client(ctx).PmThreadMember.Update().
		Where(
			pmthreadmember.ThreadID(threadID),
			pmthreadmember.UserID(userID),
		).
		SetDeleted(true).
		Exec(ctx)
}

// Unarchive brings the thread back from the user's archive.
func (r *MemberRepo) Unarchive(ctx context.Context, threadID int, userID int) error {
	return r.client(ctx).PmThreadMember.Update().
		Where(
			pmthreadmember.ThreadID(threadID),
			pmthreadmember.UserID(userID),
		).
		SetDeleted(false).
		Exec(ctx)
}
