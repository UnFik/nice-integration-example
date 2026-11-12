# public-client-pkce

[Bahasa Indonesia](README.md)

A React SPA that acts as a **tenant public OAuth client** of NICE. It signs in with authorization code + PKCE (`S256`), shows the user's identity, then calls one or more `nice_token` services (for example `inventaris` from `../shared-service-oauth`) with a **separate token per service**, obtained through the rotating refresh token.

Tested against nice revision: <fill later>

## Contents

| File | Contents |
| --- | --- |
| `src/scope.ts` | The base scope (`BASE_SCOPE`) used by the login and every refresh, and the scope per service |
| `src/oidc.ts` | Discovery, login request, callback, refresh, userinfo, revoke, and the end-session URL with `oauth4webapi` |
| `src/token-manager.ts` | The client token and tokens per `slug|project`: one refresh at a time, reuse, backoff, session end, one retry on a stale token. No React |
| `src/session.ts` | Glue: login, reload, identity through userinfo, service calls, session end, logout |
| `src/config.ts` | Validation of `VITE_*` |
| `src/ui/`, `src/App.tsx` | Pages. `App` checks `location.pathname === "/callback"`; no router |

## Prerequisites

- Node 22.18+ and npm.
- NICE dev running (`cd nice && make dev`, `http://localhost:8080`).
- The called service is registered, imported, enabled, entitled to the tenant organization, activated in the project, and the user who signs in has a role binding in that project. For `inventaris`, follow steps 1-8 of [`../shared-service-oauth/README.en.md`](../shared-service-oauth/README.en.md), then run the service (`make run`, port 8081).

## NICE setup checklist

Run from `example/public-client-pkce`. Console steps name the page; the `curl` snippets are the equivalent. The admin token comes from `nice/scripts/dev-token.sh` (if the account uses two-step verification, set `TOTP_SECRET`, see the script header). The password is read from the terminal and never written to a file.

```bash
export NICE=http://localhost:8080
read -rp 'Tenant organization owner email: ' OWNER_EMAIL; read -rsp 'Password: ' OWNER_PASSWORD; echo
OWNER_TOKEN=$(../../nice/scripts/dev-token.sh "$OWNER_EMAIL" "$OWNER_PASSWORD"); unset OWNER_PASSWORD

ORG_ID=$(curl -sS "$NICE/api/v1/me/organization" -H "Authorization: Bearer $OWNER_TOKEN" | jq -r .organization.id)
curl -sS "$NICE/api/v1/organizations/$ORG_ID/projects" -H "Authorization: Bearer $OWNER_TOKEN" | jq '.projects[] | {id, name}'
PROJECT_ID=prj_...   # tenant project where inventaris is active
```

### 1. NICE env

Add the SPA origin to `nice/.env`, then restart NICE. The list is comma separated, exact, with no trailing slash:

```text
NICE_CORS_ALLOWED_ORIGINS=http://localhost:5173,http://localhost:5180
```

Without it the browser refuses the answers of discovery, `/oauth/token`, `/oauth/userinfo`, and `/oauth/revoke`. `/oauth/authorize` and `/oauth/end_session` are full navigations, so they need no CORS.

### 2. Create a tenant `public` OAuth client

Use a project in a **tenant organization**, not the platform organization. A tenant client can only ask for service tokens for its own project; without `nice:prj` NICE uses the client's project.

Console: the tenant project, page **OAuth client**. Create a public client with:

- redirect URI `http://localhost:5180/callback`
- post-logout redirect URI `http://localhost:5180/`
- grants `authorization_code` and `refresh_token` (the default)

The admin needs `nice.project.oauthclients.manage` in that project (the organization owner has it).

```bash
curl -sS "$NICE/api/v1/projects/$PROJECT_ID/clients" -X POST \
  -H "Authorization: Bearer $OWNER_TOKEN" -H 'Content-Type: application/json' \
  -d '{"name":"public-client-pkce","client_type":"CLIENT_TYPE_PUBLIC",
       "redirect_uris":["http://localhost:5180/callback"],
       "post_logout_redirect_uris":["http://localhost:5180/"],
       "grant_types":["authorization_code","refresh_token"]}' | jq '.oauth_client | {id, client_id}'
```

