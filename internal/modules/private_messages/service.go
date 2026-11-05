package privatemessages

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/kosatka-cms/kosatka/modulekit"
)

// Service - the threads, their messages and members.
type Service interface {
	// Threads
	ThreadByID(ctx context.Context, id int) (*Thread, error)
	ThreadList(ctx context.Context, filter ThreadFilter) (*Threads, error)
	ThreadCreate(ctx context.Context, authorID int, subject string, memberIDs []int, firstMessage string) (*Thread, error)

	// Messages
	MessageList(ctx context.Context, filter MessageFilter, pagination Pagination) (*MessagePage, error)
	MessageCreate(ctx context.Context, msg Message) (*Message, error)

	MarkRead(ctx context.Context, threadID int, userID int) error
	Archive(ctx context.Context, threadID int, userID int) error
	Unarchive(ctx context.Context, threadID int, userID int) error

	ThreadTotalCount(ctx context.Context, filter ThreadFilter) (int, error)
	MessageTotalCount(ctx context.Context, filter MessageFilter) (int, error)
	// UnreadCount - the user's threads with news, not archived.
	UnreadCount(ctx context.Context, userID int) (int, error)
}

// DefaultService - Service on the threads', messages' and members'
// repositories; a new thread's recipients are checked against the
// core's users.
type DefaultService struct {
	threadRepo  ThreadRepository
	messageRepo MessageRepository
	memberRepo  MemberRepository
	userRepo    modulekit.UserRepository
	logger      *slog.Logger
}

// NewService - the private messages' service on the threads', messages'
// and members' repositories; userRepo checks new threads' recipients.
func NewService(
	threadRepo ThreadRepository,
	messageRepo MessageRepository,
	memberRepo MemberRepository,
	userRepo modulekit.UserRepository,
	baseLogger *slog.Logger,
) Service {
	return &DefaultService{
		threadRepo:  threadRepo,
		messageRepo: messageRepo,
		memberRepo:  memberRepo,
		userRepo:    userRepo,
		logger:      baseLogger.With("component", "private_messages.service"),
	}
}

// --- Threads ---

// ThreadByID - the thread with its members.
func (s *DefaultService) ThreadByID(ctx context.Context, id int) (*Thread, error) {
	thread, err := s.threadRepo.ByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("getting thread by id (id %v): %w", id, err)
	}

	return thread, nil
}

// ThreadList - the threads matching filter, most recently updated first.
func (s *DefaultService) ThreadList(ctx context.Context, filter ThreadFilter) (*Threads, error) {
	threads, err := s.threadRepo.List(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("listing threads: %w", err)
	}

	return threads, nil
}

// ErrNoSuchRecipient - a new thread's recipient isn't an active user
// (ThreadCreate): no such account, or a blocked or pending one.
var ErrNoSuchRecipient = errors.New("the recipient is not an active user")

// ThreadCreate creates a thread with its members and sends the first message.
func (s *DefaultService) ThreadCreate(ctx context.Context, authorID int, subject string, memberIDs []int, firstMessage string) (*Thread, error) {
	// All recipients must exist.
	for _, uid := range memberIDs {
		u, err := s.userRepo.ByID(ctx, uid)
		if err != nil {
			if modulekit.IsNotFound(err) {
				return nil, fmt.Errorf("user %d: %w", uid, ErrNoSuchRecipient)
			}
			return nil, fmt.Errorf("loading the recipient (user %d): %w", uid, err)
		}
		if u.Status != modulekit.UserStatusActive {
			return nil, fmt.Errorf("user %d (status %d): %w", uid, u.Status, ErrNoSuchRecipient)
		}
	}

	// The author is a member too.
	allMembers := appendIfMissing(memberIDs, authorID)

	// Create the thread with all members.
	thread, err := s.threadRepo.Create(ctx, Thread{Subject: subject}, allMembers)
	if err != nil {
		return nil, fmt.Errorf("creating thread (author_id %v, subject %v): %w", authorID, subject, err)
	}

	// Send the first message.
	_, err = s.messageRepo.Create(ctx, Message{
		ThreadID: thread.ID,
		AuthorID: authorID,
		Body:     firstMessage,
	})
	if err != nil {
		return nil, fmt.Errorf("creating first message for thread (thread_id %v): %w", thread.ID, err)
	}

	return thread, nil
}

