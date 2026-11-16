# service-client-ui

[Bahasa Indonesia](README.md)

A React UI for the `inventaris` service (`../shared-service-oauth`) that signs in to NICE as a **service client**: a `public` OAuth client in a platform organization project, bound to `inventaris`. The UI serves the user in each of their organizations that uses `inventaris`:

- the organizations and projects to choose from are read from `GET /api/v1/me/contexts`, with a delegated NICE API token (`aud=nice`);
- switching organization and project takes one refresh (`nice:org`, `nice:aud`, `nice:prj`), with no redirect to NICE;
- when NICE does not provide that discovery, the UI shows a notice and a **Pilih konteks manual** (choose context manually) form with the organization ID and the project ID;
- the `inventaris` items of the chosen context can be added, changed, and deleted, with buttons that follow the `perms` claim of the token.

When to use this example and when to use [`../public-client-pkce`](../public-client-pkce/README.en.md): a tenant client owned by one organization and one project uses `public-client-pkce`. A first-party service UI that serves many organizations uses this example.

Minimum NICE version: a revision with the changes `add-service-clients`, `add-delegated-nice-api`, and `add-seamless-org-switch` (`nice` `8160ac4` or later).

Tested against nice revision: `8160ac4` (2026-11-13)

## Contents

| File | Contents |
| --- | --- |
| `src/scope.ts` | The base scope (`BASE_SCOPE`) and `scopeFor`: base scope + `nice:org` (only when the organization changes) + `nice:aud` + `nice:prj` |
| `src/oidc.ts` | Discovery, login, callback (including `access_denied`), refresh, userinfo, revoke, and end-session with `oauth4webapi` |
| `src/token-manager.ts` | Tokens per `(organization, service, project)`: the session organization from the latest token's claim, a single refresh queue, reuse, backoff, session end, one retry on a stale token. No React |
| `src/contexts.ts` | `GET /api/v1/me/contexts` with the delegated token, every page |
| `src/session.ts` | Glue: login, reload, identity, contexts (discovered and manual), context switch, items, logout |
| `src/items.ts` | Small client for `GET/POST /items` and `GET/PUT/DELETE /items/{id}` through `tokens.call`, input validation, and error sentences |
| `src/recent-contexts.ts` | Manual contexts that worked, ids only, in `sessionStorage` |
| `src/config.ts` | Validation of `VITE_*` |
| `src/ui/`, `src/App.tsx` | Pages: top bar with the active context, the Konteks panel, the items table (cards below 640px), add/edit/delete dialogs, token status. `App` checks `location.pathname === "/callback"`, no router |

The protocol code (`oidc.ts`, the token manager, the fake NICE in `src/test/`) is copied from `public-client-pkce` and adapted, so this example can be copied on its own. A protocol fix in one of them must be applied to both.

## Prerequisites

- Node 22.18+ and npm.
- A running NICE dev (`cd nice && make dev`, by default `http://localhost:8080`). For NICE at another address, use that address in `NICE` below and in `VITE_NICE_ISSUER`.
- `inventaris` registered, imported, and active: steps 1-5 of [`../shared-service-oauth/README.en.md`](../shared-service-oauth/README.en.md). Run the service (`make run`, port 8081) with `CORS_ALLOWED_ORIGINS` listing `http://localhost:5181` (its `.env.example` already does).
- One Platform Admin account and one test user account that is a member of **two tenant organizations** (A and B).

## NICE setup checklist

Run from the `example/service-client-ui` directory. Tokens come from `nice/scripts/dev-token.sh` (if the account uses two-step verification, set `TOTP_SECRET`, see the script header). Passwords are read from the terminal and never written to a file.

```bash
export NICE=${NICE:-http://localhost:8080}   # same as VITE_NICE_ISSUER; dev-token.sh reads it too
read -rp 'Platform Admin email: ' ADMIN_EMAIL; read -rsp 'Password: ' ADMIN_PASSWORD; echo
ADMIN_TOKEN=$(../../nice/scripts/dev-token.sh "$ADMIN_EMAIL" "$ADMIN_PASSWORD"); unset ADMIN_PASSWORD
PLATFORM_ORG_ID=$(curl -sS "$NICE/api/v1/me/organization" -H "Authorization: Bearer $ADMIN_TOKEN" | jq -r .organization.id)
```

### 1. A project in the platform organization

A service client must belong to a project of the **platform organization**. A client of a tenant organization is refused when bound (`400 service_client_platform_only`). Use an existing project, or create one (console: Platform Admin, platform organization, **Project**):

```bash
UI_PROJECT_ID=$(curl -sS "$NICE/api/v1/organizations/$PLATFORM_ORG_ID/projects" -X POST \
  -H "Authorization: Bearer $ADMIN_TOKEN" -H 'Content-Type: application/json' \
  -d '{"name":"Inventaris UI","slug":"inventaris-ui"}' | jq -r .project.id)
```

