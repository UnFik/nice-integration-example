// Package mapper turns use case results into MCP results.
package mapper

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"

	calldto "github.com/nice-platform/example/agent-backend/internal/app/call/dto"
)

// FailureResult is a result marked as an error that names the code and says what it means.
func FailureResult(f *calldto.Failure) *mcp.CallToolResult {
	text := f.Code
	if f.Description != "" && f.Description != f.Code {
		text += " (" + f.Description + ")"
	}
	text += ": " + f.Message
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}, IsError: true}
}

// Unauthenticated is a tool call without the verified user of the request.
func Unauthenticated(err error) *mcp.CallToolResult {
	return FailureResult(&calldto.Failure{Code: "unauthenticated", Message: err.Error()})
}

// ApprovalUnavailable refuses a sensitive operation in an MCP client that cannot ask the user.
func ApprovalUnavailable() *mcp.CallToolResult {
	return FailureResult(&calldto.Failure{Code: "approval_unavailable",
		Message: "this operation is sensitive and needs the approval of the user, but this MCP client cannot ask for it " +
			"(it needs MCP protocol 2026-07-28 with form elicitation); nothing was sent"})
}
