# service-account-client

[Bahasa Indonesia](README.md)

An example **machine caller**: an integration with no user (a job, a daemon, a partner service) that holds the API key of a Service Account, proves it to NICE, and calls the `nice_token` service `nadi` ([`../sharedservice-oauth/backend`](../sharedservice-oauth/backend/README.en.md)) with the token it gets. Written in Go on `nice-auth-sdk/credentials` with `KindServiceAccount`. It shows four things:

- the API key is **sent to the NICE token endpoint only**. `nadi` never sees the key, only `Authorization: Bearer <token>`;
- one token is reused while it is valid, and renewed in the background before it expires;
- when `nadi` answers 401 `token_stale` or `expired`, a new token is fetched and the request is sent **once** more;
- when NICE refuses the key for good (`invalid_client`), the client stops asking for tokens and exits with an explanation.

```text
client --client_credentials: client_id=sa_..., client_secret=API key, scope=nice:aud:nadi--> NICE /oauth/token
client --Bearer JWT (aud=nadi, nice_principal_type=service_account)--> nadi /devices
```

Minimum NICE version: `nice` `5def119` (the start of the current `nice` history) or later. SDK: `github.com/ABA-Developer/nice-auth-sdk` `v0.2.2` (`Client.HTTPClient` is bound to the address of one service, and addresses must be `https` unless loopback).

Tested against nice revision: SDK `v0.2.2`, `v0.2.1` and `v0.2.0` (2026-11-24) were tested with the unit tests only. SDK `v0.1.0` and a local NICE server in Docker (the `nice:dev` image from `c7a3e33`), 2026-11-19: the unit tests and `scripts/smoke.sh`. With the SDK of `nice` `df58337` (the same code, before it moved to the public module), against the same server: `scripts/smoke.sh`, `create` then `list` (`created_by` is the Service Account id), a wrong key (`invalid_client`), a `NICE_ISSUER` and a `NADI_URL` that are `http` and not loopback (refused before NICE is called), and the key revoked while `watch` runs (`watch` stops with `invalid_client` at the next reading, feed on in `nadi`). A key revoked in the same second its token was issued is not caught by the feed (`iat` has a resolution of one second); the client then stops when the token is renewed. Against `00256d3`, through `Client.HTTPClient`: `scripts/smoke.sh`, `create` then `list` (`created_by` is the Service Account id), a wrong key (`invalid_client`), and NICE unreachable (`nice_unavailable`). Against `c7a3e33` (through `Client.Do`): `create` with the `nadi.viewer` role (403 `permission_denied`), and the key revoked while `watch` runs (`watch` stops with `invalid_client` at the next reading, feed on in `nadi`).

## Folder structure

The pattern is that of `agent-backend` and `sharedservice-oauth/backend`, which mirror `nice/internal/modules` (see `nice/AGENTS.md`), trimmed for one resource (`device`) with no repository and no facade.

```text
cmd/service-account-client/main.go   thin: slog, signals, calls bootstrap.Run, exit code
internal/
  domain/                            pure rules (*_domain.go): the slug and permissions of nadi, Device and its statuses,
                                     the kinds of failure (GrantError, ErrKeyRefused, ErrTemporary, ErrUnreachable, ServiceError)
  app/device/
    usecase/                         one action per file: ListDevices, CreateDevice, WatchDevices (*_usecase.go),
                                     Service (service_device_usecase.go), the Gateway and Waiter contracts (device_contract.go)
    dto/                             CreateInput, Reading, Failure (*_dto.go)
    internal/failure/                errors into a Failure: message and checklist step (*_helper.go)
  adapter/
    cli/                             the command table (routes.go) and one handler per command (*_handler.go);
                                     internal/mapper/ prints devices and a Failure (*_mapper.go)
    nadi/                            the port to nadi through the *http.Client of the SDK (*_gateway.go)
  platform/config/                   env and .env
  bootstrap/                         assembly; cross-layer tests with a fake NICE and a fake nadi
  arch_test.go                       architecture test: file name suffixes, one function per use case, import direction
scripts/smoke.sh                     smoke test against a NICE that is already set up
```

