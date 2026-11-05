// Package privatemessages - the private messages module: conversations
// (threads) between users, their members and messages; the profile's
// messages tab, the thread and new message pages, the e-mail of a new
// message, the unread count in the user menu.
//
// Its tables (pm_threads, pm_thread_members, pm_messages) are its own ent
// schema (ent/); the users are the core's.
package privatemessages

import (
	"context"
	"time"

	"github.com/kosatka-cms/kosatka/modulekit"
)

// --- Thread ---

// Threads - a list of threads (ThreadRepository.List).
type Threads []Thread

// Thread - a conversation.
type Thread struct {
	ID        int
	Subject   string
	CreatedAt time.Time
	UpdatedAt time.Time
	// Edges
	Messages []Message
	Members  []Member
	// Computed
	UnreadCount int
	LastMessage *Message
	IsUnread    bool
}

// HasMember reports whether the user is one of the thread's members,
// whether or not they archived it (Member.Deleted is the archive: an
// archived thread is still theirs to open and reply to).
func (t Thread) HasMember(userID int) bool {
	for _, m := range t.Members {
		if m.UserID == userID {
			return true
		}
	}
	return false
}

// ThreadFilter - which threads to list or count: a member's (UserID, by
// their IsNew and Deleted state), those ParticipantID is in too, and the
// page of the list (Limit 0 - all).
type ThreadFilter struct {
	UserID        int
	ParticipantID int // show threads this user participates in
	IsNew         *bool
	Deleted       *bool // nil = all, true = archived only, false = active only
	Limit         int
	Offset        int
}

// Pagination - limit/offset paging parameters.
type Pagination struct {
	Limit  int
	Offset int
}

// MessagePage - a page of messages with the total count.
type MessagePage struct {
	Messages Messages
	Total    int
	Limit    int
	Offset   int
}

// ThreadRepository - the threads: one with its members, a filtered list
// and count, a new one with its members.
type ThreadRepository interface {
	ByID(ctx context.Context, id int) (*Thread, error)
	List(ctx context.Context, filter ThreadFilter) (*Threads, error)
	TotalCount(ctx context.Context, filter ThreadFilter) (int, error)
	Create(ctx context.Context, thread Thread, memberIDs []int) (*Thread, error)
}

// --- Message ---

// Messages - a list of messages (MessagePage.Messages).
type Messages []Message

// Message - a message of a thread.
type Message struct {
	ID        int
	ThreadID  int
	AuthorID  int
	Body      string
	CreatedAt time.Time
	// Edges
	Author *modulekit.User
}

// MessageFilter - which messages to list or count: a thread's (ThreadID;
// 0 - any), an author's (AuthorID; nil - anyone's).
type MessageFilter struct {
	ThreadID int
	AuthorID *int
}

// MessageRepository - the messages: a filtered page of them, a new one,
// their count.
type MessageRepository interface {
	List(ctx context.Context, filter MessageFilter, pagination Pagination) (*MessagePage, error)
	Create(ctx context.Context, msg Message) (*Message, error)
	TotalCount(ctx context.Context, filter MessageFilter) (int, error)
}

// --- Member ---

// Members - a list of thread members.
type Members []Member

// Member - a user's membership of a thread: whether it has news for them
// and whether they archived it.
type Member struct {
	ThreadID int
	UserID   int
	IsNew    bool
	Deleted  bool
	JoinedAt time.Time
	// Edges
	User   *modulekit.User
	Thread *Thread
}

// MemberRepository - a user's membership of a thread: marking it read,
// archiving it and bringing it back.
type MemberRepository interface {
	MarkRead(ctx context.Context, threadID int, userID int) error
	Archive(ctx context.Context, threadID int, userID int) error
	Unarchive(ctx context.Context, threadID int, userID int) error
}
