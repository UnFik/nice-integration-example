# agent-backend-example Specification

## Purpose
Contoh backend agen (MCP server untuk service `agen`) yang menjalankan tool ke service lain atas nama user lewat token exchange NICE, tanpa meneruskan token user, tanpa identitas super, dan tanpa menyimpan refresh token, sebagai referensi untuk agen seperti NAIA.

## Requirements

### Requirement: MCP server dilindungi token NICE
The agent SHALL serve MCP over Streamable HTTP at `/mcp` and SHALL accept only requests with a NICE access token whose `aud` is `agen` and whose `perms` hold `agen.mcp.use`, verified with the `verify` package of `github.com/ABA-Developer/nice-auth-sdk`. It SHALL serve Protected Resource Metadata at `/.well-known/oauth-protected-resource` naming NICE as the authorization server and `nice:aud:agen` as the needed scope.

#### Scenario: Tanpa token
- **WHEN** an MCP client calls `/mcp` without a token
- **THEN** the agent answers 401 with `WWW-Authenticate` pointing to its resource metadata

#### Scenario: Token untuk service lain
- **WHEN** an MCP client calls `/mcp` with a token whose `aud` is `nadi`
- **THEN** the agent answers 401 `wrong_audience`

### Requirement: Konteks tool
`call_operation` SHALL accept optional `organization_id` and `project_id`, defaulting to the organization and project of the user's token, and SHALL pass them to the exchange as `nice:org` and `nice:prj`. The agent SHALL NOT pick a context the user did not name or hold. A named organization other than the token's SHALL need a `project_id`, and a named context SHALL be one that the service map lists for the service before anything is exchanged.

#### Scenario: Organisasi lain milik user
- **WHEN** the user calls `call_operation` for `nadi` with another organization they belong to and a project there that the map lists for `nadi`
- **THEN** the agent exchanges for that organization and project and returns the answer of `nadi`

#### Scenario: Konteks di luar peta
- **WHEN** the user names a project that the map does not list for the service
- **THEN** the tool result is an `invalid_argument` error and no token for the target service is exchanged

#### Scenario: Organisasi bukan milik user
- **WHEN** NICE answers the exchange with `access_denied`
- **THEN** the tool result is an error that names the reason and no service is called

### Requirement: Galat aman dan jelas
Failures of discovery, of the exchange, or of the target service SHALL be returned as MCP tool results marked as errors, with the NICE or service error code and a short explanation, and SHALL NOT contain any token, secret, approval state, or raw upstream body beyond the error code and description.

#### Scenario: Izin kurang di service
- **WHEN** the user lacks `nadi.devices.create` in the chosen project and calls `call_operation` for `POST /devices`
- **THEN** the tool result is an error naming `permission_denied`

#### Scenario: NICE tanpa discovery
- **WHEN** NICE answers `404` for its service map because it predates the discovery operations
- **THEN** the tool result is an error saying the NICE version is too old for discovery

### Requirement: Panduan setup dan uji
The example SHALL carry `README.md` and `README.en.md` with the NICE version it needs (one that has the caller service discovery operations), the manual setup checklist (service `agen` and its import, public and confidential service clients bound to `agen`, entitlement, activation, role binding, and a target catalog endpoint that is `https` or loopback), how to obtain a user token, how to call the three tools with MCP Inspector and `curl`, how a sensitive operation is approved, how to run it in a container, and a test checklist that shows `acted_by` in `nadi`. The README SHALL explain that `api_key` services use a key per Service Account and refuse user tokens, so this agent calls `nice_token` services only. The example SHALL live in `agent/`, with the MCP server and the dashboard backend in `agent/backend/` and the dashboard UI in `agent/web/`.

#### Scenario: Mitra mengikuti README
- **WHEN** a partner follows the setup, calls `list_services`, `list_operations` for `nadi`, and `call_operation` to create a device
- **THEN** the created device in `nadi` shows the agent's client as `acted_by`

