# agent-backend (`agen`)

[Bahasa Indonesia](README.md)

An example AI agent backend: an MCP server (Streamable HTTP transport) for the NICE service `agen`, written in Go with the [official MCP Go SDK](https://github.com/modelcontextprotocol/go-sdk). Every tool calls another service **on behalf of the user** of the agent, with a token from token exchange (RFC 8693):

- the user's token goes only to the NICE token endpoint, never to another service (no *token passthrough*);
- no Service Account across organizations: the permissions of the target token are the user's, cut down to what `agen` declared for that dependency;
- the agent keeps no refresh token of the user;
- the target service sees the agent as actor (`act.sub`), and `inventaris` records it as `acted_by`.

```text
MCP client --Bearer user token (aud=agen)--> agen /mcp
agen --token exchange: subject_token + client agen-backend--> NICE /oauth/token
agen --Bearer exchanged token (aud=inventaris, act.sub=agen-backend)--> inventaris /items
agen --Bearer exchanged token (aud=pengiriman)--> NICE /pengiriman/shipments --X-Api-Key--> pengiriman
```

Minimum NICE version: a revision with `add-service-clients`, `add-delegated-nice-api`, `add-seamless-org-switch`, and `add-token-exchange` (`nice` `6cc9986` or later). The MCP SDK is pinned in `go.mod` at `v1.8.0`.

Tested against nice revision: `6cc9986` (2026-11-13)

## Folder structure

The layout follows `nice/internal/modules` (see `nice/AGENTS.md`), trimmed for one small service: one resource (`call`: calling another service on behalf of the user), no facade and no repository.

```text
cmd/agent-backend/main.go            thin: calls bootstrap.Run
docs/                                openapi.json (hand-written) + a small package that embeds it; OpenAPI and README tests
internal/
  domain/                            pure rules: slug, permission, targets, ID format (*_domain.go)
  app/call/
    usecase/                         ExecuteCall (execute_call_usecase.go), Service, the Sender contract (call_contract.go)
    dto/                             Caller, Where, Key, Request, Response, Failure (*_dto.go)
    internal/{key,failure}/          ID checks and mapping of errors to Failure (*_helper.go)
  adapter/
    mcp/                             HTTP routes and the nicetoken chain (routes.go), MCP server and tool schemas (tools_routes.go),
                                     one handler per tool (*_handler.go); internal/ holds caller, challenge, mapper
    niceexchange/                    wraps niceclient: token exchange and DoAs (sender_client.go)
  platform/config/                   env and .env
  bootstrap/                         assembly; integration tests with a fake NICE
  arch_test.go                       architecture test: file name endings and import direction
scripts/smoke.sh                     smoke test against a NICE that is set up
```

Why: `main.go` only assembles; the rules and the use case do not import `net/http`, the MCP SDK, `niceclient`, or `nicetoken` (the adapters do, enforced by `internal/arch_test.go`); every file carries its layer in the name ending (`_domain`, `_usecase`, `_contract`, `_dto`, `_handler`, `_routes`, `_client`, `_helper`, `_mapper`); one `*_usecase.go` holds one function named like its file. `openapi.json` stays hand-written and is embedded from `docs/`; `docs/openapi_test.go` keeps the permission in the document equal to the one the code enforces.

## Endpoints and tools

| Endpoint | Access |
|---|---|
| `POST /mcp` | NICE token with `aud=agen` and the permission `agen.mcp.use` |
| `GET /.well-known/oauth-protected-resource` (also `.../mcp`) | public: `resource` `<PUBLIC_URL>/mcp`, `authorization_servers` the NICE issuer, `scopes_supported` the base scopes and `nice:aud:agen` |
| `GET /healthz`, `GET /openapi.json` | public, not in the OpenAPI document |

Without a token, `/mcp` answers `401` with `WWW-Authenticate: Bearer resource_metadata="<PUBLIC_URL>/.well-known/oauth-protected-resource"`. A token for another service gets `401` `wrong_audience`, a token without `agen.mcp.use` gets `403` `permission_denied`.

| Tool | Target | Dependency permission |
|---|---|---|
| `inventaris_list_items` | `GET <INVENTARIS_URL>/items` | `inventaris.items.read` |
| `inventaris_create_item` (`name`, `quantity`) | `POST <INVENTARIS_URL>/items` | `inventaris.items.create` |
| `pengiriman_list_shipments` | `GET <NICE_PROXY_URL>/pengiriman/shipments` | `pengiriman.shipments.read` |
| `pengiriman_create_shipment` (`destination`) | `POST <NICE_PROXY_URL>/pengiriman/shipments` | `pengiriman.shipments.create` |

Every tool takes optional `organization_id` (`org_...`) and `project_id` (`prj_...`). Without them, the exchange asks only for `nice:aud:<service>`, and NICE uses the organization and project of the user's token. Another organization needs its project. The agent never picks a context the user did not name, and an ID not shaped like `org_...` or `prj_...` is refused before it becomes a scope.

Sensitive operations have no tool on purpose: deleting an item (`DELETE /items/{id}`, sensitive in the `inventaris` catalog) and cancelling a shipment. Human approval of write operations is a policy of the agent application, outside this example.

### How a call works

`callAs` calls `niceclient.ExchangeClient.DoAs` with the user's token from the MCP request and the key `{Org, Project, Audience}`. `DoAs`:

- keeps a token per `(user, organization, project, service)` until 60 seconds before it expires, and never uses a token that outlives the user's token;
- runs one exchange per key for parallel calls, with no queue across keys, so `inventaris` and `pengiriman` are exchanged at the same time;
- exchanges again and resends the request once when the service answers `401` `token_stale` or `expired`.

The MCP transport is *stateless* with JSON answers: every `POST /mcp` stands alone, so `curl` can call a tool in one request and parallel calls of one client do not wait for each other.

### Errors

Failures come back as tool results marked as errors (`isError: true`) with the code, a short reason from NICE or the service when there is one, and an explanation. Tokens, secrets, and raw bodies never reach the result or the log.

| Code | Source | Meaning |
|---|---|---|
| `access_denied` | exchange | the user is not an active member of that organization, the organization is not active, or it does not entitle the target service or `agen` |
| `invalid_scope` | exchange | the service is not a dependency of `agen` or not active, or the project is not active in that organization or does not use the target service |
| `invalid_grant` (`subject_token_expiring`) | exchange | the user's token has less than 60 seconds left: refresh it in the MCP client |
| `invalid_grant` (`token_stale`, `subject_invalid`) | exchange | the user's access changed, the token is not a user token for `agen`, or the session ended: sign in again |
| `invalid_client`, `unauthorized_client` | exchange | the agent's client is refused: check `NICE_CLIENT_ID` and `NICE_CLIENT_SECRET` |
| `nice_unavailable` | exchange | NICE did not answer |
| `insufficient_scope` (`permission_denied`), `permission_denied` | service or proxy | the user does not hold the permission for that operation in that project |
| `service_account_required` | proxy | `pengiriman` uses mirrored keys (step 12 of its README), which accept only Service Accounts |
| `invalid_argument` | agent | `organization_id` or `project_id` is malformed, or another organization comes without `project_id` |

Example text of a tool result for an organization the user does not belong to:

```text
access_denied (organization is not usable): NICE refused a token for inventaris in that organization: the user is not an active member of it, the organization is not active, or it does not use inventaris or agen
```

## Prerequisites

- Directory layout: `replace github.com/nice-platform/nice => ../../nice` in `go.mod` works only when the `nice` repository sits next to `example/`.
- Go 1.27, `curl`, `jq`, and `openssl` (for `nice/scripts/dev-token.sh`).
- NICE at the minimum version above, by default at `http://localhost:8080`.
- `inventaris` running and set up through step 8 of [`../shared-service-oauth/README.en.md`](../shared-service-oauth/README.en.md), and `pengiriman` through step 11 of [`../shared-service-apikey/README.en.md`](../shared-service-apikey/README.en.md), **without step 12** (mirrored keys refuse user tokens). The test user holds `inventaris.operator` and `pengiriman.admin` in the same project.
- For the other-organization test: the test user is also a member of a second organization, with a project that has `inventaris` active and an `inventaris.operator` binding there.

## Configuration

Copy `.env.example` to `.env`. The agent reads `.env` from the directory it runs in; a variable set in the environment wins over the file.

| Variable | Default | Meaning |
|---|---|---|
| `NICE_ISSUER` | required | the NICE issuer, exactly as the `iss` claim of its tokens |
| `NICE_CLIENT_ID` | required | `client_id` of the confidential client `agen-backend` (step 8) |
| `NICE_CLIENT_SECRET` | required | the secret of that client. Sent to the NICE token endpoint only |
| `INVENTARIS_URL` | required | address of `inventaris`, e.g. `http://127.0.0.1:8081` |
| `NICE_PROXY_URL` | `NICE_ISSUER` | the NICE address whose `/pengiriman/...` is forwarded to `pengiriman` |
| `LISTEN_ADDR` | `127.0.0.1:8083` | listen address |
| `PUBLIC_URL` | `http://127.0.0.1:8083` | the address MCP clients use; the metadata `resource` is `<PUBLIC_URL>/mcp` |
| `JWKS_FILE` | `data/jwks.json` | copy of the JWKS on disk (`data/` is not committed) |

An empty required variable stops the start with a message that names it.

## Run and test

```bash
make run          # go run ./cmd/agent-backend on 127.0.0.1:8083
make test         # go test -race; needs no NICE (fake NICE, inventaris, and pengiriman proxy)
make smoke        # scripts/smoke.sh, needs a NICE set up with the checklist below
```

When `go` on `PATH` does not match `GOROOT`, call `make GO=/path/to/go test`.

The tests run the agent over in-process HTTP with the MCP client of the SDK, in front of a fake NICE that follows the token exchange rules. The tests that keep the pattern: the fake services fail when they ever receive a token with `aud=agen` (no passthrough), ten calls in one context give one exchange, and the exchanges for two services run at the same time.

## NICE setup checklist

Run from the `example/agent-backend` directory, in one shell session. Console steps name their page; the `curl` snippets are the equivalent. Passwords are read from the terminal and never written to a file.

```bash
export NICE=${NICE:-http://localhost:8080}   # same as NICE_ISSUER in .env; dev-token.sh reads it too

read -rp 'Platform Admin email: ' ADMIN_EMAIL; read -rsp 'Password: ' ADMIN_PASSWORD; echo
ADMIN_TOKEN=$(../../nice/scripts/dev-token.sh "$ADMIN_EMAIL" "$ADMIN_PASSWORD"); unset ADMIN_PASSWORD
read -rp 'Tenant organization owner email: ' OWNER_EMAIL; read -rsp 'Password: ' OWNER_PASSWORD; echo
OWNER_TOKEN=$(../../nice/scripts/dev-token.sh "$OWNER_EMAIL" "$OWNER_PASSWORD"); unset OWNER_PASSWORD

ORG_ID=$(curl -sS "$NICE/api/v1/me/organization" -H "Authorization: Bearer $OWNER_TOKEN" | jq -r .organization.id)
PROJECT_ID=prj_...   # the tenant project where inventaris and pengiriman are active
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

Console: the detail of service `agen`, catalog import. The document comes from the file, because the agent can run only after its client exists (step 8).

```bash
PREVIEW=$(jq -n --rawfile doc docs/openapi.json '{document: $doc}' | curl -sS "$NICE/api/v1/shared-services/agen/catalog-imports" -X POST \
  -H "Authorization: Bearer $ADMIN_TOKEN" -H 'Content-Type: application/json' -d @-)
jq '{added: [.diff.added[]? | {key, permission}], roles: [.diff.roles[]? | .key]}' <<<"$PREVIEW"
curl -sS "$NICE/api/v1/shared-services/agen/catalog-imports/$(jq -r .import_id <<<"$PREVIEW")/apply" -X POST \
  -H "Authorization: Bearer $ADMIN_TOKEN" -H 'Content-Type: application/json' -d "$(jq '{hash}' <<<"$PREVIEW")"
```

The preview shows one operation `mcp` (`agen.mcp.use`) and the roles `agen.admin` and `agen.user` (from `x-nice-roles`). There is no `agen.viewer`, because the document has no read operation.

### 4. Dependencies of `agen` on `inventaris` and `pengiriman`

Needs `nice.platform.catalog.manage`. The permissions of exchanged tokens are cut down to this list, so the agent can never delete an item even when the user can. The list is replaced as a whole.

```bash
curl -sS "$NICE/api/v1/shared-services/agen/dependencies" -X PUT \
  -H "Authorization: Bearer $ADMIN_TOKEN" -H 'Content-Type: application/json' \
  -d '{"dependencies":[
        {"service":"inventaris","permissions":["inventaris.items.read","inventaris.items.create"]},
        {"service":"pengiriman","permissions":["pengiriman.shipments.read","pengiriman.shipments.create"]}]}'
