package domain

import (
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Roles of a chat message. A tool message holds the results of the tool calls of the assistant message before it.
const (
	RoleUser      = "user"
	RoleAssistant = "assistant"
	RoleTool      = "tool"
)

// Kinds of a message part.
const (
	PartText       = "text"
	PartToolCall   = "tool_call"
	PartToolResult = "tool_result"
)

// Statuses of a tool call. A call that waits for the user's approval stops its turn; the answer resumes it.
const (
	CallDone             = "done"
	CallAwaitingApproval = "awaiting_approval"
)

const (
	// MaxMessageRunes bounds what a user may send in one message: a turn costs model calls.
	MaxMessageRunes = 8000
	// MaxTitleRunes bounds a conversation title the user sets.
	MaxTitleRunes = 120
	// derivedTitleRunes is how much of the first message becomes the title of a new conversation.
	derivedTitleRunes = 60
)

var (
	ErrConversationNotFound = errors.New("conversation not found")
	// ErrCallNotPending is the answer to an approval of a call that does not wait for one (unknown, answered).
	ErrCallNotPending = errors.New("tool call not awaiting approval")
	// ErrTurnInProgress refuses a second turn of a conversation while one runs.
	ErrTurnInProgress = errors.New("a turn of this conversation is in progress")
	// ErrChatOff is the answer to a turn when no model is configured.
	ErrChatOff = errors.New("chat is not configured")
	// ErrModelUnavailable is a model error worth retrying later (rate limit, overload, network).
	ErrModelUnavailable = errors.New("model unavailable")
	// ErrModelRefused is a model error a retry does not fix (bad key, unknown model, refused request).
	ErrModelRefused = errors.New("model refused the request")
)

// InvalidInputError is a message or title the user must change.
type InvalidInputError struct{ Reason string }

func (e InvalidInputError) Error() string { return "invalid input: " + e.Reason }

// TooLongError is a message over MaxMessageRunes.
type TooLongError struct{ Max int }

func (e TooLongError) Error() string { return "message too long" }

// CheckMessage refuses an empty message or one over MaxMessageRunes.
func CheckMessage(text string) error {
	if strings.TrimSpace(text) == "" {
		return InvalidInputError{Reason: "the message is empty"}
	}
	if !utf8.ValidString(text) {
		return InvalidInputError{Reason: "the message is not valid UTF-8"}
	}
	if utf8.RuneCountInString(text) > MaxMessageRunes {
		return TooLongError{Max: MaxMessageRunes}
	}
	return nil
}

// CleanTitle trims a title the user sets and refuses an empty one, one over MaxTitleRunes, or one with control
// characters.
func CleanTitle(title string) (string, error) {
	t := strings.TrimSpace(title)
	switch {
	case t == "":
		return "", InvalidInputError{Reason: "the title is empty"}
	case !utf8.ValidString(t):
		return "", InvalidInputError{Reason: "the title is not valid UTF-8"}
	case utf8.RuneCountInString(t) > MaxTitleRunes:
		return "", InvalidInputError{Reason: "the title is longer than 120 characters"}
	case strings.IndexFunc(t, unicode.IsControl) >= 0:
		return "", InvalidInputError{Reason: "the title holds a control character"}
	}
	return t, nil
}

// TitleFrom derives the title of a new conversation from its first message: one line of at most 60 characters.
func TitleFrom(text string) string {
	t := strings.Join(strings.FieldsFunc(text, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) }), " ")
	if utf8.RuneCountInString(t) <= derivedTitleRunes {
		return t
	}
	r := []rune(t)
	return strings.TrimSpace(string(r[:derivedTitleRunes-1])) + "…"
}