Why: `main.go` only assembles; the rules and use cases import neither `net/http` nor `niceauth/credentials` (only `adapter/nadi` knows the SDK, enforced by `internal/arch_test.go`); every file carries its layer in its name suffix (`_domain`, `_usecase`, `_contract`, `_dto`, `_handler`, `_gateway`, `_helper`, `_mapper`); one `*_usecase.go` holds one function named after the file.

## Commands

| Command | `nadi` permission | What it does |
|---|---|---|
| `list` | `nadi.devices.read` | prints the devices of the Service Account's project as a table |
| `create -name N -serial S [-status ST] [-location L]` | `nadi.devices.create` | registers one device and prints its id. `-status` defaults to `active` (`active`, `inactive`, `maintenance`) |
| `watch` | `nadi.devices.read` | prints the devices every `WATCH_INTERVAL` until stopped with Ctrl+C |

Results go to stdout, logs and failures to stderr. The exit code is `0` on success (including a `watch` the operator stopped), `1` on failure, `2` for a wrong command or flag.

There are no update and delete commands. `DELETE /devices/{id}` is a sensitive operation checked online on the `nadi` side; it teaches nothing more about the client side.

### How a call works

`bootstrap` makes one `credentials.Client` (`Kind: KindServiceAccount`, `ClientID` = the Service Account id, `ClientSecret` = the API key), runs `Run` in the background, and gives `adapter/nadi` a plain `*http.Client` from `client.HTTPClient(credentials.Key{Audience: "nadi"}, cfg.NadiURL)`. The gateway does not call the SDK to send a request; it only reads the errors of the SDK. Every request through that client goes like this:

1. No token yet: the SDK sends `client_credentials` to `/oauth/token` and keeps the token in memory.
2. The token is still valid: it is reused, without contacting NICE.
3. Ten minutes after it was issued (a token lives 30 minutes), `Run` fetches a new token in the background. Calls in progress keep using the old one.
4. `nadi` answers 401 `token_stale` or `expired`: the SDK drops that token, fetches a new one, and sends the request once more. The second answer is returned as it is.
5. NICE answers `invalid_client`: the SDK stops asking for tokens for that Service Account. The next call fails at once without contacting NICE, and the client exits.

The scope names no project: a Service Account token is always for the project the account was created in. There is no refresh token; "renewing" means proving the key again.

That client is bound to the address of `nadi` (`NADI_URL`): the SDK refuses to send the token to another host, a redirect target included, with the error `ErrForeignHost`. The token and the API key travel over `https` only. `http` is accepted for loopback (`localhost`, `127.0.0.1`), so a local stack needs no setting; a `NICE_ISSUER` or `NADI_URL` that is `http` to another host is refused when the client is built, before NICE is called. For a network where TLS ends somewhere else, an application sets `AllowInsecureHTTP` in `credentials.Config`; this example does not use it. The calls of the SDK to NICE follow no redirect.

A failure to get a token comes wrapped in a `*url.Error` from `http.Client`, so the gateway looks for `GrantError` and `IsTemporary` first, and only then for a `*url.Error` as `nadi` being unreachable.

### Shortest form

This example reads `.env` and injects its configuration in tests, so `bootstrap` fills `credentials.Config` itself. An integration that only needs the process environment can use `NewFromEnv`: it reads `NICE_ISSUER`, `NICE_SA_ID`, and `NICE_SA_API_KEY`, does not read `.env`, and its error names the empty variable without its value.