Copy `client_id` (not `id`, which starts with `cli_`) to `VITE_NICE_CLIENT_ID`. A public client has no secret.

### 3. User rights

The user who signs in to the SPA must be a member of that tenant organization and have a role binding of the service in the project (console: page **IAM**; for `inventaris` see step 8 of the service README). Without a binding NICE still issues a token, but the service answers 403 `permission_denied`.

## SPA configuration

```bash
cp .env.example .env    # then fill in VITE_NICE_CLIENT_ID
```

| Variable | Required | Contents |
| --- | --- | --- |
| `VITE_NICE_ISSUER` | yes | Address of NICE. `http` is only accepted for `localhost`, `127.0.0.1`, and `[::1]`; any other host must use `https` |
| `VITE_NICE_CLIENT_ID` | yes | `client_id` from step 2 |
| `VITE_REDIRECT_URI` | no | Default `<origin>/callback`; must point at `/callback` on the SPA origin |
| `VITE_POST_LOGOUT_REDIRECT_URI` | no | Default `<origin>/` |
| `VITE_SERVICES` | yes | JSON array of `{slug, endpoint, path, project?}`. `path` is the `GET` path that is called; `project` (`prj_<uuid>`) is optional |

A missing or invalid setting shows an error page naming the variable, and login does not start. `VITE_*` variables are read when `npm run dev` starts; restart it after changing `.env`.

The service list comes from configuration on purpose, not from the NICE API: the NICE API (`/api/v1`) refuses tokens of non-system clients.

## Running and tests

```bash
npm install
npm run dev        # http://localhost:5180 (strictPort)
npm test           # Vitest, no NICE needed
npm run typecheck
npm run build
```

## How it works

- **Login.** Base scope `openid profile email offline_access`; `offline_access` is required for NICE to issue a refresh token. Random `state` and `nonce`, PKCE `S256`. The login transaction (`verifier`, `state`, `nonce`) is kept in `sessionStorage` and removed as soon as `/callback` opens, so a `state` can be used only once. The code and `state` leave the address bar (`history.replaceState`) before the exchange. `oauth4webapi` checks `state`, the ID token claims, and `nonce`. An answer `error=temporarily_unavailable` with `error_description=authentication_expired` (the login page was left open for more than 30 minutes) starts a new authorization request right away.
- **Base scope on every refresh.** On every refresh NICE replaces the grant scopes with the scopes asked for (`SetCurrentScopes`). The scopes asked for must be part of the grant scopes; only `nice:aud:*` and `nice:prj:*` may go beyond them, and anything else is refused with `invalid_scope`. So every refresh sends the base login scope plus `nice:aud`/`nice:prj`. If a refresh sent only `nice:aud:<slug>`, `openid`, `profile`, and `email` would drop out of the grant, and a token for userinfo could not be obtained again without a new login. The login and the token manager use one constant, `BASE_SCOPE` in `src/scope.ts`. Here this example differs from `nice/pkg/niceclient`, which sends only `nice:aud`/`nice:prj`. Do not copy that part into a client that still needs userinfo.
- **Token per service.** For each (slug, project) pair the SPA sends a `refresh_token` grant with the base scope plus `nice:aud:<slug>` and, when set, `nice:prj:<project>`. Tokens are kept in memory under `"<slug>|<project>"` and reused while `now < iat + 10 minutes + jitter(0-60 seconds)` and `now < exp - 60 seconds`. These reuse rules mirror `nice/pkg/niceclient`.
- **Rotation.** NICE rotates the refresh token on every use. The SPA runs one refresh at a time per tab (a promise queue), reads the latest refresh token right before sending, and saves the new refresh token to `sessionStorage` before checking the rest of the answer. Concurrent requests for the same pair share one result.
- **Errors.** `invalid_grant` or `invalid_client` from the token endpoint ends the session: every token is dropped and the "Sesi berakhir" screen shows. Network failures, 5xx, and 429 are temporary: an old token that has not expired keeps being used, and the next attempt waits 5, 10, 20, 40, then 60 seconds. Other errors (for example `invalid_scope`) show on that service entry only.
- **Stale token.** When a service answers 401 with `error_description` `token_stale` or `expired` (from the JSON body, or from `WWW-Authenticate` when the service exposes it), the SPA drops the token of that pair, gets a new one once, and repeats the request once. A second 401 is shown as it is.
- **Identity.** After login and after every reload, the SPA gets the client token (key `client` in the token manager, `aud` = `client_id`) and calls `/oauth/userinfo`. After login the client token is the access token of the code exchange. After a reload it is obtained through one refresh with the base scope only, in the same refresh queue as the service tokens and under the same reuse rules. NICE builds userinfo from the grant scopes of the token used: `profile` releases `name`, `preferred_username` (holding the email), and `picture`; `email` releases `email` and `email_verified`. The page shows the email from `email`; `preferred_username` appears only in the full userinfo answer. `nice_org_id` is not in userinfo, so it is taken from the client token claims. The identity is not kept in browser storage.
- **Logout.** The refresh token is revoked at `/oauth/revoke` (a failure does not stop the logout), every `nice-example.*` key is removed from `sessionStorage`, and the browser goes to `end_session_endpoint` with `id_token_hint`, `client_id`, and `post_logout_redirect_uri`.

