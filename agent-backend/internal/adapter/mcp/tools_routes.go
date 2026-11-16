package mcpadapter

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"

	callusecase "github.com/nice-platform/example/agent-backend/internal/app/call/usecase"
)

// Handlers are the tool handlers; each one is a file named after its tool.
type Handlers struct {
	Calls *callusecase.Service
}

type listInput struct {
	OrganizationID string `json:"organization_id,omitempty" jsonschema:"Optional. ID of the organization (org_...) to act in; default the organization of the user's token. Another organization needs project_id."`
	ProjectID      string `json:"project_id,omitempty" jsonschema:"Optional. ID of the project (prj_...) to act in; default the project of the user's token."`
}

type createItemInput struct {
	Name           string `json:"name" jsonschema:"Name of the item, 1 to 120 characters"`
	Quantity       int64  `json:"quantity" jsonschema:"Quantity in stock, at least 0"`
	OrganizationID string `json:"organization_id,omitempty" jsonschema:"Optional. ID of the organization (org_...) to act in; default the organization of the user's token. Another organization needs project_id."`
	ProjectID      string `json:"project_id,omitempty" jsonschema:"Optional. ID of the project (prj_...) to act in; default the project of the user's token."`
}

type createShipmentInput struct {
	Destination    string `json:"destination" jsonschema:"Destination of the shipment, 1 to 200 characters"`
	OrganizationID string `json:"organization_id,omitempty" jsonschema:"Optional. ID of the organization (org_...) to act in; default the organization of the user's token. Another organization needs project_id."`
	ProjectID      string `json:"project_id,omitempty" jsonschema:"Optional. ID of the project (prj_...) to act in; default the project of the user's token."`
}

func ptr[T any](v T) *T { return &v }

// newServer registers the tools. Deleting an item (sensitive in the inventaris catalog) and cancelling a shipment
// have no tool on purpose: an agent that a prompt can steer should not hold them.
func newServer(h *Handlers) *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: "agen", Title: "Agen (NICE example)", Version: "1.0.0"}, nil)
	readOnly := &mcp.ToolAnnotations{ReadOnlyHint: true, DestructiveHint: ptr(false), OpenWorldHint: ptr(false)}
	creates := &mcp.ToolAnnotations{DestructiveHint: ptr(false), OpenWorldHint: ptr(false)}

	mcp.AddTool(s, &mcp.Tool{
		Name: "inventaris_list_items", Title: "List stock items",
		Description: "Lists the stock items in inventaris for the user, in the organization and project of the user's token or the ones named.",
		Annotations: readOnly,
	}, h.ListItems)
	mcp.AddTool(s, &mcp.Tool{
		Name: "inventaris_create_item", Title: "Create a stock item",
		Description: "Creates a stock item in inventaris on behalf of the user. The item records the user as creator and this agent as actor (acted_by).",
		Annotations: creates,
	}, h.CreateItem)
	mcp.AddTool(s, &mcp.Tool{
		Name: "pengiriman_list_shipments", Title: "List shipments",
		Description: "Lists the shipments in pengiriman for the user, in the organization and project of the user's token or the ones named.",
		Annotations: readOnly,
	}, h.ListShipments)
	mcp.AddTool(s, &mcp.Tool{
		Name: "pengiriman_create_shipment", Title: "Create a shipment",
		Description: "Creates a shipment in pengiriman on behalf of the user.",
		Annotations: creates,
	}, h.CreateShipment)
	return s
}
