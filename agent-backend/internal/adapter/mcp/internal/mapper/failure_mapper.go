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
