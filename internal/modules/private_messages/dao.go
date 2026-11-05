package privatemessages

import (
	"github.com/kosatka-cms/kosatka/internal/modules/private_messages/ent/generated"
)

// toThread, toMessage, toMember - the ent rows as the module's types
// (their loaded edges too; the users are the core's, the repositories
// attach them).
func toThread(t *generated.PmThread) Thread {
	thread := Thread{
		ID:        t.ID,
		Subject:   t.Subject,
		CreatedAt: t.CreatedAt,
		UpdatedAt: t.UpdatedAt,
	}

	// Messages, if loaded.
	if t.Edges.Messages != nil {
		for _, m := range t.Edges.Messages {
			thread.Messages = append(thread.Messages, toMessage(m))
		}
		// Last message.
		if len(thread.Messages) > 0 {
			last := thread.Messages[len(thread.Messages)-1]
			thread.LastMessage = &last
		}
	}

	// Members, if loaded.
	if t.Edges.Members != nil {
		for _, m := range t.Edges.Members {
			thread.Members = append(thread.Members, toMember(m))
		}
	}

	return thread
}

func toMessage(m *generated.PmMessage) Message {
	msg := Message{
		ID:        m.ID,
		ThreadID:  m.ThreadID,
		AuthorID:  m.AuthorID,
		Body:      m.Body,
		CreatedAt: m.CreatedAt,
	}

	return msg
}

func toMember(m *generated.PmThreadMember) Member {
	member := Member{
		ThreadID: m.ThreadID,
		UserID:   m.UserID,
		IsNew:    m.IsNew,
		Deleted:  m.Deleted,
		JoinedAt: m.JoinedAt,
	}

	return member
}
