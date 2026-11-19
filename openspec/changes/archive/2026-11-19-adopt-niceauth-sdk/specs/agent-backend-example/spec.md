# Spec Delta

## MODIFIED Requirements

### Requirement: MCP server dilindungi token NICE
The agent SHALL serve MCP over Streamable HTTP at `/mcp` and SHALL accept only requests with a NICE access token whose `aud` is `agen` and whose `perms` hold `agen.mcp.use`, verified with `pkg/niceauth/verify`. It SHALL serve Protected Resource Metadata at `/.well-known/oauth-protected-resource` naming NICE as the authorization server and `nice:aud:agen` as the needed scope.

#### Scenario: Tanpa token
- **WHEN** an MCP client calls `/mcp` without a token
- **THEN** the agent answers 401 with `WWW-Authenticate` pointing to its resource metadata

#### Scenario: Token untuk service lain
- **WHEN** an MCP client calls `/mcp` with a token whose `aud` is `inventaris`
- **THEN** the agent answers 401 `wrong_audience`

### Requirement: Tool memanggil inventaris atas nama user
Each tool SHALL obtain a token for `inventaris` with token exchange (`pkg/niceauth/credentials` exchange mode), using the user's token from the MCP request as subject token and the agent's confidential service client as actor, and SHALL call `inventaris` with that token only. The user's own token SHALL NOT be sent to any service other than NICE's token endpoint. The agent SHALL NOT expose tools for `api_key` services.

#### Scenario: Daftar barang
- **WHEN** the user calls `inventaris_list_items` without arguments
- **THEN** the agent exchanges for `nice:aud:inventaris` in the user's token organization and project, calls `inventaris` with the exchanged token, and returns the items

#### Scenario: Tidak ada tool pengiriman
- **WHEN** an MCP client lists the tools
- **THEN** no tool targets `pengiriman`
