package calldto

// Approval is what the MCP client sent back for an approval the agent asked: the state the agent gave and whether
// the user accepted. An empty State means no approval was asked yet.
type Approval struct {
	State    string
	Accepted bool
}

// ApprovalRequest asks the user to approve one call of a sensitive operation. State binds the answer to the user and
// the call; it is opaque to the client.
type ApprovalRequest struct {
	Message string
	State   string
}
