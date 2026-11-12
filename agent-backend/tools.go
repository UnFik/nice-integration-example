package main

import (
	"context"
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

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

// addTools registers the tools. Deleting an item (sensitive in the inventaris catalog) and cancelling a shipment
// have no tool on purpose: an agent that a prompt can steer should not hold them.
func (a *agent) addTools() {
	readOnly := &mcp.ToolAnnotations{ReadOnlyHint: true, DestructiveHint: ptr(false), OpenWorldHint: ptr(false)}
	creates := &mcp.ToolAnnotations{DestructiveHint: ptr(false), OpenWorldHint: ptr(false)}

	mcp.AddTool(a.server, &mcp.Tool{
		Name: "inventaris_list_items", Title: "List stock items",
		Description: "Lists the stock items in inventaris for the user, in the organization and project of the user's token or the ones named.",
		Annotations: readOnly,
	}, func(ctx context.Context, req *mcp.CallToolRequest, in listInput) (*mcp.CallToolResult, any, error) {
		return a.tool(ctx, req, inventaris, contextArgs{in.OrganizationID, in.ProjectID}, http.MethodGet, "/items", nil)
	})
	mcp.AddTool(a.server, &mcp.Tool{
		Name: "inventaris_create_item", Title: "Create a stock item",
		Description: "Creates a stock item in inventaris on behalf of the user. The item records the user as creator and this agent as actor (acted_by).",
		Annotations: creates,
	}, func(ctx context.Context, req *mcp.CallToolRequest, in createItemInput) (*mcp.CallToolResult, any, error) {
		body := map[string]any{"name": in.Name, "quantity": in.Quantity}
		return a.tool(ctx, req, inventaris, contextArgs{in.OrganizationID, in.ProjectID}, http.MethodPost, "/items", body)
	})
	mcp.AddTool(a.server, &mcp.Tool{
		Name: "pengiriman_list_shipments", Title: "List shipments",
		Description: "Lists the shipments in pengiriman for the user, in the organization and project of the user's token or the ones named.",
		Annotations: readOnly,
	}, func(ctx context.Context, req *mcp.CallToolRequest, in listInput) (*mcp.CallToolResult, any, error) {
		return a.tool(ctx, req, pengiriman, contextArgs{in.OrganizationID, in.ProjectID}, http.MethodGet, "/shipments", nil)
	})
	mcp.AddTool(a.server, &mcp.Tool{
		Name: "pengiriman_create_shipment", Title: "Create a shipment",
		Description: "Creates a shipment in pengiriman on behalf of the user.",
		Annotations: creates,
	}, func(ctx context.Context, req *mcp.CallToolRequest, in createShipmentInput) (*mcp.CallToolResult, any, error) {
		body := map[string]any{"destination": in.Destination}
		return a.tool(ctx, req, pengiriman, contextArgs{in.OrganizationID, in.ProjectID}, http.MethodPost, "/shipments", body)
	})
}

// tool runs one call for the user of the MCP request. A nil output with a result is an error the model reads.
func (a *agent) tool(ctx context.Context, req *mcp.CallToolRequest, t target, where contextArgs, method, path string, body any) (*mcp.CallToolResult, any, error) {
	c, err := callerOf(req)
	if err != nil {
		return (&toolError{Code: "unauthenticated", Message: err.Error()}).result(), nil, nil
	}
	res, out := a.call(ctx, c, t, where, method, path, body)
	if res != nil {
		return res, nil, nil
	}
	return nil, out, nil
}