```go
package main

import (
	"context"
	"io"
	"log"
	"os"

	"github.com/ABA-Developer/nice-auth-sdk/credentials"
)

func main() {
	ctx := context.Background()
	c, err := credentials.NewFromEnv() // NICE_ISSUER, NICE_SA_ID, NICE_SA_API_KEY
	if err != nil {
		log.Fatal(err)
	}
	go c.Run(ctx)
	// The client sends the token to this address only; it must be https unless it is loopback.
	hc, err := c.HTTPClient(credentials.Key{Audience: "nadi"}, "http://127.0.0.1:8081")
	if err != nil {
		log.Fatal(err)
	}
	resp, err := hc.Get("http://127.0.0.1:8081/devices")
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(os.Stdout, resp.Body)
}
```

The `curl` equivalent of one `list` call, for other languages:

```bash
set -a; . ./.env; set +a
TOKEN=$(curl -sS "$NICE_ISSUER/oauth/token" -d grant_type=client_credentials -d scope=nice:aud:nadi \
  --data-urlencode "client_id=$NICE_SA_ID" --data-urlencode client_secret@- <<<"$NICE_SA_API_KEY" | jq -r .access_token)
curl -sS "${NADI_URL:-http://127.0.0.1:8081}/devices" -H "Authorization: Bearer $TOKEN" | jq .
```

### Failures

Every failure is printed to stderr as `FAILED: <code> (<description>): <explanation>`, followed by the checklist step to look at. The message holds only the code and description from NICE or `nadi`, never the key, a token, or a raw body.

| Code | From | Meaning | Step | `watch` |
|---|---|---|---|---|
| `invalid_client` | NICE | the key is wrong, revoked, or expired; the Service Account is disabled; or `NICE_SA_ID` does not own the key | 4 | stops |
| `invalid_scope` | NICE | `nadi` is not entitled to the organization or not active in the Service Account's project | 2 | stops |
| `nice_unavailable` | client | NICE did not answer, 5xx, or 429 | | goes on |
| `service_unavailable` | client | `nadi` cannot be reached | 1 | goes on |
| `insufficient_scope` (`permission_denied`) | `nadi` 403 | the token lacks the permission of that command | 3 | stops |
| `invalid_token` (`token_stale`) | `nadi` 401 | access changed and even the new token is refused | | stops |
| `invalid_token` (`wrong_issuer`, `wrong_audience`) | `nadi` 401 | `nadi` trusts another NICE, or is registered under another slug | 1 | stops |
| `invalid_argument` | `nadi` 400 or client | a field of `create` is not valid; the description is the field name | | |
| a 5xx or 429 answer | `nadi` | `nadi` cannot answer now | | goes on |

`watch` goes on only for failures that can heal without a change of setup, and stops with exit code `1` for the others. While NICE is down, a token that has not expired keeps being used.

## Prerequisites

- The SDK comes from the public module `github.com/ABA-Developer/nice-auth-sdk` (`v0.2.2` in `go.mod`), so this example builds without the `nice` repo. The `nice` repo next to `example/` is needed only to run a local NICE and for `nice/scripts/dev-token.sh`.
- Go 1.27, `curl`, `jq`, and `openssl` (for `nice/scripts/dev-token.sh`).
- NICE at the minimum version above, by default at `http://localhost:8080`.
- `nadi` running and set up through step 7 of [`../sharedservice-oauth/backend/README.en.md`](../sharedservice-oauth/backend/README.en.md).

## Configuration

Copy `.env.example` to `.env`. The client reads `.env` in the directory it runs in; a variable in the environment wins over the file.

| Variable | Default | Meaning |
|---|---|---|
| `NICE_ISSUER` | required | the NICE issuer, exactly the `iss` claim of its tokens |
| `NICE_SA_ID` | required | the id of the tenant Service Account (`sa_...`, step 2), sent as `client_id` |
| `NICE_SA_API_KEY` | required | the API key of that Service Account (step 4). Sent to the NICE token endpoint only |
| `NADI_URL` | `http://127.0.0.1:8081` | the address of `sharedservice-oauth/backend` |
| `WATCH_INTERVAL` | `30s` | the pause of `watch`, as a Go duration |

