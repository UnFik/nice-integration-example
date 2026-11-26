package calldto

import "encoding/json"

// OperationSchemas are the full schemas of operations of one service, as NICE reads them from the document.
type OperationSchemas struct {
	Service string
	Schemas []OperationSchema
	// Missing are the keys the user may not call or that are not active.
	Missing []string
}

// OperationSchema is the input and response schema of one operation, the JSON NICE sent.
type OperationSchema struct {
	Key         string          `json:"key"`
	InputSchema json.RawMessage `json:"input_schema"`
	Responses   json.RawMessage `json:"responses"`
	// Truncated is true when NICE cut the schema; a cut part keeps its type alone.
	Truncated bool `json:"truncated"`
}

// SchemasView is the answer of get_operation_schema.
type SchemasView struct {
	Service string            `json:"service"`
	Schemas []OperationSchema `json:"schemas"`
	Missing []string          `json:"missing"`
}
