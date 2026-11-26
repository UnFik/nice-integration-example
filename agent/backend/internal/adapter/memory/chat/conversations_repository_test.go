package chatmemory_test

import (
	"testing"

	"github.com/nice-platform/example/agent/backend/internal/adapter/chattest"
	chatmemory "github.com/nice-platform/example/agent/backend/internal/adapter/memory/chat"
	chatusecase "github.com/nice-platform/example/agent/backend/internal/app/chat/usecase"
)

var _ chatusecase.Conversations = (*chatmemory.Repository)(nil)

func TestRepositoryKeepsTheContract(t *testing.T) {
	chattest.Run(t, func(*testing.T) chatusecase.Conversations { return chatmemory.New() })
}