### 2. `public` OAuth client

Console: that project, **OAuth client** page. Create a public client with redirect URI `http://localhost:5181/callback`, post-logout redirect URI `http://localhost:5181/`, and the grants `authorization_code` and `refresh_token`.

```bash
CLIENT=$(curl -sS "$NICE/api/v1/projects/$UI_PROJECT_ID/clients" -X POST \
  -H "Authorization: Bearer $ADMIN_TOKEN" -H 'Content-Type: application/json' \
  -d '{"name":"service-client-ui","client_type":"CLIENT_TYPE_PUBLIC",
       "redirect_uris":["http://localhost:5181/callback"],
       "post_logout_redirect_uris":["http://localhost:5181/"],
       "grant_types":["authorization_code","refresh_token"]}')
OAUTH_CLIENT_ID=$(jq -r .oauth_client.id <<<"$CLIENT"); jq -r .oauth_client.client_id <<<"$CLIENT"
```

Copy `client_id` to `VITE_NICE_CLIENT_ID`. A public client has no secret.

### 3. Bind the client to `inventaris`

Needs `nice.platform.catalog.manage`.

```bash
curl -sS "$NICE/api/v1/projects/$UI_PROJECT_ID/clients/$OAUTH_CLIENT_ID/service" -X PUT \
  -H "Authorization: Bearer $ADMIN_TOKEN" -H 'Content-Type: application/json' -d '{"service":"inventaris"}' \
  | jq '.oauth_client | {client_class, service}'
# {"client_class":"service","service":"inventaris"}
```

### 4. Dependency of `inventaris` on `nice`

Needs `nice.platform.catalog.manage`. The declaration opens the delegated NICE API token to that client, with `nice.me.contexts.read` as its ceiling. The list is replaced as a whole; if `inventaris` already declares other dependencies, include them as well.

```bash
curl -sS "$NICE/api/v1/shared-services/inventaris/dependencies" -X PUT \
  -H "Authorization: Bearer $ADMIN_TOKEN" -H 'Content-Type: application/json' \
  -d '{"dependencies":[{"service":"nice","permissions":["nice.me.contexts.read"]}]}'
```

### 5. Two organizations that use `inventaris`

For organizations A and B, run steps 6-8 of the `shared-service-oauth` README: the `inventaris` entitlement, the activation in the projects, and a binding of the test user to `inventaris.operator` in each of those projects. The test checklist uses two projects in A (P and Q) and one in B (R). Without a binding, the project does not appear in `/me/contexts`.

Optional, to see an organization marked unavailable: an organization C of the test user that is entitled to `inventaris` while the user has no `inventaris` binding in any of its projects.

### 6. NICE CORS

Nothing changes in `nice/.env`. NICE allows the redirect URI origin of every `public` OAuth client (`http://localhost:5181`) within 60 seconds of the client being created. `NICE_CORS_ALLOWED_ORIGINS` does not need to list this origin.

## Configuration

```bash
cp .env.example .env    # then fill VITE_NICE_CLIENT_ID
```

| Variable | Required | Value |
| --- | --- | --- |
| `VITE_NICE_ISSUER` | yes | Address of NICE, exactly its `NICE_ISSUER`. `http` is only accepted for `localhost`, `127.0.0.1`, and `[::1]` |
| `VITE_NICE_CLIENT_ID` | yes | `client_id` from step 2 |
| `VITE_REDIRECT_URI` | no | Default `<origin>/callback`; must point at `/callback` on the UI origin |
| `VITE_POST_LOGOUT_REDIRECT_URI` | no | Default `<origin>/` |
| `VITE_INVENTARIS_URL` | yes | Address of `inventaris`, e.g. `http://127.0.0.1:8081`; the UI calls `GET <address>/items` |

A missing or invalid configuration shows an error page that names the variable, and no login starts. Restart `npm run dev` after changing `.env`.

## Running and tests

```bash
npm install
npm run dev        # http://localhost:5181 (strictPort)
npm test           # Vitest with a fake NICE and a fake inventaris, no running NICE
npm run typecheck
npm run build
```

## How it works

