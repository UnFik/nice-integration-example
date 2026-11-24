# sharedservice-oauth (`nadi`)

[Bahasa Indonesia](README.md)

A fullstack SharedService example for NICE with the slug `nadi` (NADI: IoT device CRUD). The Go backend is a `nice_token` service that verifies NICE tokens itself, and it is also the backend-for-frontend (BFF) of its UI: it signs in to NICE as a **confidential** OAuth client bound to `nadi`, keeps the refresh tokens on the server, and gives the browser a session cookie only. The React UI talks only to the backend on the same origin: it holds no token, never calls NICE, and loads no OAuth library.

Tested against nice revision: SDK `v0.2.1` and NICE `dev` `d42b398` (2026-11-24): the unit tests, and by hand the sign-in through the UI, the context picker from `ExchangeClient.Contexts` with organizations reached as a platform operator, and a switch to a project of another organization. SDK `v0.2.0` (2026-11-24): the unit tests, and the `perms_ref` path tested by hand (see [`backend/README.en.md`](backend/README.en.md)). SDK `v0.1.0` and a local NICE server in Docker (the `nice:dev` image from `c7a3e33`), 2026-11-19: the `nadi` image was rebuilt without the `nice` context, and the unit tests and `backend/scripts/smoke.sh` pass. The UI flow below was not tested again at this revision. Against `4282316` (the NICE dev stack was built from it): login, callback, context discovery, org and project switching, data isolation per org and project (memory and PostgreSQL), CSRF, sign-out, and `scripts/smoke.sh` pass over HTTP. The browser click-through has not been tested by hand yet.

Minimum NICE version: `4282316` (`add-client-org-switch` and `add-signin-token-exchange`). SDK: `github.com/ABA-Developer/nice-auth-sdk` `v0.2.1`.

## Contents

| Folder | Contents |
|---|---|
| [`backend/`](backend/README.en.md) | The `nadi` service (Go, `nice-auth-sdk/verify`) for Bearer callers, plus `/auth/*` for the UI (`nice-auth-sdk/credentials`, `KindBFF`), the server of `web/dist`, and the device store per organization and project (memory or PostgreSQL). Its README has the operations, the service setup checklist and the smoke test |
| [`web/`](web/README.en.md) | The React 19 + Vite UI: sign-in, context picker and device table. It calls only `/auth/*` and `/devices*`. Its `Dockerfile` builds an nginx image that serves the build and passes `/auth` and `/devices` to the backend |

## How it works

```text
 browser --GET /auth/login--> backend --302--> NICE /oauth/authorize (PKCE S256, state bound to the session)
 NICE --302 /auth/callback?code--> backend --code + client_secret--> NICE /oauth/token
 browser --cookie nadi_session--> backend /auth/session, /auth/contexts, /auth/context, /auth/logout
 browser --cookie + X-CSRF-Token--> backend /devices --Bearer token of the active context--> niceauth/verify chain --> device handler
 agent-backend, Service Account --Bearer--> backend /devices   (direct path, the cookie is not used)
```