#### Scenario: Smoke memakai tool baru
- **WHEN** `scripts/smoke.sh` runs with `AGENT_USER_TOKEN` set
- **THEN** it lists exactly the three tools, calls `list_services` and `list_operations` for `nadi`, and lists devices through `call_operation`, without printing any token

#### Scenario: Folder lama tidak ada
- **WHEN** a partner looks for the agent example
- **THEN** the root README, `AGENTS.md`, and the other examples link to `agent/`, and `agent-backend/` no longer exists

### Requirement: Exchange hemat dan paralel antar konteks
The agent SHALL reuse an exchanged token per `(user, organization, project, service)` until it nears expiry or the subject token ends, SHALL reuse the delegated NICE API token per subject token for discovery, SHALL let calls for different contexts run concurrently, and SHALL retry a call once after a fresh exchange when the target answers 401 `token_stale`.

#### Scenario: Banyak panggilan
- **WHEN** the user calls `call_operation` for `GET /devices` of `nadi` ten times in one minute in the same context
- **THEN** the agent sends one exchange request for `nadi` and one for the NICE API

#### Scenario: Dua konteks sekaligus
- **WHEN** the MCP client calls `call_operation` for two different projects at the same time
- **THEN** both exchanges are in flight together

### Requirement: Image container
The example SHALL carry a `Dockerfile` that builds the dashboard UI with Node and a static binary with Go 1.27, and runs the binary on a distroless nonroot image, listening on `0.0.0.0:8083`, serving the UI at `/` and MCP at `/mcp`, and keeping the JWKS copy under `/data`.

#### Scenario: Build image
- **WHEN** a partner builds the image from `agent/`
- **THEN** the image builds without the `nice` repository and exposes port 8083

#### Scenario: Satu host untuk UI dan MCP
- **WHEN** the container runs behind one public host
- **THEN** `GET /` serves the dashboard, `/auth/*` and `/chat/*` serve its API, and `/mcp` answers MCP as before

### Requirement: Penyelesaian `perms_ref` tanpa kredensial tambahan
The agent SHALL always configure the `perms_ref` resolver of `verify` with the NICE issuer only, forwarding the token of the MCP request to NICE, and SHALL NOT read a NICE API key for it. `.env.example` and the README SHALL NOT mention `NICE_SERVICE_API_KEY`.

#### Scenario: Resolver tanpa konfigurasi
- **WHEN** the verifier config is built from a config that holds only the required variables
- **THEN** it carries a `*verify.HTTPResolver`

#### Scenario: Variabel lama tidak ada
- **WHEN** the configuration test reads `.env.example`
- **THEN** the file does not list `NICE_SERVICE_API_KEY`, and the README documents exactly the variables of `.env.example`

### Requirement: Discovery service dan operasi dari NICE
The agent SHALL offer the tools `list_services` and `list_operations`, answered from NICE's caller service map and service operations (via the `credentials` package of `github.com/ABA-Developer/nice-auth-sdk` at `v0.2.4` or later), with the MCP request's token as subject token. It SHALL NOT keep a hand-written list of services, operations, or service addresses.

#### Scenario: Peta service
- **WHEN** a user who holds `nadi` permissions in project Demo calls `list_services`
- **THEN** the result lists the user's organizations, each with its projects and the slugs of the services callable in each (including `nadi` under Demo), and the name and description of every listed service

#### Scenario: Proyek terpotong
- **WHEN** NICE marks an organization of the map as `projects_truncated`
- **THEN** the result of `list_services` marks that organization as incomplete

#### Scenario: Operasi satu service
- **WHEN** the user calls `list_operations` with `service` `nadi`
- **THEN** the result lists the operations of `nadi` whose permission the user holds, each with its key, method, path, summary, permission, request field names, and whether it is sensitive

#### Scenario: Service yang tidak boleh dipanggil
- **WHEN** the user calls `list_operations` with a slug NICE answers `404 not_found` for
- **THEN** the tool result is an error saying the user may not call that service, and no service is called

### Requirement: Panggilan operasi atas nama user
The agent SHALL offer the tool `call_operation` taking `service`, `operation`, optional `organization_id` and `project_id`, and optional `path_params`, `query`, and `body`. It SHALL resolve the service and operation from NICE at call time, never from the arguments, and SHALL send the call through the SDK's operation call, so the user's token reaches only NICE and the target receives an exchanged token.