A required variable that is empty stops the client before any network call, with a message that names it.

## Running and tests

```bash
make run                       # list
make run ARGS="create -name 'Sensor Pintu' -serial SN-2026-0002 -location 'Gudang B'"
make run ARGS=watch
make test                      # go test -race; needs no NICE (fake NICE and fake nadi)
make smoke                     # scripts/smoke.sh, needs a NICE set up with the checklist below
```

When the `go` on `PATH` does not match `GOROOT`, call `make GO=/path/to/go test`.

The tests in `internal/bootstrap` run the real assembly with the real `credentials.Client` in front of a fake NICE (the `client_credentials` token endpoint) and a fake `nadi`, so what they prove is the behavior on the wire: how many times `/oauth/token` is called and which headers reach the service. The clock of the SDK is faked so that a token renewal does not wait ten minutes. Every test ends with the same check: the key never appears in a request to `nadi`, and neither the key nor a token ever appears in stdout or stderr.

## NICE setup checklist

Run from the `example/service-account-client` directory, in one shell session. Console steps are named by their page; the `curl` snippets are their equivalent. The password is read from the terminal and never written to a file.

```bash
export NICE=${NICE:-http://localhost:8080}   # the same as NICE_ISSUER in .env; dev-token.sh reads it too

read -rp 'Tenant organization owner email: ' OWNER_EMAIL; read -rsp 'Password: ' OWNER_PASSWORD; echo
OWNER_TOKEN=$(../../nice/scripts/dev-token.sh "$OWNER_EMAIL" "$OWNER_PASSWORD"); unset OWNER_PASSWORD
PROJECT_ID=prj_...   # the tenant project where nadi is active
```

A token lasts 30 minutes; get a new one when `curl` is answered 401.

### 1. `nadi` runs and is registered

Follow [`../sharedservice-oauth/backend/README.en.md`](../sharedservice-oauth/backend/README.en.md) steps 1 through 7: the service runs at `http://127.0.0.1:8081`, is registered as `nadi`, its catalog is imported, it is active, entitled to the tenant organization, and active in `PROJECT_ID`. Step 9 of that example (`NICE_SERVICE_API_KEY`) is not required, but it is needed to try `token_stale`.

### 2. Tenant Service Account

Console: the tenant project, **Service Account**.

```bash
SA_ID=$(curl -sS "$NICE/api/v1/projects/$PROJECT_ID/service-accounts" -X POST \
  -H "Authorization: Bearer $OWNER_TOKEN" -H 'Content-Type: application/json' \
  -d '{"name":"nadi-client"}' | jq -r .service_account.id)
echo "$SA_ID"
```

A Service Account belongs to the project it is created in. A token for `nadi` is only issued when that project activates `nadi`.

### 3. Binding to a `nadi` role

Console: the tenant project, **IAM**.

```bash
curl -sS "$NICE/api/v1/projects/$PROJECT_ID/role-bindings" -X POST \
  -H "Authorization: Bearer $OWNER_TOKEN" -H 'Content-Type: application/json' \
  -d "{\"principal_type\":\"PRINCIPAL_TYPE_SERVICE_ACCOUNT\",\"principal_id\":\"$SA_ID\",\"role\":\"nadi.editor\"}"
```

`nadi.editor` holds `nadi.devices.read`, `.create`, and `.edit`. The key only proves the identity of the account; its permissions come from this binding. For an integration that only reads, use `nadi.viewer`.

IAM learns of a new Service Account asynchronously, so a binding made right after the account may be answered 400 `principal_not_in_org`. Wait a few seconds and repeat it.

### 4. API key and `.env`

Console: the tenant project, **Service Account**, the account `nadi-client`, **API key**.