- **Login.** Authorization code + PKCE `S256`, `state`, and `nonce`, with the base scope `openid profile email offline_access` only, no `nice:org`. NICE completes the login in the user's first organization (by join time) that is entitled to `inventaris`. If there is none, NICE answers `access_denied` to `/callback`. The UI explains that none of the user's organizations uses this service and offers **Masuk dengan akun lain** (sign in with another account, `prompt=select_account`).
- **Base scope on every refresh.** NICE replaces the grant scopes with the requested scopes on every refresh. Only `nice:org`, `nice:aud`, and `nice:prj` may go beyond the grant scopes. So every refresh sends the base scope, and the client token for userinfo can still be obtained.
- **Session organization.** The refresh token is bound to one organization. A refresh without `nice:org` stays in it, and a refresh with `nice:org:<org>` moves it. The UI reads the session organization from the `nice_org_id` claim of the latest issued token, and adds `nice:org` only when the target organization differs. The decision is made inside the refresh queue, right before the request is sent, because an earlier refresh in the queue may already have moved the session.
- **Contexts.** After login and every reload: the client token (`aud` = `client_id`) for `/oauth/userinfo`, then the delegated token through a `nice:aud:nice` refresh (no `nice:prj`), then `GET /api/v1/me/contexts?page_size=50`, following `page.next_page_token`. The `service` query is not sent, because NICE answers it with `400 invalid_argument` for a delegated token; the service comes from the `nice_service` claim. The chooser holds only the projects of that answer. An organization with empty `projects` is shown as unavailable.
- **Manual context.** When reading `/me/contexts` fails (NICE now refuses the `nice:aud:nice` refresh for service clients with `invalid_scope`), the Konteks panel explains it in one neutral sentence and shows a form: the organization ID (prefilled from the `nice_org_id` claim of the session) and the project ID (starting with `prj_`). **Gunakan konteks ini** (use this context) runs the same context-switch logic. Those that work go into the **Terakhir dipakai** (last used) list. An `access_denied`/`invalid_scope` refusal is shown kindly, and the session and previous context stay. The chooser from `/me/contexts` is still used when NICE answers it.
- **Switching context.** Choosing a project runs one refresh: the base scope, `nice:org:<org>` when the organization changes, `nice:aud:inventaris`, and `nice:prj:<project>` (required for a service client). The page does not change. When NICE refuses (`access_denied` for an organization that may not be used, `invalid_scope` for a project that no longer has `inventaris` active), the refresh token is not rotated. The UI stays on the previous context, shows the reason, and reloads `/me/contexts`.
- **Tokens per context.** Key `"<org>|<service>|<project>"`. A token is reused while `now < iat + 10 minutes + jitter(0-60 seconds)` and `now < exp - 60 seconds`, so going back to the previous context within 10 minutes sends no token request. The session stays in the last organization until the next refresh to another one.
- **Errors and rotation** follow `public-client-pkce`: one refresh at a time with the latest refresh token, `invalid_grant`/`invalid_client` end the session, network/5xx/429 keep the old token with a 5-60 second backoff, and a 401 `token_stale`/`expired` from the service is retried once with a new token.

### Storage

| Data | Where |
| --- | --- |
| Access tokens (client, delegated NICE, per `inventaris` context) | memory only |
| Identity, `/me/contexts` answer, active context | memory only; fetched again after a reload |
| `nice-service-ui.refresh_token`, `nice-service-ui.id_token` | `sessionStorage` |
| `nice-service-ui.login` (`verifier`, `state`, `nonce`) | `sessionStorage`, only during login |
| `nice-service-ui.recent_contexts` (list of organization and project ids, no tokens) | `sessionStorage`, gone with the session |

After a reload the UI returns to the organization of the refresh token with no project chosen. The claims shown are decoded in the browser and not verified by the UI.

## Limits

- The delegated token (`aud=nice`) of this example only carries `nice.me.contexts.read`. Against a NICE that refuses it for service clients (the `nice:aud:nice` refresh is answered `invalid_scope`), context discovery fails and the UI falls back to manual contexts.
- Editing and deleting items use the `inventaris` token of the active context. Buttons are disabled when `perms` lacks the permission; the final decision stays with `inventaris`.
- Only `inventaris` is called, although the `services` of each project are shown.
- A duplicated tab shares the same refresh token, and reusing a rotated refresh token ends the session in both tabs (see `public-client-pkce`).
- Dev only: no production build, container, or TLS.

## Manual test checklist

Steps 3-10 need a NICE that still answers context discovery for service clients. Against a NICE that refuses it, run steps 1, 2, and 11-14: the context is chosen with the manual form.

Open DevTools (Network tab with **Preserve log**, and Application > Session Storage). A and B are the organizations from step 5, P and Q the `inventaris` projects in A, R the `inventaris` project in B.

