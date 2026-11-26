// Package chatpostgres keeps conversations in PostgreSQL. Every query filters by user_sub, so the ID of another
// user's conversation is answered as not found.
package chatpostgres

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	chatdto "github.com/nice-platform/example/agent/backend/internal/app/chat/dto"
	"github.com/nice-platform/example/agent/backend/internal/domain"
)

type Repository struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Repository { return &Repository{pool: pool} }

// parseID reads the canonical 8-4-4-4-12 form. An ID in any other form cannot be stored, so it is not found rather
// than a database error.
func parseID(s string) ([16]byte, bool) {
	var id [16]byte
	if len(s) != 36 || s[8] != '-' || s[13] != '-' || s[18] != '-' || s[23] != '-' {
		return id, false
	}
	if _, err := hex.Decode(id[:], []byte(strings.ReplaceAll(s, "-", ""))); err != nil {
		return id, false
	}
	return id, true
}

func formatID(id [16]byte) string {
	return fmt.Sprintf("%x-%x-%x-%x-%x", id[0:4], id[4:6], id[6:8], id[8:10], id[10:])
}

func newID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return formatID(b)
}

func scanConversation(row pgx.Row) (chatdto.Conversation, error) {
	var (
		c  chatdto.Conversation
		id [16]byte
	)
	if err := row.Scan(&id, &c.Subject, &c.Title, &c.CreatedAt, &c.UpdatedAt); err != nil {
		return chatdto.Conversation{}, err
	}
	c.ID = formatID(id)
	c.CreatedAt, c.UpdatedAt = c.CreatedAt.UTC(), c.UpdatedAt.UTC()
	return c, nil
}

const conversationColumns = "id, user_sub, title, created_at, updated_at"

func (r *Repository) List(ctx context.Context, subject string) ([]chatdto.Conversation, error) {
	rows, err := r.pool.Query(ctx,
		"SELECT "+conversationColumns+" FROM conversations WHERE user_sub = $1 ORDER BY updated_at DESC, id", subject)
	if err != nil {
		return nil, err
	}
	out, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (chatdto.Conversation, error) {
		return scanConversation(row)
	})
	if out == nil && err == nil {
		out = []chatdto.Conversation{}
	}
	return out, err
}

func (r *Repository) Get(ctx context.Context, subject, id string) (chatdto.Conversation, []chatdto.Message, error) {
	key, ok := parseID(id)
	if !ok {
		return chatdto.Conversation{}, nil, domain.ErrConversationNotFound
	}
	c, err := scanConversation(r.pool.QueryRow(ctx,
		"SELECT "+conversationColumns+" FROM conversations WHERE id = $1 AND user_sub = $2", key, subject))
	if errors.Is(err, pgx.ErrNoRows) {
		return chatdto.Conversation{}, nil, domain.ErrConversationNotFound
	}
	if err != nil {
		return chatdto.Conversation{}, nil, err
	}
	rows, err := r.pool.Query(ctx, `SELECT m.id, m.seq, m.role, m.parts, m.created_at
		FROM messages m JOIN conversations c ON c.id = m.conversation_id
		WHERE m.conversation_id = $1 AND c.user_sub = $2 ORDER BY m.seq`, key, subject)
	if err != nil {
		return chatdto.Conversation{}, nil, err
	}
	msgs, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (chatdto.Message, error) {
		var (
			m   chatdto.Message
			mid [16]byte
		)
		if err := row.Scan(&mid, &m.Seq, &m.Role, &m.Parts, &m.CreatedAt); err != nil {
			return chatdto.Message{}, err
		}
		m.ID, m.ConversationID, m.CreatedAt = formatID(mid), c.ID, m.CreatedAt.UTC()
		return m, nil
	})
	if err != nil {
		return chatdto.Conversation{}, nil, err
	}
	return c, msgs, nil
}

func (r *Repository) Create(ctx context.Context, c chatdto.Conversation) error {
	key, ok := parseID(c.ID)
	if !ok || c.Subject == "" {
		return errors.New("a conversation needs a UUID and a subject")
	}
	_, err := r.pool.Exec(ctx,
		"INSERT INTO conversations (id, user_sub, title, created_at, updated_at) VALUES ($1, $2, $3, $4, $5)",
		key, c.Subject, c.Title, c.CreatedAt, c.UpdatedAt)
	return err
}

func (r *Repository) Rename(ctx context.Context, subject, id, title string, at time.Time) error {
	key, ok := parseID(id)
	if !ok {
		return domain.ErrConversationNotFound
	}
	tag, err := r.pool.Exec(ctx,
		"UPDATE conversations SET title = $3, updated_at = $4 WHERE id = $1 AND user_sub = $2", key, subject, title, at)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrConversationNotFound
	}
	return nil
}

// Delete relies on ON DELETE CASCADE to remove the messages with the conversation.
func (r *Repository) Delete(ctx context.Context, subject, id string) error {
	key, ok := parseID(id)
	if !ok {
		return domain.ErrConversationNotFound
	}
	tag, err := r.pool.Exec(ctx, "DELETE FROM conversations WHERE id = $1 AND user_sub = $2", key, subject)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrConversationNotFound
	}
	return nil
}
