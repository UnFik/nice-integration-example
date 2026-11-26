package calldto

// ServiceOperations are the operations of one service whose permission the user holds.
type ServiceOperations struct {
	Service    string
	Operations []Operation
}

// Operation is one operation of a service as its catalog lists it.
type Operation struct {
	Key           string
	Method        string
	Path          string
	Summary       string
	Description   string
	Permission    string
	Tag           string
	Sensitive     bool
	Headers       []Header
	RequestFields []RequestField
}

// Header is a header an operation needs; the SDK sends it.
type Header struct {
	Name  string
	Value string
}

// RequestField names one parameter or top-level body property of an operation, never a value. Description, Format
// and Enum come from the document of the service and help the model fill the field.
type RequestField struct {
	Location      string `json:"location"`
	Name          string `json:"name"`
	Required      bool   `json:"required,omitempty"`
	Type          string `json:"type,omitempty"`
	Description   string `json:"description,omitempty"`
	Format        string `json:"format,omitempty"`
	Enum          []any  `json:"enum,omitempty"`
	EnumTruncated bool   `json:"enum_truncated,omitempty"`
}

// OperationsView is the answer of list_operations.
type OperationsView struct {
	Service    string          `json:"service"`
	Operations []OperationView `json:"operations"`
}

type OperationView struct {
	Key           string         `json:"key"`
	Method        string         `json:"method"`
	Path          string         `json:"path"`
	Summary       string         `json:"summary,omitempty"`
	Description   string         `json:"description,omitempty"`
	Permission    string         `json:"permission,omitempty"`
	Tag           string         `json:"tag,omitempty"`
	Sensitive     bool           `json:"sensitive"`
	RequestFields []RequestField `json:"request_fields"`
}

// OperationInput is a call_operation as the model asked for it. Nothing in it is trusted: the service and the
// operation are looked up in the answers of NICE.
type OperationInput struct {
	Service    string
	Operation  string
	Where      Where
	PathParams map[string]string
	Query      map[string]string
	Body       any
}

// OperationCall is one call of an operation, resolved from the map and the operations of NICE.
type OperationCall struct {
	Service    MapService
	Operation  Operation
	Org        string
	Project    string
	PathParams map[string]string
	Query      map[string][]string
	Body       []byte
}

// OperationResult is the answer of a service, with the body cut at a size limit.
type OperationResult struct {
	Status int
	Body   []byte
}

// CallView is the answer of call_operation: the status of the service and its JSON body.
type CallView struct {
	Status int `json:"status"`
	Body   any `json:"body,omitempty"`
}