```

### 5. Activate the service

```bash
curl -sS "$NICE/api/v1/shared-services/agen/activate" -X POST -H "Authorization: Bearer $ADMIN_TOKEN" | jq '.shared_service | {slug, status}'
```

### 6. Project in the platform organization

A service client must belong to a project of the platform organization. Use an existing project, or create one (console: Platform Admin, platform organization, **Project**).

```bash
AGENT_PROJECT_ID=$(curl -sS "$NICE/api/v1/organizations/$PLATFORM_ORG_ID/projects" -X POST \
  -H "Authorization: Bearer $ADMIN_TOKEN" -H 'Content-Type: application/json' \
  -d '{"name":"Agen","slug":"agen"}' | jq -r .project.id)
```

### 7. Client `agen-user` (public)

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

### 8. Client `agen-backend` (confidential)

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

### 9. User organization: entitlement, activation, binding

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

### 10. Run the agent

```bash
make run
curl -sS http://127.0.0.1:8083/healthz
```

Leave it running in another terminal.

### 11. User token

The user token for the agent is requested through the client `agen-user` with `nice:aud:agen` and the project where the user holds `agen.user`. `nice:org` makes the sign-in land in that organization.

```bash
read -rp 'Test user email: ' USER_EMAIL; read -rsp 'Password: ' USER_PASSWORD; echo
USER_TOKEN=$(CLIENT_ID=$USER_CLIENT_ID REDIRECT=http://localhost:6274/oauth/callback \
  ../../nice/scripts/dev-token.sh "$USER_EMAIL" "$USER_PASSWORD" "" "openid nice:org:$ORG_ID nice:aud:agen nice:prj:$PROJECT_ID")
