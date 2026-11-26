package mcpadapter

import (
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	callusecase "github.com/nice-platform/example/agent/backend/internal/app/call/usecase"
	chatdto "github.com/nice-platform/example/agent/backend/internal/app/chat/dto"
)

// Handlers are the tool handlers; each one is a file named after its tool.
type Handlers struct {
	Calls *callusecase.Service
}

type listServicesInput struct{}

type listOperationsInput struct {
	Service string `json:"service" jsonschema:"Slug of a service from list_services, e.g. nadi"`
}

type getOperationSchemaInput struct {
	Service    string   `json:"service" jsonschema:"Slug of a service from list_services, e.g. nadi"`
	Operations []string `json:"operations" jsonschema:"1 to 10 keys of operations of that service from list_operations"`
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

var readOnly = &mcp.ToolAnnotations{ReadOnlyHint: true, DestructiveHint: ptr(false), OpenWorldHint: ptr(false)}

// tools is the one table both the MCP server and the chat read, so the model of the chat sees the same names,
// descriptions, and input schemas an MCP client gets from tools/list.
var tools = []*mcp.Tool{
	{
		Name: "list_services", Title: "List the services the user may call",
		Description: "Lists the organizations of the user, their projects, and in each project the services the user may call on their behalf. Call it first, then list_operations for a service.",
		Annotations: readOnly,
		InputSchema: schemaFor[listServicesInput](),
	},
	{
		Name: "list_operations", Title: "List the operations of a service",
		Description: "Lists the operations of one service whose permission the user holds: key, method, path, summary, request fields, and whether it is sensitive (a sensitive operation asks the user to approve it).",
		Annotations: readOnly,
		InputSchema: schemaFor[listOperationsInput](),
	},
	{
		Name: "get_operation_schema", Title: "Get the schemas of operations",
		Description: "Gives, for 1 to 10 operations from list_operations, the JSON Schema of their input (path, query, header, and body, with nested properties, required names, formats, and allowed values) and of their answers by status. Call it before call_operation when the operation takes a body or a parameter whose meaning list_operations does not make clear. truncated means the schema was cut and is not complete; missing lists keys the user may not call.",
		Annotations: readOnly,
		InputSchema: operationSchemaInput(),
	},
	{
		Name: "call_operation", Title: "Call an operation of a service",
		Description: "Calls one operation from list_operations on behalf of the user, in the organization and project of the user's token or the ones named. The service records the user as caller and this agent as actor (acted_by). A sensitive operation is sent only after the user approves it.",
		Annotations: &mcp.ToolAnnotations{DestructiveHint: ptr(true), OpenWorldHint: ptr(false)},
		InputSchema: schemaFor[callOperationInput](),
	},
}

// schemaFor infers the schema the way mcp.AddTool would, so setting it explicitly changes nothing on the wire.
func schemaFor[T any]() *jsonschema.Schema {
	s, err := jsonschema.ForType(reflect.TypeFor[T](), &jsonschema.ForOptions{})
	if err != nil {
		panic(fmt.Sprintf("input schema of %T: %v", *new(T), err))
	}
	return s
}

// operationSchemaInput is the inferred schema with operations as a plain array of 1 to 10 keys: the inference allows
// null for a slice, which models read as an optional argument.
func operationSchemaInput() *jsonschema.Schema {
	s := schemaFor[getOperationSchemaInput]()
	ops := s.Properties["operations"]
	ops.Type, ops.Types = "array", nil
	ops.MinItems, ops.MaxItems = ptr(1), ptr(10)
	return s
}

func tool(name string) *mcp.Tool {
	for _, t := range tools {
		if t.Name == name {
			// A copy: the SDK keeps the pointer, and the table must not change under the chat.
			c := *t
			return &c
		}
	}
	panic("unknown tool " + name)
}

// ToolSpecs describes the MCP tools to the chat model: the same name, description, and input schema.
func ToolSpecs() []chatdto.ToolSpec {
	out := make([]chatdto.ToolSpec, 0, len(tools))
	for _, t := range tools {
		raw, err := json.Marshal(t.InputSchema)
		if err != nil {
			panic(fmt.Sprintf("input schema of %s: %v", t.Name, err))
		}
		out = append(out, chatdto.ToolSpec{Name: t.Name, Description: t.Description, InputSchema: raw})
	}
	return out
}

// newServer registers the four tools. The list is the same for every user: what a user may call is read from NICE
// when a tool runs. NICE does not cap an exchanged token below the user's own permissions, so the limits the agent
// adds are its own: operations the catalog marks sensitive need the approval of the user, and only nice_token
// services are called, because an api_key service accepts Service Accounts only.
func newServer(h *Handlers) *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: "agen", Title: "Agen (NICE example)", Version: "2.0.0"}, nil)
	mcp.AddTool(s, tool("list_services"), h.ListServices)
	mcp.AddTool(s, tool("list_operations"), h.ListOperations)
	mcp.AddTool(s, tool("get_operation_schema"), h.GetOperationSchema)
	mcp.AddTool(s, tool("call_operation"), h.CallOperation)
	return s
}