```bash
curl -sS "$NICE/api/v1/projects/$PROJECT_ID/api-keys" -X POST \
  -H "Authorization: Bearer $OWNER_TOKEN" -H 'Content-Type: application/json' \
  -d "{\"service_account_id\":\"$SA_ID\",\"name\":\"nadi-client\"}" | jq '{id: .api_key.id, key}'
```

The key is shown once. Copy `.env.example` to `.env`, then set `NICE_SA_ID` to `$SA_ID` and `NICE_SA_API_KEY` to that key. `.env` is not committed.

### 5. Smoke

```bash
scripts/smoke.sh
```

The script reads `.env` only, never prints the key or a token, and checks: NICE answers, `nadi` answers `/healthz`, then the `list` command succeeds. It creates no device, so it is safe to run again. Failures map to checklist steps:

| Answer | Step most likely missed |
|---|---|
| `.env` is missing, `NICE_SA_ID` is empty | 2 |
| `NICE_ISSUER` or `NICE_SA_API_KEY` is empty | 4 |
| NICE cannot be reached | NICE must be running, and `NICE_ISSUER` must be its address |
| `nadi` cannot be reached, `service_unavailable` | 1 |
| `invalid_client` | 4: the Service Account id does not own the key, the key is revoked or expired, the account is disabled |
| `invalid_scope` | 2: the Service Account's project must activate `nadi` (step 1) |
| `permission_denied` | 3 |

## Test checklist

1. **Create, then read.** The new device shows in the list, and its `CREATED BY` column is the Service Account id (`sa_...`):

   ```bash
   make run ARGS="create -name 'Sensor Pintu' -serial SN-2026-0002"
   make run
   ```

2. **A read-only role.** Replace the binding of step 3 with `nadi.viewer` (or use another Service Account that only holds `nadi.viewer`), then run `create` again. The answer is `FAILED: insufficient_scope (permission_denied)` with step 3, and exit code `1`. `list` still succeeds.

   A change of binding shows in the next token. A `list` or `create` process always fetches a new token, so it shows at once.

3. **The key is revoked while `watch` runs.** Run `watch`, then revoke its key from the Console (**Service Account**, **API key**, revoke) or with `curl`:

   ```bash
   WATCH_INTERVAL=10s make run ARGS=watch
   # in another terminal; KEY_ID is .api_key.id of the answer of step 4
   curl -sS -X DELETE "$NICE/api/v1/projects/$PROJECT_ID/api-keys/$KEY_ID?service_account_id=$SA_ID" \
     -H "Authorization: Bearer $OWNER_TOKEN" -o /dev/null -w '%{http_code}\n'    # 204
   ```

   How soon `watch` stops depends on `nadi`, because the service verifies the token, not the key:

   - **The access change feed is on in `nadi`** (`NICE_SERVICE_API_KEY` is set): the next reading is answered 401 `token_stale`, the client proves the key again, NICE answers `invalid_client`, and `watch` exits within one interval. This is what was observed when the example was tested.
   - **Without the feed:** the token already issued is still accepted until the client renews it, about ten minutes after the token was issued.

   On both paths the log holds "NICE refused the API key for good", and `watch` exits with `FAILED: invalid_client` and code `1`.

4. **NICE is down for a while.** Stop NICE while `watch` runs: readings go on with the token at hand until that token expires, then every round reports `nice_unavailable` and tries again. Once NICE is back, `watch` recovers without a restart.

## Limits

- **One Service Account, one service.** No several accounts in one process, and no call to an `api_key` service (see [`../shared-service-apikey`](../shared-service-apikey/README.en.md) for the proxy path and the key per Service Account).
- **Tokens in memory only.** Every process fetches its own token; nothing is stored on disk and there is no automatic key rotation.
- **No update and delete operations**, and nothing on behalf of a user: this token is the Service Account's. To act on behalf of a user, see [`../agent-backend`](../agent-backend/README.en.md) (token exchange).
- **Rate limits.** NICE limits `/oauth/token` per address and `client_id`, and counts API key failures per address. Do not ask for a token per request; reuse it as this example does.
