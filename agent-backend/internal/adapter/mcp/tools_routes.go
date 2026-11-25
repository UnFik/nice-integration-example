package mcpadapter

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"

	callusecase "github.com/nice-platform/example/agent-backend/internal/app/call/usecase"
)

// Handlers are the tool handlers; each one is a file named after its tool.
type Handlers struct {
	Calls *callusecase.Service
}

type listOperationsInput struct {
	Service string `json:"service" jsonschema:"Slug of a service from list_services, e.g. nadi"`
}

type callOperationInput struct {
	Service        string            `json:"service" jsonschema:"Slug of a service from list_services, e.g. nadi"`
	Operation      string            `json:"operation" jsonschema:"Key of an operation of that service from list_operations"`
	OrganizationID string            `json:"organization_id,omitempty" jsonschema:"Optional. ID of the organization (org_...) to act in; default the organization of the user's token. Another organization needs project_id."`
	ProjectID      string            `json:"project_id,omitempty" jsonschema:"Optional. ID of a project (prj_...) that list_services lists for the service; default the project of the user's token."`
	PathParams     map[string]string `json:"path_params,omitempty" jsonschema:"Value of every request field of location path, by name"`
	Query          map[string]string `json:"query,omitempty" jsonschema:"Optional. Values of request fields of location query, by name"`
	Body           any               `json:"body,omitempty" jsonschema:"Optional. JSON body with the request fields of location body"`
}

func ptr[T any](v T) *T { return &v }

// newServer registers the three tools. The list is the same for every user: what a user may call is read from NICE
// when a tool runs. NICE does not cap an exchanged token below the user's own permissions, so the limits the agent
// adds are its own: operations the catalog marks sensitive need the approval of the user, and only nice_token
// services are called, because an api_key service accepts Service Accounts only.
func newServer(h *Handlers) *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: "agen", Title: "Agen (NICE example)", Version: "2.0.0"}, nil)
	readOnly := &mcp.ToolAnnotations{ReadOnlyHint: true, DestructiveHint: ptr(false), OpenWorldHint: ptr(false)}

	mcp.AddTool(s, &mcp.Tool{
		Name: "list_services", Title: "List the services the user may call",
		Description: "Lists the organizations of the user, their projects, and in each project the services the user may call on their behalf. Call it first, then list_operations for a service.",
		Annotations: readOnly,
	}, h.ListServices)
	mcp.AddTool(s, &mcp.Tool{
		Name: "list_operations", Title: "List the operations of a service",
		Description: "Lists the operations of one service whose permission the user holds: key, method, path, summary, request fields, and whether it is sensitive (a sensitive operation asks the user to approve it).",
		Annotations: readOnly,
	}, h.ListOperations)
	mcp.AddTool(s, &mcp.Tool{
		Name: "call_operation", Title: "Call an operation of a service",
		Description: "Calls one operation from list_operations on behalf of the user, in the organization and project of the user's token or the ones named. The service records the user as caller and this agent as actor (acted_by). A sensitive operation is sent only after the user approves it.",
		Annotations: &mcp.ToolAnnotations{DestructiveHint: ptr(true), OpenWorldHint: ptr(false)},
	}, h.CallOperation)
	return s
}
