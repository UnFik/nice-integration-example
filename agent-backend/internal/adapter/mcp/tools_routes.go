package mcpadapter

import (
	"github.com/google/jsonschema-go/jsonschema"
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

// createDeviceInput follows DeviceInput of nadi, which refuses unknown fields; nadi checks the lengths itself.
type createDeviceInput struct {
	Name           string `json:"name" jsonschema:"Name of the device, 1 to 120 characters"`
	SerialNumber   string `json:"serial_number" jsonschema:"Serial number of the device, 1 to 64 characters"`
	Status         string `json:"status" jsonschema:"Status of the device: active, inactive or maintenance"`
	Location       string `json:"location,omitempty" jsonschema:"Optional. Where the device is, at most 120 characters"`
	OrganizationID string `json:"organization_id,omitempty" jsonschema:"Optional. ID of the organization (org_...) to act in; default the organization of the user's token. Another organization needs project_id."`
	ProjectID      string `json:"project_id,omitempty" jsonschema:"Optional. ID of the project (prj_...) to act in; default the project of the user's token."`
}

func ptr[T any](v T) *T { return &v }

// deviceStatuses are the statuses nadi accepts.
var deviceStatuses = []any{"active", "inactive", "maintenance"}

// createDeviceSchema is the inferred schema with the status enum added, so the SDK refuses another status before
// any exchange.
func createDeviceSchema() *jsonschema.Schema {
	s, err := jsonschema.For[createDeviceInput](nil)
	if err != nil {
		panic(err)
	}
	s.Properties["status"].Enum = deviceStatuses
	return s
}

// newServer registers the tools. Deleting a device (sensitive in the nadi catalog) has no tool on purpose: an agent
// that a prompt can steer should not hold it, and NICE does not cap the exchanged token below the user's own
// permissions, so this list is the only limit. Only nice_token services have tools: an api_key service accepts
// Service Accounts only, so it cannot be called on behalf of a user.
func newServer(h *Handlers) *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: "agen", Title: "Agen (NICE example)", Version: "1.0.0"}, nil)
	readOnly := &mcp.ToolAnnotations{ReadOnlyHint: true, DestructiveHint: ptr(false), OpenWorldHint: ptr(false)}
	creates := &mcp.ToolAnnotations{DestructiveHint: ptr(false), OpenWorldHint: ptr(false)}

	mcp.AddTool(s, &mcp.Tool{
		Name: "nadi_list_devices", Title: "List IoT devices",
		Description: "Lists the IoT devices in nadi for the user, in the organization and project of the user's token or the ones named.",
		Annotations: readOnly,
	}, h.ListDevices)
	mcp.AddTool(s, &mcp.Tool{
		Name: "nadi_create_device", Title: "Register an IoT device",
		Description: "Registers an IoT device in nadi on behalf of the user. The device records the user as creator and this agent as actor (acted_by).",
		Annotations: creates, InputSchema: createDeviceSchema(),
	}, h.CreateDevice)
	return s
}
