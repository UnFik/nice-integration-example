# NICE integration examples

[Bahasa Indonesia](README.md)

Five runnable examples of the ways a system connects to NICE. Each example stands alone with its own README, `.env.example`, tests, and NICE setup checklist.

| Example | Use it when | Stack |
|---|---|---|
| [`sharedservice-oauth/`](sharedservice-oauth/README.en.md) | Your service can be changed to verify NICE tokens. Clients call it directly, and NICE is not on the request path (`auth_mode: nice_token`). Its UI talks to the service backend only: the backend signs in to NICE as a confidential service client, holds the tokens, and switches organization and project (backend-for-frontend). Items per organization and project, optional PostgreSQL. | `backend/`: Go + `nice/pkg/niceauth/verify` + `nice/pkg/niceauth/credentials`, port 8081; `web/`: Vite + React, port 5181 in development |
| [`shared-service-apikey/`](shared-service-apikey/README.en.md) | A legacy service that only knows its own API key. Clients call it through the NICE proxy `/<slug>/...` (`auth_mode: api_key`). | TypeScript + Hono, port 8082 |
| [`public-client-pkce/`](public-client-pkce/README.en.md) | A browser app owned by one organization (tenant client) that signs in with a NICE account and calls `nice_token` services on the user's behalf. | Vite + React + `oauth4webapi`, port 5180 |
| [`agent-backend/`](agent-backend/README.en.md) | An AI agent backend (MCP server) that runs tools against `nice_token` services on behalf of the user through token exchange, without passing the user's token on. It does not call `api_key` services: they use a key per Service Account and refuse user tokens. | Go + MCP Go SDK + `nice/pkg/niceauth/credentials`, port 8083 |
| [`service-account-client/`](service-account-client/README.en.md) | An integration with no user (a job, a daemon, a partner service) that holds the API key of a Service Account, trades it at NICE for a token, and calls a `nice_token` service directly. The key goes to NICE only; the token is reused and renewed. | Go + `nice/pkg/niceauth/credentials` (`KindServiceAccount`), a command-line program with no port |

```text
 SPA (public client) --PKCE--> NICE /oauth/*
 SPA --Bearer JWT (aud=nadi)--> sharedservice-oauth/backend      (direct, local verification)
 Service Account --Bearer JWT (aud=pengiriman)--> NICE /pengiriman/... --X-Api-Key--> shared-service-apikey
```

```text
 UI --session cookie--> sharedservice-oauth/backend --PKCE + client_secret, refresh nice:org nice:aud:nadi nice:prj--> NICE /oauth/token
 sharedservice-oauth/backend --sign-in token exchanged for nice:aud:nice--> NICE /api/v1/me/contexts
```

```text
 MCP client --Bearer JWT (aud=agen)--> agent-backend --token exchange--> NICE /oauth/token
 agent-backend --Bearer exchanged JWT (aud=nadi, act)--> sharedservice-oauth/backend
```

```text
 service-account-client --client_credentials: Service Account id + API key, nice:aud:nadi--> NICE /oauth/token
 service-account-client --Bearer JWT (aud=nadi)--> sharedservice-oauth/backend      (the key never reaches the service)
```

The SPA does not call the `api_key` service because CORS does not cover the NICE proxy path `/<slug>/...`.

The OAuth protocol code in the browser (`oidc.ts`, the token manager) lives only in `public-client-pkce`. The `sharedservice-oauth` UI has no protocol code: it is all in the backend, through `nice/pkg/niceauth/credentials`.

### Minimum NICE version

| Example | NICE version |
|---|---|
| `sharedservice-oauth` | `00256d3` (`rename-sdk-to-niceauth`: the Go SDK in `nice/pkg/niceauth`; it already includes `add-client-org-switch` and `add-signin-token-exchange`) |
| `shared-service-apikey` | `40ddb20` (key operations are required when registering an `api_key` service; mirrored keys reach the upstream through the proxy) |
| `public-client-pkce` | `c7a5086`; without `NICE_CORS_ALLOWED_ORIGINS` from `60939ed` (`add-delegated-nice-api`) |
| `agent-backend` | `00256d3` (`rename-sdk-to-niceauth`; it already includes `add-token-exchange` from `6cc9986`, which `acted_by` in `sharedservice-oauth` needs too) |
| `service-account-client` | `00256d3` (`rename-sdk-to-niceauth`: `nice/pkg/niceauth/credentials` and `Client.HTTPClient`) |

## Prerequisites

- Directory layout: `nice/` must sit next to `example/`. The Go examples use `nice/pkg/niceauth/verify` and `nice/pkg/niceauth/credentials` through `replace ../../nice` (`agent-backend`, `service-account-client`) and `replace ../../../nice` (`sharedservice-oauth/backend`).
- A dev NICE running on the host (`cd nice && make db && make migrate && make dev`, `http://localhost:8080`).
- Settings in `nice/.env` (restart NICE after changing them):

  | Variable | Value | For |
  |---|---|---|
  | `NICE_OUTBOUND_ALLOW_HTTP` | `true` | health probes and the proxy to `http://127.0.0.1` services |
  | `NICE_OUTBOUND_ALLOW_CIDRS` | `127.0.0.0/8` | same |
  | `NICE_CORS_ALLOWED_ORIGINS` | add `http://localhost:5180` | the SPA, only on a NICE before `60939ed`; from then on NICE allows the redirect URI origin of `public` clients |

- Go 1.27 (Go example), Node 22.18+ and npm (TypeScript examples), and `bash`, `curl`, and `jq` (smoke scripts).

Register service endpoints with `127.0.0.1`, not `localhost`, so NICE never tries `::1`.

For a NICE that runs in Docker (`make up` in `nice/`), `docker compose up -d --build` at the root of this repository runs `sharedservice-oauth` (the backend with the UI at `http://localhost:8081`) and its PostgreSQL; register the service as `http://inventaris:8081`. See the comments in `compose.yml`.

## Setup in NICE

Service registration, catalog import, entitlement, activation, bindings, OAuth clients, Service Accounts, and API keys are done by hand. Each example's README holds a numbered checklist with the console steps and equivalent `curl` snippets. When a check fails, `scripts/smoke.sh` in the service examples and in `agent-backend` names the step that was probably missed.

## NICE contracts followed

- [`nice/docs/token-verification.md`](../nice/docs/token-verification.md): token verification in a service, the access change feed, and sensitive operations.
- [`nice/docs/gateway-integration.md`](../nice/docs/gateway-integration.md): the service and client integration flow.
- [`nice/docs/api.md`](../nice/docs/api.md): sections "Token OIDC", "Service api_key dan proxy", "Kunci per Service Account", "Akses lintas origin", "Service client", "Token API NICE terdelegasi", "Pindah organisasi lewat refresh", "Konteks pemanggil", and "Token exchange: backend atas nama user".
