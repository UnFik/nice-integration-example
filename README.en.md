# NICE integration examples

[Bahasa Indonesia](README.md)

Four runnable examples of the ways a system connects to NICE. Each example stands alone with its own README, `.env.example`, tests, and NICE setup checklist.

| Example | Use it when | Stack |
|---|---|---|
| [`shared-service-oauth/`](shared-service-oauth/README.en.md) | Your service can be changed to verify NICE tokens. Clients call it directly, and NICE is not on the request path (`auth_mode: nice_token`). | Go + `nice/pkg/nicetoken`, port 8081 |
| [`shared-service-apikey/`](shared-service-apikey/README.en.md) | A legacy service that only knows its own API key. Clients call it through the NICE proxy `/<slug>/...` (`auth_mode: api_key`). | TypeScript + Hono, port 8082 |
| [`public-client-pkce/`](public-client-pkce/README.en.md) | A browser app owned by one organization (tenant client) that signs in with a NICE account and calls `nice_token` services on the user's behalf. | Vite + React + `oauth4webapi`, port 5180 |
| [`service-client-ui/`](service-client-ui/README.en.md) | A first-party service UI that serves many organizations (service client): contexts from `/api/v1/me/contexts`, organization and project switched by refresh. | Vite + React + `oauth4webapi`, port 5181 |

```text
 SPA (public client) --PKCE--> NICE /oauth/*
 SPA --Bearer JWT (aud=inventaris)--> shared-service-oauth      (direct, local verification)
 Service Account --Bearer JWT (aud=pengiriman)--> NICE /pengiriman/... --X-Api-Key--> shared-service-apikey
```

```text
 UI (service client) --PKCE, then refresh nice:aud:nice--> NICE /api/v1/me/contexts
 UI --refresh nice:org nice:aud:inventaris nice:prj--> NICE /oauth/token --> UI --Bearer JWT--> shared-service-oauth
```

The SPA does not call the `api_key` service because CORS does not cover the NICE proxy path `/<slug>/...`.

The SPA protocol code (`oidc.ts`, the token manager, the fake NICE for tests) is duplicated on purpose in `public-client-pkce` and `service-client-ui`, so each example can be copied on its own. A protocol fix in one of them must be applied to both.

### Minimum NICE version

| Example | NICE version |
|---|---|
| `shared-service-oauth` | `c7a5086` |
| `shared-service-apikey` | `2638605` (mirrored keys reach the upstream through the proxy) |
| `public-client-pkce` | `c7a5086`; without `NICE_CORS_ALLOWED_ORIGINS` from `60939ed` (`add-delegated-nice-api`) |
| `service-client-ui` | `8160ac4` (`add-service-clients`, `add-delegated-nice-api`, `add-seamless-org-switch`) |

## Prerequisites

- Directory layout: `nice/` must sit next to `example/`. The Go example uses `nice/pkg/nicetoken` through `replace ../../nice`.
- A dev NICE running on the host (`cd nice && make db && make migrate && make dev`, `http://localhost:8080`).
- Settings in `nice/.env` (restart NICE after changing them):

  | Variable | Value | For |
  |---|---|---|
  | `NICE_OUTBOUND_ALLOW_HTTP` | `true` | health probes and the proxy to `http://127.0.0.1` services |
  | `NICE_OUTBOUND_ALLOW_CIDRS` | `127.0.0.0/8` | same |
  | `NICE_CORS_ALLOWED_ORIGINS` | add `http://localhost:5180` | the SPA, only on a NICE before `60939ed`; from then on NICE allows the redirect URI origin of `public` clients |

- Go 1.27 (Go example), Node 22.18+ and npm (TypeScript examples), and `bash`, `curl`, and `jq` (smoke scripts).

Register service endpoints with `127.0.0.1`, not `localhost`, so NICE never tries `::1`.

## Setup in NICE

Service registration, catalog import, entitlement, activation, bindings, OAuth clients, Service Accounts, and API keys are done by hand. Each example's README holds a numbered checklist with the console steps and equivalent `curl` snippets. When a check fails, `scripts/smoke.sh` in both service examples names the step that was probably missed.

## NICE contracts followed

- [`nice/docs/token-verification.md`](../nice/docs/token-verification.md): token verification in a service, the access change feed, and sensitive operations.
- [`nice/docs/gateway-integration.md`](../nice/docs/gateway-integration.md): the service and client integration flow.
- [`nice/docs/api.md`](../nice/docs/api.md): sections "Token OIDC", "Service api_key dan proxy", "Kunci per Service Account", "Akses lintas origin", "Service client", "Token API NICE terdelegasi", "Pindah organisasi lewat refresh", and "Konteks pemanggil".