// --- Messages ---

// MessageList - a page of the messages matching filter, oldest first,
// with the total count.
func (s *DefaultService) MessageList(ctx context.Context, filter MessageFilter, pagination Pagination) (*MessagePage, error) {
	msgs, err := s.messageRepo.List(ctx, filter, pagination)
	if err != nil {
		return nil, fmt.Errorf("listing messages: %w", err)
	}

	return msgs, nil
}

// ErrNotThreadMember - the message's author isn't one of the thread's
// members (MessageCreate).
var ErrNotThreadMember = errors.New("not a member of the thread")

// MessageCreate adds the message to its thread (ErrNotThreadMember - its
// author isn't a member).
func (s *DefaultService) MessageCreate(ctx context.Context, msg Message) (*Message, error) {
	// The thread must exist and the author must be a member.
	thread, err := s.threadRepo.ByID(ctx, msg.ThreadID)
	if err != nil {
		return nil, fmt.Errorf("loading the thread (id %d): %w", msg.ThreadID, err)
	}

	if !thread.HasMember(msg.AuthorID) {
		return nil, fmt.Errorf("user %d, thread %d: %w", msg.AuthorID, msg.ThreadID, ErrNotThreadMember)
	}

	res, err := s.messageRepo.Create(ctx, msg)
	if err != nil {
		return nil, fmt.Errorf("creating message (thread_id %v, author_id %v): %w", msg.ThreadID, msg.AuthorID, err)
	}

	return res, nil
}

// --- Members ---

// MarkRead marks the thread read for the user.
func (s *DefaultService) MarkRead(ctx context.Context, threadID int, userID int) error {
	err := s.memberRepo.MarkRead(ctx, threadID, userID)
	if err != nil {
		return fmt.Errorf("marking thread read (thread_id %v, user_id %v): %w", threadID, userID, err)
	}

	return nil
}

// Archive moves the thread to the user's archive.
func (s *DefaultService) Archive(ctx context.Context, threadID int, userID int) error {
	err := s.memberRepo.Archive(ctx, threadID, userID)
	if err != nil {
		return fmt.Errorf("archiving thread (thread_id %v, user_id %v): %w", threadID, userID, err)
	}

	return nil
}

// Unarchive brings the thread back from the user's archive.
func (s *DefaultService) Unarchive(ctx context.Context, threadID int, userID int) error {
	err := s.memberRepo.Unarchive(ctx, threadID, userID)
	if err != nil {
		return fmt.Errorf("unarchiving thread (thread_id %v, user_id %v): %w", threadID, userID, err)
	}

	return nil
}

// --- Helpers ---

func appendIfMissing(slice []int, val int) []int {
	for _, v := range slice {
		if v == val {
			return slice
		}
	}
	return append(slice, val)
}

// ThreadTotalCount - how many threads match filter.
func (s *DefaultService) ThreadTotalCount(ctx context.Context, filter ThreadFilter) (int, error) {
	return s.threadRepo.TotalCount(ctx, filter)
}

// MessageTotalCount - how many messages match filter.
func (s *DefaultService) MessageTotalCount(ctx context.Context, filter MessageFilter) (int, error) {
	return s.messageRepo.TotalCount(ctx, filter)
}

// UnreadCount - see Service.UnreadCount.
func (s *DefaultService) UnreadCount(ctx context.Context, userID int) (int, error) {
	isNew, deleted := true, false
	n, err := s.threadRepo.TotalCount(ctx, ThreadFilter{UserID: userID, IsNew: &isNew, Deleted: &deleted})
	if err != nil {
		return 0, fmt.Errorf("counting unread threads (user %d): %w", userID, err)
	}
	return n, nil
}
