# agent-backend (`agen`)

[Bahasa Indonesia](README.md)

An example AI agent backend: an MCP server (Streamable HTTP transport) for the NICE service `agen`, written in Go with the [official MCP Go SDK](https://github.com/modelcontextprotocol/go-sdk). The agent writes no tool list per service: it asks NICE which services the user may call in which project, and their operations (*caller service discovery*), then calls an operation **on behalf of the user** of the agent, with a token from token exchange (RFC 8693):

- the user's token goes only to the NICE token endpoint, never to another service (no *token passthrough*);
- no Service Account across organizations: the permissions of the target token are the user's in the target project, with no further limit from NICE (see "Permission limit");
- the agent keeps no refresh token of the user;
- the target service sees the agent as actor (`act.sub`), and `nadi` records it as `acted_by`;
- an operation the catalog marks sensitive is sent only after the user approves it in the MCP client.

```text
MCP client --Bearer user token (aud=agen)--> agen /mcp
agen --token exchange (aud=nice)--> NICE /oauth/token, then GET /api/v1/me/services and /me/services/nadi/operations
agen --token exchange: subject_token + client agen-backend (aud=nadi)--> NICE /oauth/token
agen --Bearer exchanged token (act.sub=agen-backend)--> <catalog endpoint of nadi>/devices
```

Minimum NICE version: `235445c` (`add-caller-service-discovery`), because the tools use `GET /api/v1/me/services` and `GET /api/v1/me/services/{slug}/operations`; an older NICE answers `404` and the tools answer `discovery_unavailable`. Before it, `b361d25` (`service-calls-without-service-account`), because the resolution of `perms_ref` forwards the token of the request, not an API key. SDK: `github.com/ABA-Developer/nice-auth-sdk`, `v0.2.4` (`Services`, `ServiceOperations`, `CallOperation`). The MCP SDK is pinned in `go.mod` at `v1.8.0`, which has MCP protocol `2026-07-28` for approvals without a session.

Tested against nice revision: SDK `v0.2.4` and `nice` `235445c`, 2026-11-25, with the unit tests (fake NICE and `nadi`), then `scripts/smoke.sh` and the delete with an approval (two rounds of protocol `2026-07-28`) against NICE `dev-235445c` on a VPS, with an `https` endpoint of `nadi`.

## Folder structure

The layout follows `nice/internal/modules` (see `nice/AGENTS.md`), trimmed for one small service: one resource (`call`: discovery and calls to other services on behalf of the user), no facade and no repository.

```text
cmd/agent-backend/main.go            thin: calls bootstrap.Run
docs/                                openapi.json (hand-written) + a small package that embeds it; OpenAPI and README tests
internal/
  domain/                            pure rules: slug, permission, targets, ID format, safe endpoints (*_domain.go)
  app/call/
    usecase/                         ListServices, ListOperations, CallOperation (one *_usecase.go per function), Service,
                                     the Discoverer and Operator contracts (call_contract.go)
    dto/                             Caller, service map, operations, approval, Failure (*_dto.go)
    internal/{key,failure,approval}/ ID checks, mapping of errors to Failure, signed approval state (*_helper.go)
  adapter/
    mcp/                             HTTP routes and the niceauth/verify chain (routes.go), MCP server and tool schemas (tools_routes.go),
                                     one handler per tool (*_handler.go); internal/ holds caller, challenge, mapper, approval
    niceexchange/                    wraps niceauth: Services, ServiceOperations (discovery_client.go), CallOperation
                                     (operation_client.go)
  platform/config/                   env and .env
  bootstrap/                         assembly; integration tests with a fake NICE
  arch_test.go                       architecture test: file name endings and import direction
scripts/smoke.sh                     smoke test against a NICE that is set up
```

Why: `main.go` only assembles; the rules and the use case do not import `net/http`, the MCP SDK, `niceauth/credentials`, or `niceauth/verify` (the adapters do, enforced by `internal/arch_test.go`); every file carries its layer in the name ending (`_domain`, `_usecase`, `_contract`, `_dto`, `_handler`, `_routes`, `_client`, `_helper`, `_mapper`); one `*_usecase.go` holds one function named like its file. `openapi.json` stays hand-written and is embedded from `docs/`; `docs/openapi_test.go` keeps the permission in the document equal to the one the code enforces.

## Endpoints and tools

| Endpoint | Access |
|---|---|
| `POST /mcp` | NICE token with `aud=agen` and the permission `agen.mcp.use` |
| `GET /.well-known/oauth-protected-resource` (also `.../mcp`) | public: `resource` `<PUBLIC_URL>/mcp`, `authorization_servers` the NICE issuer, `scopes_supported` the base scopes and `nice:aud:agen` |
| `GET /healthz`, `GET /openapi.json` | public, not in the OpenAPI document |

Without a token, `/mcp` answers `401` with `WWW-Authenticate: Bearer resource_metadata="<PUBLIC_URL>/.well-known/oauth-protected-resource"`. A token for another service gets `401` `wrong_audience`, a token without `agen.mcp.use` gets `403` `permission_denied`.

| Tool | Arguments | Answer |
|---|---|---|
| `list_services` | none | the organizations of the user, their projects, and the slugs of the services the user may call in each project, with the name and description of each service. An organization whose projects NICE lists only in part is marked `projects_truncated` |
| `list_operations` | `service` | the operations of that service whose permission the user holds: `key`, `method`, `path`, `summary`, `permission`, `request_fields` (name, location `path`/`query`/`body`, required or not), and `sensitive` |
| `call_operation` | `service`, `operation` (key), `organization_id`, `project_id`, `path_params`, `query`, `body` | the `status` and JSON `body` of the answer of the service |

The tool list is the same for every user; what a user may call is read from NICE when a tool runs, from the roles of the user at that moment, not from a claim of the token. The agent keeps neither the map nor the operations.

`call_operation` matches `service` with the map and `operation` with the list of operations on every call. The address (endpoint and base path of the catalog), the method, and the headers of the operation never come from the arguments of the model. Before any exchange for the target service, the agent refuses with `invalid_argument`:

- a `service` that is not a slug, or an `organization_id`/`project_id` not shaped like `org_...`/`prj_...` (those values become scopes);
- another organization without `project_id`, or a project the map does not list for that service in that organization (unless NICE marks the projects of that organization `projects_truncated`);
- an operation key that does not exist, a `query` name that is not a `query` field of the operation;
- a path parameter that is missing, unknown, empty, `.`, `..`, or holds `/`, `\`, or a control character (checked by the SDK).

Without `organization_id` and `project_id`, the exchange asks only for `nice:aud:<service>`, and NICE uses the organization and project of the user's token. `body` is sent as it is, as JSON; the target service checks it, and `nadi` refuses unknown fields.

### Approval of sensitive operations

The exchanged token carries the user's permissions in the target project, the same as a service client token of that user (`nice/docs/api.md`, "Token exchange: backend atas nama user"). NICE does not cut them down for the actor: when the user holds `nadi.devices.delete`, the token the agent holds does too. Since the tool list no longer limits the operations, the limit the agent adds is an approval: an operation the catalog marks `sensitive` (in `nadi`: `DELETE /devices/{id}`) is sent only after the user approves it.

The MCP server runs without sessions, so the agent cannot send an elicitation request of its own to the client. It uses the multi round-trip pattern of MCP protocol `2026-07-28` (SEP-2322):

1. The first `call_operation` for a sensitive operation answers `resultType: input_required` with `inputRequests.approve` (a form elicitation: a message that names the service, the summary of the operation, the method and filled path, the organization, and the project; schema `{approve: boolean}`) and a `requestState`.
2. The MCP client shows the form to the user, then calls the same tool again with the same arguments, `inputResponses.approve` (`action: accept`, `approve: true`), and the `requestState`.
3. The agent sends the operation once. `decline`, `cancel`, or `approve: false` answers `not_approved`, with no exchange for the target service and no call.

`requestState` is an opaque state signed with HMAC and bound to the user and the exact call (service, operation, context, parameters, query, body), good for 5 minutes and for one use. A state for another user, another call, one that was altered, expired, or already used makes the agent ask for a new approval. The key is made at random when the process starts, so a restart drops pending approvals.

An MCP client on an older protocol, or one that declares no form elicitation, cannot be asked: a sensitive operation answers `approval_unavailable` and is not sent. Other operations still run. A write that is not sensitive (e.g. `POST /devices`) asks for no approval; the catalog administrator decides which operations are sensitive. For testing, give the user `nadi.editor` (no delete permission), or `nadi.admin` to try the approval.

### Why only `nice_token` services

An `api_key` service uses a key per Service Account and refuses user tokens (`403` `service_account_required`), so an agent that acts on behalf of the user calls `nice_token` services only. NICE leaves `api_key` services out of the map and answers `404` for their operations, so the `api_key` example service in this repo, `pengiriman`, answers `service_not_allowed`. Calling an `api_key` service through a Service Account that belongs to the agent would drop the identity of the user (the target service would no longer see the user as `created_by`) and is not the pattern of this example.

### How a call works

`list_services` and `list_operations` call `credentials.ExchangeClient.Services` and `ServiceOperations` with the user's token from the MCP request. Both share one delegated token of the NICE API (`aud=nice`) per user token, so discovery adds one exchange per user token, not per service or project. `call_operation` reads the map and the operations again (two `GET`s to NICE), then calls `CallOperation`, which sends through `DoAs` with the key `{Org, Project, Audience}`. `DoAs`:

- keeps a token per `(user, organization, project, service)` until 60 seconds before it expires, and never uses a token that outlives the user's token;
- runs one exchange per key for parallel calls, with no queue across keys, so different contexts (organization or project) are exchanged at the same time;
- exchanges again and resends the request once when the service answers `401` `token_stale` or `expired`.

A catalog endpoint must be `https` unless its host is a loopback: the agent refuses another endpoint with `insecure_endpoint` before any token is sent to it, the same rule as the SDK.

The MCP transport is *stateless* with JSON answers: every `POST /mcp` stands alone, so `curl` can call a tool in one request and parallel calls of one client do not wait for each other.

### Errors

Failures come back as tool results marked as errors (`isError: true`) with the code, a short reason from NICE or the service when there is one, and an explanation. Tokens, secrets, and raw bodies never reach the result or the log.

| Code | Source | Meaning |
|---|---|---|
| `discovery_unavailable` | NICE | NICE has no caller service discovery yet: upgrade NICE |
| `service_not_allowed` | NICE, agent | the user may not call that service in any project, or it is not a `nice_token` service |
| `token_stale_after_upgrade` | NICE | the delegated token was issued before NICE was upgraded (`403 delegation_not_allowed`): get a new user token |
| `insecure_endpoint` | agent | the catalog endpoint of that service is `http` to a host that is not a loopback |
| `approval_unavailable` | agent | a sensitive operation in an MCP client that cannot ask for an approval |
| `not_approved` | agent | the user declined or dismissed the approval |
| `access_denied` | exchange | the user is not an active member of that organization, the organization is not active, or it does not entitle the target service or `agen` |
| `invalid_scope` | exchange | the target service is not active, or the project is not active in that organization or does not use the target service |
| `invalid_grant` (`subject_token_expiring`) | exchange | the user's token has less than 60 seconds left: refresh it in the MCP client |
| `invalid_grant` (`token_stale`, `subject_invalid`) | exchange | the user's access changed, the token is not a user token for `agen`, or the session ended: sign in again |
| `invalid_client`, `unauthorized_client` | exchange | the agent's client is refused: check `NICE_CLIENT_ID` and `NICE_CLIENT_SECRET` |
| `nice_unavailable` | exchange | NICE did not answer |
| `insufficient_scope` (`permission_denied`), `permission_denied` | service | the user does not hold the permission for that operation in that project |
| `invalid_argument` | agent, service | the arguments do not fit the map or the operation (see "Endpoints and tools"); a device field is not valid (`nadi`) |

Example text of a tool result for an organization the user does not belong to:

```text
access_denied (organization is not usable): NICE refused a token for nadi in that organization: the user is not an active member of it, the organization is not active, or it does not use nadi or agen
```

## Prerequisites

- The SDK comes from the public module `github.com/ABA-Developer/nice-auth-sdk`, so this example builds without the `nice` repo. The code needs SDK `v0.2.4` or later. The `nice` repo next to `example/` is needed only to run a local NICE and for `nice/scripts/dev-token.sh`.
- Go 1.27, `curl`, `jq`, and `openssl` (for `nice/scripts/dev-token.sh`).
- NICE at the minimum version above, by default at `http://localhost:8080`.
- `nadi` running and set up through step 8 of [`../sharedservice-oauth/backend/README.en.md`](../sharedservice-oauth/backend/README.en.md) (register, import, activate, entitlement, project activation, binding). The catalog endpoint of `nadi` is the address the agent calls, so it must be `https` or a loopback (locally: `http://127.0.0.1:8081`). The test user holds the role `nadi.editor` in the same project, or `nadi.admin` to try the approval of a delete. Do not use `nadi.operator`: that is the platform operator role of the service, not a project role.
- For the other-organization test: the test user is also a member of a second organization, with a `nadi` entitlement, a project that has `nadi` active, and a `nadi.editor` binding there (steps 6 to 8 of the `nadi` README for that organization).

## Configuration

Copy `.env.example` to `.env`. The agent reads `.env` from the directory it runs in; a variable set in the environment wins over the file.

| Variable | Default | Meaning |
|---|---|---|
| `NICE_ISSUER` | required | the NICE issuer, exactly as the `iss` claim of its tokens |
| `NICE_CLIENT_ID` | required | `client_id` of the confidential client `agen-backend` (step 7) |
| `NICE_CLIENT_SECRET` | required | the secret of that client. Sent to the NICE token endpoint only |
| `LISTEN_ADDR` | `127.0.0.1:8083` | listen address |
| `PUBLIC_URL` | `http://127.0.0.1:8083` | the address MCP clients use; the metadata `resource` is `<PUBLIC_URL>/mcp` |
| `JWKS_FILE` | `data/jwks.json` | copy of the JWKS on disk (`data/` is not committed) |

An empty required variable stops the start with a message that names it. No service address is configured: the agent calls each service at the endpoint of the NICE catalog.

The resolution of `perms_ref` (a caller whose `agen` permissions do not fit a token) is always on, with no configuration: the agent forwards the token of the request itself to NICE (`POST /api/v1/permission-resolutions` with `Authorization: Bearer`). The agent holds no NICE credential besides the service client `agen-backend`. When NICE is unreachable or refuses that token, the call answers `503`.

## Run and test

```bash
make run          # go run ./cmd/agent-backend on 127.0.0.1:8083
make test         # go test -race; needs no NICE (fake NICE and nadi)
make smoke        # scripts/smoke.sh, needs a NICE set up with the checklist below
```

When `go` on `PATH` does not match `GOROOT`, call `make GO=/path/to/go test`.

The tests run the agent over in-process HTTP with the MCP client of the SDK, in front of a fake NICE that follows the token exchange and discovery rules. The tests that keep the pattern: the fake services fail when they ever receive a token with `aud=agen` (no passthrough), ten calls in one context give one exchange for `nadi` and one for the NICE API, the exchanges for two different contexts (organization or project) run at the same time, and a sensitive operation is sent exactly once after an approval and never without one.

### Running in a container

The `Dockerfile` builds a static binary with Go 1.27 and runs it on the distroless `nonroot` image. The image sets `LISTEN_ADDR=0.0.0.0:8083` and `JWKS_FILE=/data/jwks.json`; `.env` does not go into the image, so the configuration comes from the environment.

```bash
docker build -t agen .
docker run --rm -p 8083:8083 -v agen-data:/data \
  -e NICE_ISSUER=https://nice.example.com \
  -e PUBLIC_URL=https://agen.example.com -e NICE_CLIENT_ID -e NICE_CLIENT_SECRET agen
```

- `PUBLIC_URL` must be the public `https` address MCP clients use: the resource metadata names `<PUBLIC_URL>/mcp`, and the endpoint of `agen` in NICE must point to the same address.
- `NICE_ISSUER` and the catalog endpoint of every service must be `https` unless the host is a loopback: the SDK refuses `http` to another host, and inside a container `127.0.0.1` is the container itself. A catalog endpoint that only works inside a Docker network (e.g. `http://nadi:8081`) answers `insecure_endpoint`.
- The secret is passed from the shell environment (`-e NICE_CLIENT_SECRET` without a value), not written on the command line.

## NICE setup checklist

Run from the `example/agent-backend` directory, in one shell session. Console steps name their page; the `curl` snippets are the equivalent. Passwords are read from the terminal and never written to a file.

```bash
export NICE=${NICE:-http://localhost:8080}   # same as NICE_ISSUER in .env; dev-token.sh reads it too

read -rp 'Platform Admin email: ' ADMIN_EMAIL; read -rsp 'Password: ' ADMIN_PASSWORD; echo
ADMIN_TOKEN=$(../../nice/scripts/dev-token.sh "$ADMIN_EMAIL" "$ADMIN_PASSWORD"); unset ADMIN_PASSWORD
read -rp 'Tenant organization owner email: ' OWNER_EMAIL; read -rsp 'Password: ' OWNER_PASSWORD; echo
OWNER_TOKEN=$(../../nice/scripts/dev-token.sh "$OWNER_EMAIL" "$OWNER_PASSWORD"); unset OWNER_PASSWORD

ORG_ID=$(curl -sS "$NICE/api/v1/me/organization" -H "Authorization: Bearer $OWNER_TOKEN" | jq -r .organization.id)
PROJECT_ID=prj_...   # the tenant project where nadi is active
USER_ID=usr_...      # the test user of the agent
PLATFORM_ORG_ID=$(curl -sS "$NICE/api/v1/me/organization" -H "Authorization: Bearer $ADMIN_TOKEN" | jq -r .organization.id)
```

Tokens last 30 minutes; get new ones when `curl` answers 401.

### 1. NICE environment

Same as the other service examples: `nice/.env` holds `NICE_OUTBOUND_ALLOW_HTTP=true` and `NICE_OUTBOUND_ALLOW_CIDRS=127.0.0.0/8`, and NICE is restarted. Without them NICE refuses the endpoint `http://127.0.0.1:8083`.

### 2. Register `agen`

Console: Platform Admin, **Shared Service**, **Register Shared Service**. The default mode is `nice_token`.

```bash
curl -sS "$NICE/api/v1/shared-services" -X POST \
  -H "Authorization: Bearer $ADMIN_TOKEN" -H 'Content-Type: application/json' \
  -d '{"slug":"agen","name":"Agen","endpoint":"http://127.0.0.1:8083","health_path":"/healthz"}' | jq '.shared_service | {slug, status}'
```

### 3. Import `docs/openapi.json`

Console: the detail of service `agen`, catalog import. The document comes from the file, because the agent can run only after its client exists (step 7).

```bash
PREVIEW=$(jq -n --rawfile doc docs/openapi.json '{document: $doc}' | curl -sS "$NICE/api/v1/shared-services/agen/catalog-imports" -X POST \
  -H "Authorization: Bearer $ADMIN_TOKEN" -H 'Content-Type: application/json' -d @-)
jq '{added: [.diff.added[]? | {key, permission}], roles: [.diff.roles[]? | .key]}' <<<"$PREVIEW"
curl -sS "$NICE/api/v1/shared-services/agen/catalog-imports/$(jq -r .import_id <<<"$PREVIEW")/apply" -X POST \
  -H "Authorization: Bearer $ADMIN_TOKEN" -H 'Content-Type: application/json' -d "$(jq '{hash}' <<<"$PREVIEW")"
```

The preview shows one operation `mcp` (`agen.mcp.use`) and the roles `agen.admin` and `agen.user` (from `x-nice-roles`). There is no `agen.viewer`, because the document has no read operation.

### 4. Activate the service

```bash
curl -sS "$NICE/api/v1/shared-services/agen/activate" -X POST -H "Authorization: Bearer $ADMIN_TOKEN" | jq '.shared_service | {slug, status}'
```

### 5. Project in the platform organization

A service client must belong to a project of the platform organization. Use an existing project, or create one (console: Platform Admin, platform organization, **Project**).

```bash
AGENT_PROJECT_ID=$(curl -sS "$NICE/api/v1/organizations/$PLATFORM_ORG_ID/projects" -X POST \
  -H "Authorization: Bearer $ADMIN_TOKEN" -H 'Content-Type: application/json' \
  -d '{"name":"Agen","slug":"agen"}' | jq -r .project.id)
```

### 6. Client `agen-user` (public)

The client through which users and MCP clients get tokens with `aud=agen`. Console: that project, **OAuth client**, then bind it to `agen` (needs `nice.platform.catalog.manage`). The redirect URIs below are those of MCP Inspector; `dev-token.sh` uses the first one.

```bash
CLIENT=$(curl -sS "$NICE/api/v1/projects/$AGENT_PROJECT_ID/clients" -X POST \
  -H "Authorization: Bearer $ADMIN_TOKEN" -H 'Content-Type: application/json' \
  -d '{"name":"agen-user","client_type":"CLIENT_TYPE_PUBLIC",
       "redirect_uris":["http://localhost:6274/oauth/callback","http://localhost:6274/oauth/callback/debug"],
       "grant_types":["authorization_code","refresh_token"]}')
USER_CLIENT_ID=$(jq -r .oauth_client.client_id <<<"$CLIENT")
curl -sS "$NICE/api/v1/projects/$AGENT_PROJECT_ID/clients/$(jq -r .oauth_client.id <<<"$CLIENT")/service" -X PUT \
  -H "Authorization: Bearer $ADMIN_TOKEN" -H 'Content-Type: application/json' -d '{"service":"agen"}' \
  | jq '.oauth_client | {client_class, service}'
# {"client_class":"service","service":"agen"}
```

### 7. Client `agen-backend` (confidential)

The actor of the token exchange. The exchange refuses `public` clients, and keeping two clients shows that the secret lives on the agent server only. NICE requires a redirect URI and the `authorization_code` grant on every OAuth client, even though this backend never signs in.

```bash
CLIENT=$(curl -sS "$NICE/api/v1/projects/$AGENT_PROJECT_ID/clients" -X POST \
  -H "Authorization: Bearer $ADMIN_TOKEN" -H 'Content-Type: application/json' \
  -d '{"name":"agen-backend","client_type":"CLIENT_TYPE_CONFIDENTIAL","redirect_uris":["http://127.0.0.1:8083/unused"],
       "grant_types":["authorization_code"]}')
curl -sS "$NICE/api/v1/projects/$AGENT_PROJECT_ID/clients/$(jq -r .oauth_client.id <<<"$CLIENT")/service" -X PUT \
  -H "Authorization: Bearer $ADMIN_TOKEN" -H 'Content-Type: application/json' -d '{"service":"agen"}' \
  | jq '.oauth_client | {client_class, service}'
cp -n .env.example .env && chmod 600 .env
sed -i '/^NICE_CLIENT_ID=/d; /^NICE_CLIENT_SECRET=/d' .env
jq -r '"NICE_CLIENT_ID=" + .oauth_client.client_id, "NICE_CLIENT_SECRET=" + .client_secret' <<<"$CLIENT" >> .env
unset CLIENT
```

The secret is shown once and goes straight into `.env` without being printed. If it is lost, rotate the client secret in the console and replace `NICE_CLIENT_SECRET`.

### 8. User organization: entitlement, activation, binding

Sign-in through a service client completes only in an organization that entitles `agen`. An exchange into another organization checks the same, plus the entitlement of the target service, but does not need `agen` active in a project of that organization. Console: Platform Admin, **Organizations** (entitlement), then the tenant project, **Product & Activation** and **IAM**.

```bash
curl -sS "$NICE/api/v1/organizations/$ORG_ID/entitlements" -X POST \
  -H "Authorization: Bearer $ADMIN_TOKEN" -H 'Content-Type: application/json' -d '{"service":"agen"}'
curl -sS "$NICE/api/v1/projects/$PROJECT_ID/services/agen/activation" -X PUT -H "Authorization: Bearer $OWNER_TOKEN"
curl -sS "$NICE/api/v1/projects/$PROJECT_ID/role-bindings" -X POST \
  -H "Authorization: Bearer $OWNER_TOKEN" -H 'Content-Type: application/json' \
  -d "{\"principal_type\":\"PRINCIPAL_TYPE_USER\",\"principal_id\":\"$USER_ID\",\"role\":\"agen.user\"}" | jq -r .role_binding.id
```

IAM copies the catalog and activations asynchronously: when the binding answers `unknown_role` or `service_not_active`, wait a few seconds and try again. For the other-organization test, give the second organization the `agen` entitlement too.

### 9. Run the agent

```bash
make run
curl -sS http://127.0.0.1:8083/healthz
```

Leave it running in another terminal.

### 10. User token

The user token for the agent is requested through the client `agen-user` with `nice:aud:agen` and the project where the user holds `agen.user`. `nice:org` makes the sign-in land in that organization.

```bash
read -rp 'Test user email: ' USER_EMAIL; read -rsp 'Password: ' USER_PASSWORD; echo
USER_TOKEN=$(CLIENT_ID=$USER_CLIENT_ID REDIRECT=http://localhost:6274/oauth/callback \
  ../../nice/scripts/dev-token.sh "$USER_EMAIL" "$USER_PASSWORD" "" "openid nice:org:$ORG_ID nice:aud:agen nice:prj:$PROJECT_ID")
unset USER_PASSWORD
export AGENT_USER_TOKEN=$USER_TOKEN   # read by scripts/smoke.sh
```

### 11. Smoke

```bash
make smoke
```

The script reads `.env` and `AGENT_USER_TOKEN` from the environment, never prints a token or the secret, and checks: NICE answers, `/healthz` answers, `/mcp` without a token answers `401` with `resource_metadata`, the metadata names `NICE_ISSUER`, then, with the user token, `tools/list` holds the three tools, `list_services` lists `nadi`, `list_operations` for `nadi` lists `GET /devices`, and `call_operation` of that operation succeeds. `AGENT_URL` points the script at an agent elsewhere (e.g. `AGENT_URL=https://agen.example.com make smoke`). Failures map to steps:

| Answer | Step likely missed |
|---|---|
| no `.env`, agent unreachable | 7, 9 |
| `/mcp` with the token answers `401` `wrong_audience` | 10: the token must be requested with `nice:aud:agen` through `agen-user` |
| `/mcp` answers `403` `permission_denied` | 8: the `agen.user` binding |
| a tool answers `discovery_unavailable` | NICE is older than the minimum version |
| a tool answers `service_not_allowed`, or `list_services` has no `nadi` | steps 6 to 8 of the `nadi` README for that user and project |
| a tool answers `insecure_endpoint` | the catalog endpoint of `nadi` is not `https` or a loopback |
| a tool answers `invalid_client` | 7: `NICE_CLIENT_ID` or `NICE_CLIENT_SECRET` |
| a tool answers `invalid_scope` | `nadi` is not active in the project of the token (step 7 of the `nadi` README) |
| a tool answers `invalid_grant` | 10: the user token is about to expire, stale, or its session ended; get a new one |

## Test checklist

With `USER_TOKEN` from step 10. The function `tool` calls one tool in one request:

```bash
tool() {   # tool <name> [arguments JSON]
  jq -nc --arg name "$1" --argjson args "${2:-"{}"}" \
    '{jsonrpc: "2.0", id: 1, method: "tools/call", params: {name: $name, arguments: $args}}' |
  curl -sS http://127.0.0.1:8083/mcp -H "Authorization: Bearer $USER_TOKEN" \
    -H 'Content-Type: application/json' -H 'Accept: application/json, text/event-stream' -d @- |
  jq '.result | {isError, structuredContent, text: .content[0].text}'
}
```

1. **Tool list.** Three tools: `list_services`, `list_operations`, `call_operation`:

   ```bash
   curl -sS http://127.0.0.1:8083/mcp -H "Authorization: Bearer $USER_TOKEN" -H 'Content-Type: application/json' \
     -H 'Accept: application/json, text/event-stream' -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}' | jq '[.result.tools[].name]'
   ```

2. **Discovery.** The map lists `nadi` in the project of the user; the operations of `nadi` list `GET /devices` and `DELETE /devices/{id}` with `sensitive: true` (when the user holds the delete permission):

   ```bash
   tool list_services
   tool list_operations '{"service":"nadi"}' | jq '.structuredContent.operations[] | {key, method, path, sensitive}'
   ```

   Note the keys of `POST /devices`, `GET /devices`, and `DELETE /devices/{id}` as `CREATE`, `LIST`, and `DELETE`.

3. **Device created by the agent.** `created_by` is the user and `acted_by` is the `client_id` of `agen-backend` (`NICE_CLIENT_ID`):

   ```bash
   tool call_operation "{\"service\":\"nadi\",\"operation\":\"$CREATE\",
     \"body\":{\"name\":\"Sensor Suhu\",\"serial_number\":\"SN-2026-0001\",\"status\":\"active\",\"location\":\"Gudang A\"}}"
   tool call_operation "{\"service\":\"nadi\",\"operation\":\"$LIST\"}"
   ```

4. **Another organization of the user.** With a second organization (`ORG_B_ID`, project `PROJECT_B_ID`) that meets the prerequisites and entitles `agen`:

   ```bash
   tool call_operation "{\"service\":\"nadi\",\"operation\":\"$LIST\",\"organization_id\":\"$ORG_B_ID\",\"project_id\":\"$PROJECT_B_ID\"}"
   ```

   It succeeds, with the devices of that organization. A project that `list_services` does not list for `nadi` answers `invalid_argument` with no exchange.

5. **Delete with an approval.** With a user who holds `nadi.devices.delete` and `DEVICE_ID` from step 3. A first call without the headers of protocol `2026-07-28` answers `approval_unavailable` and deletes nothing:

   ```bash
   tool call_operation "{\"service\":\"nadi\",\"operation\":\"$DELETE\",\"path_params\":{\"id\":\"$DEVICE_ID\"}}"
   ```

   A client of protocol `2026-07-28` sends the headers `MCP-Protocol-Version`, `Mcp-Method`, `Mcp-Name`, and a `_meta` with the elicitation capability in every request. The first round returns the approval message and a `requestState`; the second sends the answer of the user with that state:

   ```bash
   approve() {   # approve <arguments JSON> [state] [true|false]
     jq -nc --argjson args "$1" --arg state "${2:-}" --argjson ok "${3:-true}" '
       {jsonrpc: "2.0", id: 1, method: "tools/call", params: ({name: "call_operation", arguments: $args,
         _meta: {"io.modelcontextprotocol/protocolVersion": "2026-07-28",
                 "io.modelcontextprotocol/clientInfo": {name: "curl", version: "1"},
                 "io.modelcontextprotocol/clientCapabilities": {elicitation: {form: {}}}}}
         + (if $state == "" then {} else {requestState: $state,
             inputResponses: {approve: {action: "accept", content: {approve: $ok}}}} end))}' |
     curl -sS http://127.0.0.1:8083/mcp -H "Authorization: Bearer $USER_TOKEN" -H 'Content-Type: application/json' \
       -H 'Accept: application/json, text/event-stream' -H 'MCP-Protocol-Version: 2026-07-28' \
       -H 'Mcp-Method: tools/call' -H 'Mcp-Name: call_operation' -d @-
   }
   ARGS="{\"service\":\"nadi\",\"operation\":\"$DELETE\",\"path_params\":{\"id\":\"$DEVICE_ID\"}}"
   FIRST=$(approve "$ARGS"); jq '.result | {resultType, message: .inputRequests.approve.params.message}' <<<"$FIRST"
   approve "$ARGS" "$(jq -r .result.requestState <<<"$FIRST")" true | jq '.result | {isError, structuredContent}'
   ```

   The second round answers `status: 204` and the device is gone from `$LIST`. Sending the same state again, or that state with another `DEVICE_ID`, answers with a new approval request; `false` answers `not_approved`.

6. **An organization that is not the user's.** The result is an error, and `nadi` is not called: `invalid_argument` when the organization is not in the map, or `access_denied` from the exchange:

   ```bash
   tool call_operation "{\"service\":\"nadi\",\"operation\":\"$LIST\",\"organization_id\":\"$OTHER_ORG_ID\",\"project_id\":\"$OTHER_PROJECT_ID\"}"
   ```

7. **NICE audit.** Every exchange is recorded as `token.exchange` with the user as actor and `details` `client_id`, `service`, and `project_id`, including the exchange for the NICE API (`service` `nice`). A successful exchange is recorded in the target organization. A refusal is recorded in the organization of the user's token, with `details.reason` (e.g. `not_member`, `invalid_scope`) and `details.target_organization_id`:

   ```bash
   curl -sS "$NICE/api/v1/audit/records?action=token.exchange&page_size=10" -H "Authorization: Bearer $OWNER_TOKEN" \
     | jq '.records[] | {actor_id, outcome, project_id, details}'
   ```

   The agent keeps exchanged tokens, so many tool calls in one context give only one record per context, plus one for the NICE API.

8. **MCP Inspector (optional).** `npx @modelcontextprotocol/inspector`, Streamable HTTP transport to `http://127.0.0.1:8083/mcp`. NICE offers no Dynamic Client Registration, so enter the `client_id` of `agen-user` by hand and add `nice:prj:<project>` to the scope; the scopes of the metadata alone are not enough for a service client token. NICE without dev mode refuses `http` redirects of a public client, so the Inspector can sign in only to a local NICE running in dev mode. This step is not part of the integration run of this example.

## Limits

- **No DCR.** MCP clients are registered by hand (step 6).
- **No `api_key` services.** Those services serve Service Accounts only (see "Why only `nice_token` services"), so this example does not show a call to an `api_key` service.
- **Approvals only in clients of protocol `2026-07-28`.** An older client cannot run a sensitive operation (`approval_unavailable`).
- **Approvals per process.** The set of used nonces lives in memory: with several replicas, one approval can be used again within 5 minutes on another replica for the exact same call. A restart drops pending approvals.
- **No input schema per operation.** `request_fields` hold the name, location, and basic type only; the agent does not validate `body`, the target service checks it.
- **One project per call.** A question over many projects means one `call_operation` per project, each with one exchange; `/oauth/token` is rate limited per client (30 a minute by default).
- No chat UI or LLM, and no agent work while the user is away.
- **No access change feed in the agent.** A stale user token is still refused at the exchange (`invalid_grant` `token_stale`), because NICE checks the feed itself.