#### Scenario: Daftar perangkat lewat operasi
- **WHEN** the user calls `call_operation` with `service` `nadi`, the operation key of `GET /devices`, and no context
- **THEN** the agent exchanges for `nadi` in the organization and project of the user's token, calls the catalog endpoint of `nadi` plus `/devices`, and returns the devices

#### Scenario: Buat perangkat lewat operasi
- **WHEN** the user calls `call_operation` with the operation key of `POST /devices` and a `body` with `name`, `serial_number`, and `status`
- **THEN** `nadi` receives that body as JSON and the created device shows the agent's client as `acted_by`

#### Scenario: Operasi tidak dikenal
- **WHEN** the user calls `call_operation` with an operation key that `list_operations` does not list for that service
- **THEN** the tool result is an `invalid_argument` error and no token for the target service is exchanged

#### Scenario: Parameter path tidak aman
- **WHEN** a path parameter is missing, unknown, empty, `.`, `..`, or holds a slash, a backslash, or a control character
- **THEN** the tool result is an `invalid_argument` error naming the parameter and no token for the target service is exchanged

#### Scenario: Query tidak dikenal
- **WHEN** `query` names a field that the operation does not list as a query field
- **THEN** the tool result is an `invalid_argument` error and no token for the target service is exchanged

#### Scenario: Tidak ada service api_key
- **WHEN** the user's organization uses the `api_key` service `pengiriman`
- **THEN** neither `list_services` nor `list_operations` offers `pengiriman`, and `call_operation` refuses it

### Requirement: Persetujuan untuk operasi sensitif
Before calling an operation marked sensitive, `call_operation` SHALL ask the user to approve through an MCP form elicitation and SHALL keep the server stateless. The call SHALL proceed only when the user accepts. Any approval SHALL be bound to the user and the exact call, SHALL be usable once, and SHALL expire. Reading the service map and operations (one exchange for the NICE API) MAY happen before the approval; no token for the target service SHALL be exchanged before it.

#### Scenario: User menyetujui
- **WHEN** the user calls `call_operation` for the sensitive delete operation of `nadi` with a device ID, and approves in the MCP client
- **THEN** the agent sends the delete to `nadi` once and returns its answer

#### Scenario: User menolak atau membatalkan
- **WHEN** the user declines or cancels the approval
- **THEN** the tool result is an error saying the user did not approve, and no token for the target service is exchanged and nothing is sent to it

#### Scenario: Client tanpa elicitation
- **WHEN** the MCP client supports neither form elicitation nor multi round-trip requests
- **THEN** the sensitive operation is refused with an error saying approval is not possible in this client, and no token for the target service is exchanged and nothing is sent to it

#### Scenario: Persetujuan dipakai ulang untuk panggilan lain
- **WHEN** a retry carries an approval state issued for another user, another operation, other arguments, or one that has expired or been altered
- **THEN** the agent does not call the operation and asks for approval again or refuses

#### Scenario: Operasi tidak sensitif
- **WHEN** the user calls a non-sensitive operation
- **THEN** no approval is asked

### Requirement: Alamat service dari katalog
The agent SHALL call each service at the endpoint and base path that NICE's service map gives, and SHALL NOT read `NADI_URL` or any other service address from its configuration. `.env.example` and the README SHALL NOT mention `NADI_URL`, and the README SHALL state that a catalog endpoint must be `https` unless its host is a loopback.

#### Scenario: Konfigurasi tanpa alamat service
- **WHEN** the agent starts with only `NICE_ISSUER`, `NICE_CLIENT_ID`, and `NICE_CLIENT_SECRET` set
- **THEN** it starts, and the configuration test finds no `NADI_URL` in `.env.example`

#### Scenario: Endpoint http non-loopback
- **WHEN** the catalog endpoint of a service is `http` to a host that is not a loopback
- **THEN** `call_operation` returns an error and sends no token to that endpoint