| # | Requirement | Step | Expected result |
| --- | --- | --- | --- |
| 1 | Sign in as a service client | Press **Masuk**, sign in as the test user | The authorize request carries `code_challenge_method=S256` and `scope=openid profile email offline_access` without `nice:org`. Back at `http://localhost:5181/` without `code` in the URL. `nice_org_id` under Identitas is the user's first organization (by join time) entitled to `inventaris`; an earlier organization without the entitlement is skipped. The card of that organization is marked **Organisasi saat ini** |
| 2 | Sign in without an organization | Sign in with an account that has no organization entitled to `inventaris` | The **Service tidak tersedia** screen explains that none of the user's organizations uses this service. **Masuk dengan akun lain** opens authorize with `prompt=select_account` |
| 3 | Contexts from NICE | Look at the Konteks section after step 1 | One `POST /oauth/token` with `scope=openid profile email offline_access nice:aud:nice`, then `GET /api/v1/me/contexts?page_size=50` without `service`, answered 200. The chooser holds exactly the organizations and projects of that answer (compare with the Response tab), with the `services` of each project. The token card "Token API NICE terdelegasi" shows `perms` `["nice.me.contexts.read"]` and `nice_service` `inventaris` |
| 4 | Organization without projects | With organization C from step 5 | C shows "Tidak tersedia" with the reason that the user holds no `inventaris` access in any of its projects, and no button |
| 5 | Switch project | Choose P in A | One `POST /oauth/token` with `scope=... nice:aud:inventaris nice:prj:<P>` (no `nice:org` when the session is already in A), then `GET http://127.0.0.1:8081/items`. The P button is marked active and the items of A are listed |
| 6 | Switch organization | From A/P, choose R in B | Exactly one `POST /oauth/token` with `scope=openid profile email offline_access nice:org:<B> nice:aud:inventaris nice:prj:<R>`, answered 200 with a new `refresh_token`. No document request to NICE (the browser does not leave the page). The items of B are listed |
| 7 | Back to the previous context | Within 10 minutes, choose P in A again | No `POST /oauth/token`; `GET /items` uses the same A/P token as step 5 |
| 8 | Reload | After step 7, press F5 | Still signed in, no redirect. The first refresh carries the base scope only and answers a token with `nice_org_id` B, the organization of the latest refresh token (step 6), although the last context on screen was A/P. Then `GET /oauth/userinfo` and `/me/contexts`; B is marked **Organisasi saat ini**. Session Storage holds only `nice-service-ui.refresh_token` and `nice-service-ui.id_token` |
| 9 | Access removed in between | After the chooser shows, deactivate `inventaris` in Q (console: project Q, **Product & Activation**), then choose Q. Q has no token yet, so the UI has to ask NICE | One `POST /oauth/token` with `nice:org:<A> nice:aud:inventaris nice:prj:<Q>` answers 400 `invalid_scope`. The UI stays on the previous context, shows the reason, and calls `GET /api/v1/me/contexts` again; Q is no longer offered. The refresh token in Session Storage is the one from before the click (not rotated). Activate Q again afterwards |
| 10 | Organization removed | Choose P in A (the session moves to A), revoke the `inventaris` entitlement of organization B, then choose R from the old chooser. After the reload of step 8, R has no token yet | `POST /oauth/token` with `nice:org:<B>` answers 400 `access_denied` (`the organization is not entitled to the service of this client`), the presented refresh token stays usable, the UI stays on A/P, and B leaves the chooser after the contexts reload. Afterwards grant B the entitlement again, then deactivate and activate `inventaris` in R again: until then `/me/contexts` lists B without projects |
| 11 | Session ended | In Session Storage, change `nice-service-ui.refresh_token` to `rusak`, then choose a context that has no token yet | `POST /oauth/token` answers `invalid_grant`; the **Sesi berakhir** screen; no `nice-service-ui.*` key is left |
| 12 | Logout | Press **Keluar** | `POST /oauth/revoke`, then navigation to `/oauth/end_session` with `post_logout_redirect_uri=http://localhost:5181/`; back on the **Masuk** page |
| 13 | Manual context | Against a NICE that refuses `nice:aud:nice` for service clients, sign in and look at the Konteks panel. Enter the ID of project P and press **Gunakan konteks ini**. Then enter a project ID in an organization the user may not use | The panel shows the notice that NICE no longer provides org/project discovery for public SPAs and the **Pilih konteks manual** form with the organization ID prefilled from `nice_org_id`. A valid choice runs one `POST /oauth/token` with `nice:aud:inventaris nice:prj:<P>`, the items show, and the context joins **Terakhir dipakai** (Session Storage only gains `nice-service-ui.recent_contexts` holding ids). A refused choice shows a kind reason, the previous context stays, and the refresh token does not change |
| 14 | Manage items | With an active context, press **Tambah barang**, fill in, save. Then **Ubah**, then **Hapus** and confirm. Repeat with a user whose `perms` only holds `inventaris.items.read` | `POST /items`, `PUT /items/<id>`, `DELETE /items/<id>` (204) are answered with success and each shows a notice and reloads the list. The dialog closes with Escape and focus returns to its opener. For the read-only user, Tambah, Ubah, and Hapus are disabled with a note naming the missing permission |
