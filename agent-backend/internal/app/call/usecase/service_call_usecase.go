// Package callusecase runs one call to another service on behalf of the user of an MCP request.
package callusecase

import (
	"log/slog"

	"github.com/nice-platform/example/agent-backend/internal/domain"
)

// Service runs calls. Bases is where each target is called: inventaris directly, pengiriman (an api_key service)
// through the proxy of NICE, which checks the exchanged token and adds the service's own key. ClientVars names the
// settings of the agent's own client, for the message of a refused client.
type Service struct {
	Sender     Sender
	Bases      map[domain.Target]string
	ClientVars string
	Log        *slog.Logger
}
