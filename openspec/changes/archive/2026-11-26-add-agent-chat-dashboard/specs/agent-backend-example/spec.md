# Spec Delta

## MODIFIED Requirements

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

### Requirement: Image container
The example SHALL carry a `Dockerfile` that builds the dashboard UI with Node and a static binary with Go 1.27, and runs the binary on a distroless nonroot image, listening on `0.0.0.0:8083`, serving the UI at `/` and MCP at `/mcp`, and keeping the JWKS copy under `/data`.

#### Scenario: Build image
- **WHEN** a partner builds the image from `agent/`
- **THEN** the image builds without the `nice` repository and exposes port 8083

#### Scenario: Satu host untuk UI dan MCP
- **WHEN** the container runs behind one public host
- **THEN** `GET /` serves the dashboard, `/auth/*` and `/chat/*` serve its API, and `/mcp` answers MCP as before