unset USER_PASSWORD
export AGENT_USER_TOKEN=$USER_TOKEN   # read by scripts/smoke.sh
```

### 12. Smoke

```bash
make smoke
```

The script reads `.env` and `AGENT_USER_TOKEN` from the environment, never prints a token or the secret, and checks: NICE answers, `/healthz` answers, `/mcp` without a token answers `401` with `resource_metadata`, the metadata names `NICE_ISSUER`, then, with the user token, `tools/list` holds the four tools and `inventaris_list_items` succeeds. Failures map to steps:

| Answer | Step likely missed |
|---|---|
| no `.env`, agent unreachable | 8, 10 |
| `/mcp` with the token answers `401` `wrong_audience` | 11: the token must be requested with `nice:aud:agen` through `agen-user` |
| `/mcp` answers `403` `permission_denied` | 9: the `agen.user` binding |
| a tool answers `invalid_client` | 8: `NICE_CLIENT_ID` or `NICE_CLIENT_SECRET` |
| a tool answers `invalid_scope` | 4 (dependencies), or `inventaris` is not active in the project of the token |
| a tool answers `invalid_grant` | 11: the user token is about to expire, stale, or its session ended; get a new one |

## Test checklist

With `USER_TOKEN` from step 11. The function `tool` calls one tool in one request:

```bash
tool() {   # tool <name> [arguments JSON]
  jq -nc --arg name "$1" --argjson args "${2:-"{}"}" \
    '{jsonrpc: "2.0", id: 1, method: "tools/call", params: {name: $name, arguments: $args}}' |
  curl -sS http://127.0.0.1:8083/mcp -H "Authorization: Bearer $USER_TOKEN" \
    -H 'Content-Type: application/json' -H 'Accept: application/json, text/event-stream' -d @- |
  jq '.result | {isError, structuredContent, text: .content[0].text}'
}
```

1. **Tool list.** Four tools, no delete or cancel:

   ```bash
   curl -sS http://127.0.0.1:8083/mcp -H "Authorization: Bearer $USER_TOKEN" -H 'Content-Type: application/json' \
     -H 'Accept: application/json, text/event-stream' -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}' | jq '[.result.tools[].name]'
   ```

2. **Item created by the agent.** `created_by` is the user and `acted_by` is the `client_id` of `agen-backend` (`NICE_CLIENT_ID`):

   ```bash
   tool inventaris_create_item '{"name":"Kabel UTP","quantity":3}'
   tool inventaris_list_items
   ```

3. **Shipments through the proxy.** `created_by` is the user; `pengiriman` does not see the actor:

   ```bash
   tool pengiriman_create_shipment '{"destination":"Gudang Timur, Surabaya"}'
   tool pengiriman_list_shipments
   ```

4. **Another organization of the user.** With a second organization (`ORG_B_ID`, project `PROJECT_B_ID`) that meets the prerequisites and entitles `agen`:

   ```bash
   tool inventaris_list_items "{\"organization_id\":\"$ORG_B_ID\",\"project_id\":\"$PROJECT_B_ID\"}"
   ```

   It succeeds, with the items of that organization.

5. **An organization that is not the user's.** The result is an `access_denied` error, and `inventaris` is not called:

   ```bash
   tool inventaris_list_items "{\"organization_id\":\"$OTHER_ORG_ID\",\"project_id\":\"$OTHER_PROJECT_ID\"}"
   ```

6. **NICE audit.** Every exchange is recorded as `token.exchange` with the user as actor and `details` `client_id`, `service`, and `project_id`. A successful exchange is recorded in the target organization. A refusal is recorded in the organization of the user's token, with `details.reason` (e.g. `not_member`, `invalid_scope`) and `details.target_organization_id`:

   ```bash
   curl -sS "$NICE/api/v1/audit/records?action=token.exchange&page_size=10" -H "Authorization: Bearer $OWNER_TOKEN" \
     | jq '.records[] | {actor_id, outcome, project_id, details}'
   ```

   The agent keeps exchanged tokens, so many tool calls in one context give only one record per service.

7. **MCP Inspector (optional).** `npx @modelcontextprotocol/inspector`, Streamable HTTP transport to `http://127.0.0.1:8083/mcp`. NICE offers no Dynamic Client Registration, so enter the `client_id` of `agen-user` by hand and add `nice:prj:<project>` to the scope; the scopes of the metadata alone are not enough for a service client token. This step is not part of the integration run of this example.

## Limits

- **No DCR.** MCP clients are registered by hand (step 7).
- **No actor in `pengiriman`.** The NICE proxy forwards only `X-Nice-Principal`. An actor header (`X-Nice-Actor`) could be a proposal for NICE.
- **No mirrored keys in `pengiriman`.** After step 12 of the `pengiriman` README, the proxy refuses user tokens with `service_account_required`.
- **No sensitive tools**, no chat UI or LLM, and no agent work while the user is away.
- **No access change feed in the agent.** A stale user token is still refused at the exchange (`invalid_grant` `token_stale`), because NICE checks the feed itself.