- **Sign-in.** `GET /auth/login` creates a pending session (a cookie that lives 10 minutes), then redirects to NICE with the scope `openid profile email offline_access`. `GET /auth/callback` matches `state` with the session of that browser, exchanges the `code` with the `client_secret`, then creates a new session with a new ID. A callback whose `state` is unknown, already used, or belongs to another browser is refused without keeping any token. A failed sign-in (`access_denied`, `state_mismatch`, `temporarily_unavailable`) is sent to the UI as `/?login_error=<code>`; the URL never carries a token.
- **Session.** The `nadi_session` cookie holds a random 32-byte ID (`HttpOnly`, `SameSite=Lax`, `Path=/`, `Secure` when `PUBLIC_URL` is `https`). The session record on the server keeps another random `niceauth/credentials` subject, the identity from `/oauth/userinfo`, the active context `{org, project}`, and a CSRF token. Refresh tokens are kept by `credentials.MemoryStore` per subject, so the cookie value never keys a token. A session ends after `SESSION_IDLE_TIMEOUT` (default 8 hours) without use, and the backend revokes its refresh token.
- **Contexts.** After sign-in and on every `GET /auth/contexts`, the backend takes the sign-in token (`Client.SignInToken`, a refresh with the base scopes only), exchanges it for a delegated NICE API token (`ExchangeClient`, `nice:aud:nice`), then reads every page of `GET /api/v1/me/contexts`. After sign-in, the first project of the current organization is picked at once. The session keeps IDs only; organization and project names are always read from NICE.
- **Switching context.** `POST /auth/context {org, project}` asks for a token with `Key{Org, Audience: "nadi", Project}`: one refresh with `nice:org:<org> nice:aud:nadi nice:prj:<project>` plus the base scopes. The context of the session changes only when NICE issued a token. A refusal (`access_denied`, `invalid_scope`) is answered `409 context_refused`, the context stays, and the refresh token is unchanged. Two requests of one session at once use the refresh token one at a time.
- **Device requests of the UI.** For a request with the cookie and no `Authorization`, the session middleware puts the token of the active context as `Authorization: Bearer`, then the same `niceauth/verify` chain as for Bearer callers runs: verification, permission, tenant from `nice_org_id`, feed, and the online check. The `niceauth/credentials` cache does not know the feed. So when the feed has marked the token of the active context stale (for example after the user's role changed), the middleware drops that token and asks NICE for a new one once before passing the request on; the `perms` of `GET /auth/session` take the same path. If the new token is stale too, the request is answered `401 token_stale` without asking again. A request that carries `Authorization` is a Bearer caller and the cookie is ignored; a Bearer caller renews its own token.
- **CSRF.** A write with the cookie (`POST`, `PUT`, `DELETE`, `/auth/context` and `/auth/logout` included) must carry `X-CSRF-Token` from `GET /auth/session` and an `Origin` equal to `PUBLIC_URL`; otherwise `403 {"error":"csrf"}`.
- **Sign-out.** `POST /auth/logout` revokes the refresh token at NICE (`/oauth/revoke`), removes the session and clears the cookie. When NICE does not answer, the answer is `503` and the session stays, so the user does not look signed out while the token still works. The NICE SSO session is not ended.
- **Device data.** A device belongs to one organization and one project: the scope key `{OrgID, ProjectID, Subject, Actor}` comes from the claims of the verified token (`nice_org_id`, `prj`, `sub`, `act`), never from the request. A token without `prj` is answered `403 project_required`. See [`backend/README.en.md`](backend/README.en.md#data-per-organization-and-project).

### `/auth/*` answers

| Operation | Answer |
|---|---|
| `GET /auth/login[?prompt=select_account]` | `302` to NICE |
| `GET /auth/callback` | `302` to `PUBLIC_URL/`, or `PUBLIC_URL/?login_error=<code>` |
| `GET /auth/session` | `200 {"user":{"id","name","email"},"context":{"org","project"}\|null,"perms":[...],"csrf_token":"...","context_error":""}`; `401 unauthenticated` or `401 session_ended` |
| `GET /auth/contexts` | `200 {"current_organization_id","organizations":[{"id","name","slug","projects":[{"id","name","slug"}],"projects_truncated"}]}`; `502 contexts_unavailable` (the description holds the code of NICE); `503` |
| `POST /auth/context` | `200` as `/auth/session`; `400 invalid_argument`; `403 csrf`; `409 context_refused`; `503` |
| `POST /auth/logout` | `204`; `403 csrf`; `503 temporarily_unavailable` (the session stays) |

Device requests with the cookie may also be answered `401 unauthenticated`/`session_ended`, `409 context_required` (no context yet), or `409 context_refused`. The `/auth/*` operations are not in the OpenAPI document, because they are not operations of the NICE catalog.

## Configuration

All configuration is in `backend/.env` (copy it from `backend/.env.example`). The UI has none; its Docker image reads only `BACKEND_ADDR`, the address of the backend (default `nadi:8081`).

| Variable | Default | Meaning |
|---|---|---|
| `NICE_ISSUER` | required | the NICE issuer, exactly the `iss` claim |
| `NICE_CLIENT_ID` | required | `client_id` of the confidential OAuth client bound to `nadi` |
| `NICE_CLIENT_SECRET` | required | the secret of that client. Sent to NICE only; never printed |
| `PUBLIC_URL` | `http://localhost:8081` | the origin the browser sees. Development with Vite, and Docker: `http://localhost:5181`. The redirect URI is derived as `PUBLIC_URL/auth/callback` |
| `SESSION_IDLE_TIMEOUT` | `8h` | a session not used for this long ends |
| `WEB_DIR` | `../web/dist` | the output of `npm run build`, served at `PUBLIC_URL`; without that directory the backend serves no UI. Docker leaves it empty, since the UI has a container of its own |
| `DATABASE_URL` | empty | PostgreSQL for the devices; empty keeps them in memory |
| `LISTEN_ADDR`, `JWKS_FILE`, `CORS_ALLOWED_ORIGINS`, `NICE_SERVICE_API_KEY`, `SMOKE_SA_ID`, `SMOKE_API_KEY` | | see [`backend/README.en.md`](backend/README.en.md#configuration) |

Without `NICE_ISSUER`, `NICE_CLIENT_ID`, or `NICE_CLIENT_SECRET`, the backend stops at start with a message that names the variable.

## Running

Development (UI at `http://localhost:5181`, `PUBLIC_URL=http://localhost:5181` in `backend/.env`):

```bash
cd web && npm install && cd ..  # install dependencies once
make run                        # backend on 8081 and web on 5181
```

Press Ctrl+C to stop both. Vite forwards `/auth` and `/devices` to the backend.

Build, one process (UI at `http://localhost:8081`, `PUBLIC_URL=http://localhost:8081`):

```bash
cd web && npm run build         # web/dist
cd backend && make run          # serves web/dist on 8081
```

Docker: `make up` in this folder (same as `docker compose up -d --build` at the repository root) runs the backend, the UI, and PostgreSQL for `DATABASE_URL` in separate containers (see `compose.yml`):

- the backend (`backend/Dockerfile`) at `http://localhost:8081` for Bearer callers, with no UI;
- the UI (`web/Dockerfile`, nginx) at `http://localhost:5181`, which passes `/auth` and `/devices` to the backend as the Vite proxy does, so the browser still sees one origin.

`backend/.env` is still needed for `NICE_ISSUER`, `NICE_CLIENT_ID`, and `NICE_CLIENT_SECRET`; `compose.yml` sets `PUBLIC_URL`, `WEB_DIR`, and `DATABASE_URL`. The UI container and `npm run dev` both use port 5181, so run only one of them.

Tests: `make test` in `backend/` (no NICE; a fake NICE on `httptest`), and `npm test` in `web/` (a fake backend through a `fetch` stub).

## NICE setup checklist

1. Run steps 1-11 of [`backend/README.en.md`](backend/README.en.md#nice-setup-checklist). Step 2 creates the confidential OAuth client in a project of the platform organization with two redirect URIs (`http://localhost:5181/auth/callback` for development and Docker, `http://localhost:8081/auth/callback` for the one-process build), and the smoke test of step 11 checks `/auth/session` and `/auth/login`.
2. Bind the client to `nadi` (needs `nice.platform.catalog.manage`). The binding makes the client a service client: sign-in picks an organization of the user that is entitled to `nadi`, and the client may switch organizations through refresh.

   ```bash
   curl -sS "$NICE/api/v1/projects/$UI_PROJECT_ID/clients/$OAUTH_CLIENT_ID/service" -X PUT \
     -H "Authorization: Bearer $ADMIN_TOKEN" -H 'Content-Type: application/json' -d '{"service":"nadi"}' \
     | jq '.oauth_client | {client_class, service}'
   # {"client_class":"service","service":"nadi"}
   ```

3. Prepare two organizations that use `nadi`: run backend steps 6-8 for organization A with projects P and Q, and organization B with project R, with the same test user as a member of both and an `nadi.editor` binding in P and R. For Q, bind `nadi.viewer` only (to test read-only). Optional: an organization C of the test user that is entitled to `nadi` with no binding in any project.
4. Nothing changes in `nice/.env` for the UI: the browser only navigates to the NICE sign-in page and never calls the NICE API, so NICE CORS is not involved.

Expected result: `GET /api/v1/projects/$UI_PROJECT_ID/clients/$OAUTH_CLIENT_ID/token-reach` answers `client_class` `service`, `service` `nadi`, `base_scopes` containing `offline_access`, and `may_switch_organization` `true`.

## Browser test checklist

Open DevTools (the Network tab with **Preserve log**, and Application > Storage). A, B, C, P, Q, and R come from step 3 above. Run the backend and the UI as in "Running" (development, build, or Docker), and keep the NICE log (`make dev`) in another terminal.

| # | Requirement | Step | Expected result |
|---|---|---|---|
| 1 | Sign-in through the backend | Press **Masuk**, sign in as the test user | Navigation to `/auth/login`, then to `$NICE/oauth/authorize` with `code_challenge_method=S256` and `scope=openid profile email offline_access`. Back at `/` with no `code`, `state`, or token in the URL. The `nadi_session` cookie is `HttpOnly` and `SameSite=Lax`. Name and email show, the active context is the first project of the current organization, and its devices show |
| 2 | No organization uses the service | Sign in with an account without an organization entitled to `nadi` | The screen explains that no organization of the user uses this service, with a **Masuk dengan akun lain** button (`/auth/login?prompt=select_account`). No session cookie |
| 3 | Callback that does not match | Open `/auth/callback?code=x&state=forged` | Back at `/?login_error=state_mismatch`, no session, and the NICE log shows no `POST /oauth/token` |
| 4 | Server-side session | After step 1, look at Network and Storage | The network panel shows requests to the UI origin only (`/auth/*`, `/devices`), besides the sign-in navigation. `localStorage`, `sessionStorage`, and the answer of `/auth/session` hold no access, refresh, or ID token (no `eyJ` string) |
| 5 | Contexts from NICE | Open the context picker | `GET /auth/contexts` holds exactly A/P, A/Q, and B/R with names from NICE; the active context is marked. C shows as unavailable with the reason that the user has no `nadi` access in any project. The NICE log shows the token exchange and `GET /api/v1/me/contexts` |
| 6 | Organization switch | From A/P, pick B/R | One `POST /auth/context`; the NICE log shows exactly one refresh with `nice:org:<B> nice:aud:nadi nice:prj:<R>` and the audit record `token.org_switch`. The page does not navigate; the devices of B show and those of A do not |
| 7 | Permission from the token | Pick A/Q (`nadi.viewer` only) | The Add, Edit, and Delete buttons are not available; the devices of Q show (empty, apart from P) |
| 8 | Access revoked in between | Open the picker, then turn `nadi` off in Q (console: project Q, **Product & Activation**), then pick Q | `POST /auth/context` is answered `409 context_refused` (`invalid_scope`), the active context stays, the UI shows the reason and reloads the choices; Q is no longer offered. Turn Q back on afterwards |
| 9 | Managing devices | In A/P, add, edit, then delete a device | `POST /devices`, `PUT /devices/<id>`, `DELETE /devices/<id>` carry `X-CSRF-Token` and succeed (delete needs `nadi.admin` and `NICE_SERVICE_API_KEY`). The device's `created_by` is the `usr_` of the test user |
| 10 | CSRF | From a page of another origin (for example `http://localhost:5180`), submit a `POST` form to `/devices` of the UI origin | Answered `403 csrf`; no device is added |
| 11 | Bearer caller | Run `backend/scripts/smoke.sh` while the browser session exists | The smoke test passes; `GET /devices` with the Service Account token is answered `200`, unaffected by the cookie |
| 12 | NICE session ended | End the user's session in NICE (console: account, **Sessions**, revoke this session), then pick a context not used before | `401 session_ended`; the UI shows that the session ended and the **Masuk** button |
| 13 | Backend restart | Sign in, stop and start the backend, reload the page | `GET /auth/session` is answered `401`; the UI shows **Masuk** |
| 14 | Sign-out | Press **Keluar** | `POST /auth/logout` is answered `204`; the NICE log shows `POST /oauth/revoke`; the cookie is gone and a reload shows **Masuk** |
| 15 | Sign-out while NICE is down | Sign in, stop NICE, press **Keluar** | `503`; the UI stays signed in and offers to try again. Once NICE runs again, **Keluar** succeeds |
| 16 | One origin | Development: the calls to `/auth/session` and `/devices` come from `http://localhost:5181` without an `OPTIONS` preflight. Build: open `http://localhost:8081`. Docker: open `http://localhost:5181` | `GET /` serves the UI and `GET /auth/session` is answered by the backend on the same origin |

## Production notes

- **Refresh token store.** This example uses `credentials.MemoryStore` and in-memory sessions: a restart signs everybody out, and only one process can run. In production, keep refresh tokens encrypted in a database or Redis, implement `credentials.Store` including a cross-process `Lock` (refresh tokens rotate and only one process may use one at a time), and keep sessions where every process can read them. The Store must keep tokens apart per `Key.Org`.
- **Cookie.** Cookies of `localhost` are visible to every port, so the cookie name is specific. Production uses its own domain, HTTPS (`PUBLIC_URL` `https://...` turns `Secure` on), and `https` redirect URIs.
- **Data per organization and project.** The `tenants` and `devices` tables are keyed by the `org_id` and `project_id` of the token, with no foreign key to NICE. PostgreSQL row-level security (`SET LOCAL app.org_id`) can be added as a second guard. An organization or project deleted in NICE does not notify the service; its data stays until the service cleans it up itself.
- **Labels.** The token only gives the scope (IDs). Organization and project names are read from NICE (`/auth/contexts`) every time the picker opens, because they can change in NICE.
- **Sign-out.** Only the refresh token is revoked; the NICE SSO session stays, so the next **Masuk** may succeed without a password.
