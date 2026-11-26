package chatpostgres_test

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/nice-platform/example/agent/backend/internal/adapter/chattest"
	chatpostgres "github.com/nice-platform/example/agent/backend/internal/adapter/postgres/chat"
	chatdto "github.com/nice-platform/example/agent/backend/internal/app/chat/dto"
	chatusecase "github.com/nice-platform/example/agent/backend/internal/app/chat/usecase"
	"github.com/nice-platform/example/agent/backend/internal/domain"
	"github.com/nice-platform/example/agent/backend/internal/platform/postgres"
	"github.com/nice-platform/example/agent/backend/migrations"
)

var _ chatusecase.Conversations = (*chatpostgres.Repository)(nil)

// schemaURL makes a schema of the test's own in TEST_DATABASE_URL, dropped when the test ends, and answers a URL
// whose search_path points at it, so the tests never see other data. It skips without TEST_DATABASE_URL.
func schemaURL(t *testing.T) string {
	t.Helper()
	raw := os.Getenv("TEST_DATABASE_URL")
	if raw == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	ctx := context.Background()
	admin, err := postgres.Open(ctx, raw)
	if err != nil {
		t.Fatal(err)
	}
	b := make([]byte, 6)
	_, _ = rand.Read(b)
	schema := "agen_chat_test_" + hex.EncodeToString(b)
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil {
		admin.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = admin.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE")
		admin.Close()
	})
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	q.Set("search_path", schema)
	u.RawQuery = q.Encode()
	return u.String()
}

// open builds a repository on dbURL the way the agent does on start: open, migrate, new.
func open(t *testing.T, dbURL string) (*chatpostgres.Repository, *pgxpool.Pool) {
	t.Helper()
	ctx := context.Background()
	pool, err := postgres.Open(ctx, dbURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := postgres.Migrate(ctx, pool, migrations.FS); err != nil {
		t.Fatal(err)
	}
	return chatpostgres.New(pool), pool
}

func TestRepositoryKeepsTheContract(t *testing.T) {
	chattest.Run(t, func(t *testing.T) chatusecase.Conversations {
		repo, _ := open(t, schemaURL(t))
		return repo
	})
}

func TestHistorySurvivesANewRepository(t *testing.T) {
	dbURL := schemaURL(t)
	ctx := context.Background()
	first, firstPool := open(t, dbURL)
	c := chattest.NewConversation(t, first, "usr_alice", "kept", chattest.Base)
	sent := []string{"hello", "again"}
	for i, text := range sent {
		if _, err := first.Append(ctx, "usr_alice", c.ID, chattest.Text(domain.RoleUser, text, chattest.Base.Add(time.Duration(i+1)*time.Second))); err != nil {
			t.Fatal(err)
		}
	}
	firstPool.Close()

	// Migrating again on start must be a no-op.
	second, _ := open(t, dbURL)
	list, err := second.List(ctx, "usr_alice")
	if err != nil || len(list) != 1 || list[0].ID != c.ID {
		t.Fatalf("list after restart = %+v, %v", list, err)
	}
	_, msgs, err := second.Get(ctx, "usr_alice", c.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(msgs) != len(sent) || msgs[0].Parts[0].Text != "hello" || msgs[1].Seq != 2 {
		t.Errorf("messages after restart = %+v", msgs)
	}
}

func TestDeleteCascadesToMessageRows(t *testing.T) {
	repo, pool := open(t, schemaURL(t))
	ctx := context.Background()
	c := chattest.NewConversation(t, repo, "usr_alice", "gone", chattest.Base)
	if _, err := repo.Append(ctx, "usr_alice", c.ID, chattest.Text(domain.RoleUser, "hi", chattest.Base)); err != nil {
		t.Fatal(err)
	}
	if err := repo.Delete(ctx, "usr_alice", c.ID); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM messages").Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("%d message rows left after Delete", n)
	}
}

// A tool result holds what the called service returned, never a token: no row may hold a JWT (they start with eyJ).
func TestNoTokenInStoredRows(t *testing.T) {
	repo, pool := open(t, schemaURL(t))
	ctx := context.Background()
	c := chattest.NewConversation(t, repo, "usr_alice", "devices", chattest.Base)
	msgs := []chatdto.Message{
		chattest.Text(domain.RoleUser, "list my devices", chattest.Base.Add(time.Second)),
		chattest.Call(chattest.Base.Add(2*time.Second), "call_1"),
		chattest.Result("call_1", `{"status":200,"headers":{"content-type":"application/json"},
			"body":{"devices":[{"id":"dev_1","name":"Sensor gudang","serial_number":"SN-1","status":"active",
			"created_by":"usr_alice","create_time":"2026-03-01T09:00:00Z"}]}}`, chattest.Base.Add(3*time.Second)),
		chattest.Text(domain.RoleAssistant, "You have one device: Sensor gudang.", chattest.Base.Add(4*time.Second)),
	}
	for _, m := range msgs {
		if _, err := repo.Append(ctx, "usr_alice", c.ID, m); err != nil {
			t.Fatal(err)
		}
	}
	rows, err := pool.Query(ctx, `SELECT parts::text FROM messages
		UNION ALL SELECT concat_ws(' ', id, user_sub, title) FROM conversations`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	n := 0
	for rows.Next() {
		var text string
		if err := rows.Scan(&text); err != nil {
			t.Fatal(err)
		}
		n++
		if strings.Contains(text, "eyJ") {
			t.Errorf("a stored row holds a JWT: %s", text)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if n != len(msgs)+1 {
		t.Errorf("scanned %d rows, want %d", n, len(msgs)+1)
	}
}
