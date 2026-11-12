# NICE integration examples

[Bahasa Indonesia](README.md)

Three runnable examples for the three ways a system connects to NICE. Each example stands alone with its own README, `.env.example`, tests, and NICE setup checklist.

| Example | Use it when | Stack |
|---|---|---|
| [`shared-service-oauth/`](shared-service-oauth/README.en.md) | Your service can be changed to verify NICE tokens. Clients call it directly, and NICE is not on the request path (`auth_mode: nice_token`). | Go + `nice/pkg/nicetoken`, port 8081 |
| [`shared-service-apikey/`](shared-service-apikey/README.en.md) | A legacy service that only knows its own API key. Clients call it through the NICE proxy `/<slug>/...` (`auth_mode: api_key`). | TypeScript + Hono, port 8082 |
| [`public-client-pkce/`](public-client-pkce/README.en.md) | A browser app that signs in with a NICE account and calls `nice_token` services on the user's behalf. | Vite + React + `oauth4webapi`, port 5180 |

```text
 SPA (public client) --PKCE--> NICE /oauth/*
 SPA --Bearer JWT (aud=inventaris)--> shared-service-oauth      (direct, local verification)
 Service Account --Bearer JWT (aud=pengiriman)--> NICE /pengiriman/... --X-Api-Key--> shared-service-apikey
```

The SPA does not call the `api_key` service because CORS does not cover the NICE proxy path `/<slug>/...`.

## Prerequisites

- Directory layout: `nice/` must sit next to `example/`. The Go example uses `nice/pkg/nicetoken` through `replace ../../nice`.
- A dev NICE running on the host (`cd nice && make db && make migrate && make dev`, `http://localhost:8080`).
- Settings in `nice/.env` (restart NICE after changing them):

  | Variable | Value | For |
  |---|---|---|
  | `NICE_OUTBOUND_ALLOW_HTTP` | `true` | health probes and the proxy to `http://127.0.0.1` services |
  | `NICE_OUTBOUND_ALLOW_CIDRS` | `127.0.0.0/8` | same |
  | `NICE_CORS_ALLOWED_ORIGINS` | add `http://localhost:5180` | the SPA |

- Go 1.27 (Go example), Node 22.18+ and npm (TypeScript examples), and `bash`, `curl`, and `jq` (smoke scripts).

Register service endpoints with `127.0.0.1`, not `localhost`, so NICE never tries `::1`.

## Setup in NICE

Service registration, catalog import, entitlement, activation, bindings, OAuth clients, Service Accounts, and API keys are done by hand. Each example's README holds a numbered checklist with the console steps and equivalent `curl` snippets. When a check fails, `scripts/smoke.sh` in both service examples names the step that was probably missed.

## NICE contracts followed

- [`nice/docs/token-verification.md`](../nice/docs/token-verification.md): token verification in a service, the access change feed, and sensitive operations.
- [`nice/docs/gateway-integration.md`](../nice/docs/gateway-integration.md): the service and client integration flow.
- [`nice/docs/api.md`](../nice/docs/api.md): sections "Token OIDC", "Service api_key dan proxy", "Kunci per Service Account", and "Akses lintas origin".