### Storage

| Data | Where |
| --- | --- |
| Access tokens (client and per service) | memory only |
| Identity (userinfo answer and `nice_org_id`) | memory only; fetched again after a reload |
| `nice-example.refresh_token`, `nice-example.id_token` | `sessionStorage` |
| `nice-example.login` (`verifier`, `state`, `nonce`) | `sessionStorage`, only while signing in |

The SPA uses neither `localStorage` nor cookies. The claims shown are decoded in the browser and not verified by the SPA ("tidak diverifikasi oleh SPA"); the services verify their tokens themselves.

## CORS requirements of the called service

The browser calls the service directly with an `Authorization` header, so the service must answer the `OPTIONS` preflight from origin `http://localhost:5180` with `Access-Control-Allow-Origin`, `Access-Control-Allow-Methods` including `GET`, and `Access-Control-Allow-Headers` including `Authorization`. Exposing `WWW-Authenticate` through `Access-Control-Expose-Headers` is recommended but not required, because the SPA also reads `error_description` from the body. The `inventaris` example already does this (`CORS_ALLOWED_ORIGINS=http://localhost:5180` in its `.env`).

## Why this SPA does not call `api_key` services

`api_key` services are called through the NICE proxy at `<NICE>/<slug>/...`, and NICE CORS only covers `/api/v1/*`, `/.well-known/openid-configuration`, and `/oauth/token|keys|userinfo|revoke`. The browser refuses proxy answers for another origin, so `api_key` services can only be called from a backend.

## Limits

- **A duplicated tab ends both tabs.** Duplicating a tab copies `sessionStorage`, so two tabs hold the same refresh token. As soon as one tab uses a refresh token the other tab already rotated, NICE treats it as a replay and revokes the whole token family (audit `token.refresh_reuse`). That tab gets `invalid_grant` right away, and the other tab gets `invalid_grant` at its next refresh, so **both** tabs show "Sesi berakhir". Sharing one session across tabs is outside this example.
- The SPA does not verify the ID token signature; the token comes straight from the token endpoint, as OIDC allows for the code flow.
- Dev only: no production build, container, or TLS.

## Manual test checklist

Open DevTools (Network, and Application > Session Storage). Each step names the expected result.

