package chatpostgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"

	chatdto "github.com/nice-platform/example/agent/backend/internal/app/chat/dto"
	"github.com/nice-platform/example/agent/backend/internal/domain"
)

// lockConversation takes the row lock of the conversation of subject, so appends and call updates of one
// conversation run one at a time and the next seq cannot be taken twice.
func lockConversation(ctx context.Context, tx pgx.Tx, subject string, key [16]byte) error {
	var one int
	err := tx.QueryRow(ctx, "SELECT 1 FROM conversations WHERE id = $1 AND user_sub = $2 FOR UPDATE", key, subject).Scan(&one)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrConversationNotFound
	}
	return err
}

func (r *Repository) Append(ctx context.Context, subject, conversationID string, m chatdto.Message) (chatdto.Message, error) {
	key, ok := parseID(conversationID)
	if !ok {
		return chatdto.Message{}, domain.ErrConversationNotFound
	}
	if m.ID == "" {
		m.ID = newID()
	}
	mid, ok := parseID(m.ID)
	if !ok {
		return chatdto.Message{}, errors.New("a message ID must be a UUID")
	}
	if m.Parts == nil {
		m.Parts = []chatdto.Part{}
	}
	m.ConversationID = conversationID
	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		if err := lockConversation(ctx, tx, subject, key); err != nil {
			return err
		}
		if err := tx.QueryRow(ctx,
			"SELECT COALESCE(MAX(seq), 0) + 1 FROM messages WHERE conversation_id = $1", key).Scan(&m.Seq); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx,
			"INSERT INTO messages (id, conversation_id, seq, role, parts, created_at) VALUES ($1, $2, $3, $4, $5, $6)",
			mid, key, m.Seq, m.Role, m.Parts, m.CreatedAt); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, "UPDATE conversations SET updated_at = $2 WHERE id = $1", key, m.CreatedAt)
		return err
	})
	if err != nil {
		return chatdto.Message{}, err
	}
	return m, nil
}

func (r *Repository) UpdateCall(ctx context.Context, subject, conversationID string, call chatdto.ToolCall) error {
	key, ok := parseID(conversationID)
	if !ok {
		return domain.ErrConversationNotFound
	}
	return pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		if err := lockConversation(ctx, tx, subject, key); err != nil {
			return err
		}
		var (
			mid   [16]byte
			parts []chatdto.Part
		)
		// The containment test uses the call ID only as a JSON value, never as SQL text.
		err := tx.QueryRow(ctx, `SELECT id, parts FROM messages
			WHERE conversation_id = $1 AND parts @> jsonb_build_array(jsonb_build_object('kind', $2::text, 'call', jsonb_build_object('id', $3::text)))
			ORDER BY seq LIMIT 1`, key, domain.PartToolCall, call.ID).Scan(&mid, &parts)
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.ErrCallNotPending
		}
		if err != nil {
			return err
		}
		for i, p := range parts {
			if p.Kind == domain.PartToolCall && p.Call != nil && p.Call.ID == call.ID {
				parts[i].Call = &call
				_, err := tx.Exec(ctx, "UPDATE messages SET parts = $2 WHERE id = $1", mid, parts)
				return err
			}
		}
		return domain.ErrCallNotPending
	})
}
