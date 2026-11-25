## MODIFIED Requirements

### Requirement: MCP server dilindungi token NICE
The agent SHALL serve MCP over Streamable HTTP at `/mcp` and SHALL accept only requests with a NICE access token whose `aud` is `agen` and whose `perms` hold `agen.mcp.use`, verified with the `verify` package of `github.com/ABA-Developer/nice-auth-sdk`. It SHALL serve Protected Resource Metadata at `/.well-known/oauth-protected-resource` naming NICE as the authorization server and `nice:aud:agen` as the needed scope.

#### Scenario: Tanpa token
- **WHEN** an MCP client calls `/mcp` without a token
- **THEN** the agent answers 401 with `WWW-Authenticate` pointing to its resource metadata

#### Scenario: Token untuk service lain
- **WHEN** an MCP client calls `/mcp` with a token whose `aud` is `nadi`
- **THEN** the agent answers 401 `wrong_audience`

### Requirement: Konteks tool
Tools SHALL accept optional `organization_id` and `project_id`, defaulting to the organization and project of the user's token, and SHALL pass them to the exchange as `nice:org` and `nice:prj`. The agent SHALL NOT pick a context the user did not name or hold.

#### Scenario: Organisasi lain milik user
- **WHEN** the user calls `nadi_list_devices` with another organization they belong to and a project there
- **THEN** the agent exchanges for that organization and project and returns its devices

#### Scenario: Organisasi bukan milik user
- **WHEN** NICE answers the exchange with `access_denied`
- **THEN** the tool result is an error that names the reason and no service is called

### Requirement: Galat aman dan jelas
Failures of the exchange or of the target service SHALL be returned as MCP tool results marked as errors, with the NICE or service error code and a short explanation, and SHALL NOT contain any token, secret, or raw upstream body beyond the error code and description.

#### Scenario: Izin kurang di service
- **WHEN** the user lacks `nadi.devices.create` and calls `nadi_create_device`
- **THEN** the tool result is an error naming `permission_denied`

### Requirement: Tanpa tool sensitif
The agent SHALL NOT expose tools for operations marked sensitive in the target catalog, such as deleting a device in `nadi`. Because an exchanged token carries the user's permissions in the target project without a ceiling, the set of tools SHALL be the only limit the agent places on what it does for the user, and the README SHALL say so.

#### Scenario: Daftar tool
- **WHEN** an MCP client lists the tools
- **THEN** no tool deletes a device

### Requirement: Panduan setup dan uji
The example SHALL carry `README.md` and `README.en.md` with the NICE version it needs, the manual setup checklist (service `agen` and its import, public and confidential service clients bound to `agen`, entitlement, activation, and role binding), how to obtain a user token, how to call the tools with MCP Inspector and `curl`, how to run it in a container, and a test checklist that shows `acted_by` in `nadi`. The README SHALL explain that `api_key` services use a key per Service Account and refuse user tokens, so this agent offers tools for `nice_token` services only.

#### Scenario: Mitra mengikuti README
- **WHEN** a partner follows the setup and calls `nadi_create_device`
- **THEN** the created device in `nadi` shows the agent's client as `acted_by`

### Requirement: Exchange hemat dan paralel antar konteks
The agent SHALL reuse an exchanged token per `(user, organization, project, service)` until it nears expiry or the subject token ends, SHALL let calls for different contexts run concurrently, and SHALL retry a call once after a fresh exchange when the target answers 401 `token_stale`.

#### Scenario: Banyak panggilan
- **WHEN** the user calls `nadi_list_devices` ten times in one minute in the same context
- **THEN** the agent sends one exchange request

#### Scenario: Dua konteks sekaligus
- **WHEN** the MCP client calls `nadi_list_devices` for two different projects at the same time
- **THEN** both exchanges are in flight together

## ADDED Requirements

### Requirement: Tool memanggil nadi atas nama user
Each tool SHALL obtain a token for `nadi` with token exchange (the exchange mode of the `credentials` package of `github.com/ABA-Developer/nice-auth-sdk`), using the user's token from the MCP request as subject token and the agent's confidential service client as actor, and SHALL call `nadi` at `NADI_URL` with that token only. The user's own token SHALL NOT be sent to any service other than NICE's token endpoint. The agent SHALL NOT expose tools for `api_key` services.

#### Scenario: Daftar perangkat
- **WHEN** the user calls `nadi_list_devices` without arguments
- **THEN** the agent exchanges for `nice:aud:nadi` in the user's token organization and project, calls `GET /devices` on `nadi` with the exchanged token, and returns the devices

#### Scenario: Buat perangkat
- **WHEN** the user calls `nadi_create_device` with `name`, `serial_number`, and `status`
- **THEN** the agent sends `POST /devices` with those fields, plus `location` only when given, and returns the created device

#### Scenario: Tidak ada tool pengiriman
- **WHEN** an MCP client lists the tools
- **THEN** no tool targets `pengiriman`

### Requirement: Image container
The example SHALL carry a `Dockerfile` that builds a static binary with Go 1.27 and runs it on a distroless nonroot image, listening on `0.0.0.0:8083` and keeping the JWKS copy under `/data`.

#### Scenario: Build image
- **WHEN** a partner builds the image from `agent-backend/`
- **THEN** the image builds without the `nice` repository and exposes port 8083

## REMOVED Requirements

### Requirement: Tool memanggil inventaris atas nama user
**Reason**: service `inventaris` sudah berganti nama menjadi `nadi` (`sharedservice-oauth/backend`, perangkat di `/devices`).
**Migration**: lihat requirement "Tool memanggil nadi atas nama user"; ganti `INVENTARIS_URL` dengan `NADI_URL`.