| # | Requirement | Steps | Expected result |
| --- | --- | --- | --- |
| 1 | Configuration | Empty `VITE_NICE_CLIENT_ID`, restart `npm run dev`, open the SPA | Page "Konfigurasi belum benar" names `VITE_NICE_CLIENT_ID`; no request to NICE. Fill it in again and restart |
| 2 | Login | Press **Masuk**, sign in at NICE | Back on the SPA at `http://localhost:5180/` with no `code` or `state` in the URL. Session Storage holds only `nice-example.refresh_token` and `nice-example.id_token`. The authorize request carries `code_challenge_method=S256` and `scope=openid profile email offline_access`. One `GET /oauth/userinfo` with the access token of the code exchange, and no refresh |
| 3 | Identity | Look at the home page | The Identitas section shows `sub`, name, email (from `email`), and `nice_org_id` with a note that it comes from the client access token claims. `preferred_username` appears only in the full userinfo answer. The "Access token client" card shows `aud` holding the `client_id`. The note "tidak diverifikasi oleh SPA" is shown |
| 4 | State mismatch | Open `http://localhost:5180/callback?code=x&state=palsu` | "Login gagal" screen with a **Masuk lagi** button; no `POST /oauth/token` |
| 5 | Expired login | Press **Masuk**, leave the NICE login page open for more than 30 minutes, then sign in | The SPA goes straight to a new authorize request with no error screen |
| 6 | Call a service | Press **Panggil** on `inventaris` | One `POST /oauth/token` with `grant_type=refresh_token` and `scope=openid profile email offline_access nice:aud:inventaris`, then `GET http://127.0.0.1:8081/items` with `Authorization: Bearer ...`. The panel shows status 200 and the body. An `inventaris` token card appears with `aud=inventaris` |
| 7 | Token reuse | Press **Panggil** again within a minute | No new `POST /oauth/token` |
| 8 | Second service and two at once | Add a second entry, e.g. `inventaris` with `"project":"<PROJECT_ID>"`, restart, reload the SPA, press **Panggil semua** | After one base-scope refresh for the identity on reload, two `POST /oauth/token` one after the other (the second starts after the first finished), the `refresh_token` of the second request equals the one the first answered, both 200, no redirect to NICE |
| 9 | No rights | Add an entry whose slug is not active in the project, e.g. `{"slug":"belumada","endpoint":"http://127.0.0.1:8081","path":"/items"}`, then **Panggil** | That entry shows `invalid_scope`; the `inventaris` entry can still be called |
| 10 | Stale token | With the `inventaris` feed active (step 9 of the service README), call `inventaris`, change the user's binding on page **IAM**, wait a few seconds, call again | The first `GET /items` is answered 401 `token_stale`, then one `POST /oauth/token` and one repeated `GET /items`; the panel shows the result of the repeat (e.g. 200, or 403 `permission_denied` when the binding was removed) |
| 11 | Session ended | In Session Storage change `nice-example.refresh_token` to `broken`, then press **Panggil** on an entry without a token (or reload first) | `POST /oauth/token` is answered `invalid_grant`; "Sesi berakhir" screen with **Masuk lagi**; no `nice-example.*` key left |
| 12 | NICE unreachable | Call `inventaris`, wait 11 minutes, block `localhost:8080/oauth/token` in DevTools (Network request blocking) or stop NICE, press **Panggil** | `POST /oauth/token` fails, `GET /items` is still sent with the old token and answered 200, the session does not end. Pressing again within 5 seconds sends no `POST /oauth/token`. After unblocking, a call after the backoff gets a new token |
| 13 | Reload | After calling `inventaris` (step 6), press F5 | Still on the home page with no redirect to NICE. One `POST /oauth/token` with `grant_type=refresh_token` and `scope=openid profile email offline_access` (no `nice:aud`) answered 200, then one `GET /oauth/userinfo`; the Identitas section shows again with the email and `nice_org_id`. The "Access token client" card shows a new token with `aud` holding the `client_id`. Session Storage still holds only `nice-example.refresh_token` and `nice-example.id_token`. **Panggil** gets service tokens through the refresh token |
| 14 | Tab closed | Close the tab, open `http://localhost:5180` in a new tab | The **Masuk** page (signed out) |
| 15 | Logout | Press **Keluar** | `POST /oauth/revoke` with `token_type_hint=refresh_token`, then navigation to `/oauth/end_session?id_token_hint=...&post_logout_redirect_uri=http://localhost:5180/`; back on the **Masuk** page; no `nice-example.*` key |
| 16 | Logout when revoke fails | Block `localhost:8080/oauth/revoke` in DevTools, sign in, press **Keluar** | Same as step 15; the failed revoke does not hold the logout |
| 17 | Duplicated tab | After login, duplicate the tab; press **Panggil** in the new tab, go back to the old tab and reload; then in the new tab press **Panggil** on an entry without a token (or reload) | The old tab shows "Sesi berakhir" right away, because its identity refresh uses a refresh token that was already rotated; the new tab also shows "Sesi berakhir" at its next refresh. Both tabs end |
