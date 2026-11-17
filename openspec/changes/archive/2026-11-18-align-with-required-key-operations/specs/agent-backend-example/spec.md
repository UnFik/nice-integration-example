## REMOVED Requirements

### Requirement: Tool memanggil service atas nama user
**Reason**: Service `api_key` kini memakai kunci per Service Account dan menolak principal user, jadi tool `pengiriman_*` tidak dapat berfungsi atas nama user. Requirement digantikan versi yang hanya mencakup `inventaris`.
**Migration**: Lihat requirement "Tool memanggil inventaris atas nama user".

### Requirement: Exchange hemat dan paralel
**Reason**: Skenario "Dua service sekaligus" bergantung pada tool `pengiriman_*`. Diganti dengan paralelisme antar konteks pada `inventaris`.
**Migration**: Lihat requirement "Exchange hemat dan paralel antar konteks".

## MODIFIED Requirements

### Requirement: Panduan setup dan uji
The example SHALL carry `README.md` and `README.en.md` with the NICE version it needs, the manual setup checklist (service `agen` and its import, public and confidential service clients bound to `agen`, a dependency on `inventaris` with read and create permissions, role binding), how to obtain a user token, how to call the tools with MCP Inspector and `curl`, and a test checklist that shows `acted_by` in `inventaris`. The README SHALL explain that `api_key` services use a key per Service Account and refuse user tokens, so this agent offers tools for `nice_token` services only.

#### Scenario: Mitra mengikuti README
- **WHEN** a partner follows the setup and calls `inventaris_create_item`
- **THEN** the created item in `inventaris` shows the agent's client as `acted_by`

## ADDED Requirements

### Requirement: Tool memanggil inventaris atas nama user
Each tool SHALL obtain a token for `inventaris` with token exchange (`pkg/niceclient` exchange mode), using the user's token from the MCP request as subject token and the agent's confidential service client as actor, and SHALL call `inventaris` with that token only. The user's own token SHALL NOT be sent to any service other than NICE's token endpoint. The agent SHALL NOT expose tools for `api_key` services.

#### Scenario: Daftar barang
- **WHEN** the user calls `inventaris_list_items` without arguments
- **THEN** the agent exchanges for `nice:aud:inventaris` in the user's token organization and project, calls `inventaris` with the exchanged token, and returns the items

#### Scenario: Tidak ada tool pengiriman
- **WHEN** an MCP client lists the tools
- **THEN** no tool targets `pengiriman`

### Requirement: Exchange hemat dan paralel antar konteks
The agent SHALL reuse an exchanged token per `(user, organization, project, service)` until it nears expiry or the subject token ends, SHALL let calls for different contexts run concurrently, and SHALL retry a call once after a fresh exchange when the target answers 401 `token_stale`.

#### Scenario: Banyak panggilan
- **WHEN** the user calls `inventaris_list_items` ten times in one minute in the same context
- **THEN** the agent sends one exchange request

#### Scenario: Dua konteks sekaligus
- **WHEN** the MCP client calls `inventaris_list_items` for two different projects at the same time
- **THEN** both exchanges are in flight together
