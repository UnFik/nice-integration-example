package mcpadapter

import (
	"context"
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	calldto "github.com/nice-platform/example/agent-backend/internal/app/call/dto"
	"github.com/nice-platform/example/agent-backend/internal/domain"
)

// ListItems is the tool inventaris_list_items.
func (h *Handlers) ListItems(ctx context.Context, req *mcp.CallToolRequest, in listInput) (*mcp.CallToolResult, any, error) {
	return h.callTool(ctx, req, domain.Inventaris, calldto.Where{Org: in.OrganizationID, Project: in.ProjectID}, http.MethodGet, "/items", nil)
}
