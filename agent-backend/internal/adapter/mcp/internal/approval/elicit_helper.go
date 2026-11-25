// Package approval asks the user of an MCP client to approve a sensitive operation, the sessionless way: the tool
// result carries an elicitation and a state, and the client calls the tool again with the answer and the state.
package approval

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"

	calldto "github.com/nice-platform/example/agent-backend/internal/app/call/dto"
)

// requestID names the one input the agent asks for.
const requestID = "approve"

// multiRoundTrip is the first protocol version whose clients answer input requests by calling the tool again.
const multiRoundTrip = "2026-07-28"

var schema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		requestID: map[string]any{"type": "boolean", "title": "Approve", "description": "Send this operation now"},
	},
	"required": []string{requestID},
}

// Answer is what the client sent back for an approval: the state, and whether the user accepted with approve set.
func Answer(req *mcp.CallToolRequest) calldto.Approval {
	if req == nil || req.Params == nil || req.Params.RequestState == "" {
		return calldto.Approval{}
	}
	a := calldto.Approval{State: req.Params.RequestState}
	if r, ok := req.Params.InputResponses[requestID].(*mcp.ElicitResult); ok && r.Action == "accept" {
		a.Accepted = r.Content[requestID] == true
	}
	return a
}

// Ask turns an approval request into a tool result with an input request. It reports false when the client cannot
// ask: a stateless server cannot send it an elicitation of its own, so the operation must not be sent.
func Ask(req *mcp.CallToolRequest, ask *calldto.ApprovalRequest) (*mcp.CallToolResult, bool) {
	if !canAsk(req) {
		return nil, false
	}
	return &mcp.CallToolResult{
		InputRequests: mcp.InputRequestMap{requestID: &mcp.ElicitParams{Mode: "form", Message: ask.Message, RequestedSchema: schema}},
		RequestState:  ask.State,
	}, true
}

func canAsk(req *mcp.CallToolRequest) bool {
	if req == nil || req.Session == nil {
		return false
	}
	ip := req.Session.InitializeParams()
	if ip == nil || ip.ProtocolVersion < multiRoundTrip || ip.Capabilities == nil || ip.Capabilities.Elicitation == nil {
		return false
	}
	// No form and no url declared means form, as the MCP SDK reads it.
	e := ip.Capabilities.Elicitation
	return e.Form != nil || e.URL == nil
}
