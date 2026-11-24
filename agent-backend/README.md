# agent-backend (`agen`)

[English](README.en.md)

Contoh backend agen AI: MCP server (transport Streamable HTTP) untuk service NICE `agen`, ditulis dalam Go dengan [SDK MCP Go resmi](https://github.com/modelcontextprotocol/go-sdk). Setiap tool memanggil service lain **atas nama user** yang sedang memakai agen, dengan token hasil token exchange (RFC 8693):

- token user hanya dikirim ke endpoint token NICE, tidak pernah ke service lain (tanpa *token passthrough*);
- tidak ada Service Account lintas organisasi: izin token tujuan adalah izin user, dipotong deklarasi dependensi `agen`;
- agen tidak menyimpan refresh token user;
- service tujuan melihat agen sebagai aktor (`act.sub`), dan `inventaris` mencatatnya sebagai `acted_by`.

```text
MCP client --Bearer token user (aud=agen)--> agen /mcp
agen --token exchange: subject_token + client agen-backend--> NICE /oauth/token
agen --Bearer token hasil exchange (aud=inventaris, act.sub=agen-backend)--> inventaris /items
```

Versi NICE minimum: revisi yang memuat `add-service-clients`, `add-delegated-nice-api`, `add-seamless-org-switch`, dan `add-token-exchange` (`nice` `6cc9986` atau sesudahnya). SDK: `github.com/ABA-Developer/nice-auth-sdk` `v0.2.2`. SDK MCP dipin di `go.mod` pada `v1.8.0`.

Diuji terhadap nice revisi: `6cc9986` (2026-11-13). SDK `v0.1.0` (2026-11-19) hanya diuji dengan tes unit, karena `agen` dan `inventaris` tidak terdaftar di stack lokal; `make smoke` belum dijalankan ulang. SDK `v0.2.0`, `v0.2.1`, dan `v0.2.2` (2026-11-24) juga hanya diuji dengan tes unit.

## Struktur folder

Polanya meniru `nice/internal/modules` (lihat `nice/AGENTS.md`), dipangkas untuk satu service kecil: satu resource (`call`: pemanggilan service lain atas nama user), tanpa facade dan repository.

```text
cmd/agent-backend/main.go            tipis: memanggil bootstrap.Run
docs/                                openapi.json (ditulis tangan) + paket kecil yang meng-embed-nya; tes OpenAPI dan README
internal/
  domain/                            aturan murni: slug, izin, target, format ID (*_domain.go)
  app/call/
    usecase/                         ExecuteCall (execute_call_usecase.go), Service, kontrak Sender (call_contract.go)
    dto/                             Caller, Where, Key, Request, Response, Failure (*_dto.go)
    internal/{key,failure}/          validasi ID dan pemetaan galat ke Failure (*_helper.go)
  adapter/
    mcp/                             rute HTTP dan rantai niceauth/verify (routes.go), server MCP dan skema tool (tools_routes.go),
                                     satu handler per tool (*_handler.go); internal/ untuk caller, challenge, mapper
    niceexchange/                    pembungkus niceauth: token exchange dan DoAs (sender_client.go)
  platform/config/                   env dan .env
  bootstrap/                         perakitan; tes integrasi dengan NICE palsu
  arch_test.go                       tes arsitektur: akhiran nama berkas dan arah impor
scripts/smoke.sh                     uji smoke terhadap NICE yang sudah diatur
```

Alasannya: `main.go` hanya merakit; aturan dan use case tidak mengimpor `net/http`, SDK MCP, `niceauth/credentials`, atau `niceauth/verify` (adapter yang mengimpor, ditegakkan `internal/arch_test.go`); setiap berkas membawa lapisannya di akhiran nama (`_domain`, `_usecase`, `_contract`, `_dto`, `_handler`, `_routes`, `_client`, `_helper`, `_mapper`); satu `*_usecase.go` berisi satu fungsi bernama sama dengan berkasnya. `openapi.json` tetap ditulis tangan dan di-embed dari `docs/`; `docs/openapi_test.go` menjaga bahwa izin di dokumen sama dengan yang ditegakkan kode.

## Endpoint dan tool

| Endpoint | Akses |
|---|---|
| `POST /mcp` | token NICE `aud=agen` dengan izin `agen.mcp.use` |
| `GET /.well-known/oauth-protected-resource` (juga `.../mcp`) | terbuka: `resource` `<PUBLIC_URL>/mcp`, `authorization_servers` issuer NICE, `scopes_supported` scope dasar dan `nice:aud:agen` |
| `GET /healthz`, `GET /openapi.json` | terbuka, tidak ada di dokumen OpenAPI |

Tanpa token, `/mcp` dijawab `401` dengan `WWW-Authenticate: Bearer resource_metadata="<PUBLIC_URL>/.well-known/oauth-protected-resource"`. Token untuk service lain dijawab `401` `wrong_audience`, token tanpa `agen.mcp.use` dijawab `403` `permission_denied`.

| Tool | Tujuan | Izin dependensi |
|---|---|---|
| `inventaris_list_items` | `GET <INVENTARIS_URL>/items` | `inventaris.items.read` |
| `inventaris_create_item` (`name`, `quantity`) | `POST <INVENTARIS_URL>/items` | `inventaris.items.create` |

Setiap tool menerima `organization_id` (`org_...`) dan `project_id` (`prj_...`) opsional. Tanpa keduanya, exchange hanya meminta `nice:aud:<service>`, dan NICE memakai organisasi dan project token user. Organisasi lain wajib disertai project-nya. Agen tidak memilih konteks yang tidak disebut user, dan ID yang tidak berbentuk `org_...` atau `prj_...` ditolak sebelum menjadi scope.

Operasi sensitif sengaja tidak punya tool: menghapus barang (`DELETE /items/{id}`, sensitif di katalog `inventaris`). Persetujuan manusia untuk operasi tulis adalah kebijakan aplikasi agen, di luar contoh ini.

### Mengapa hanya service `nice_token`

Service `api_key` memakai kunci per Service Account dan menolak token user (`403` `service_account_required`), jadi agen yang bertindak atas nama user hanya menyediakan tool untuk service `nice_token`. Contoh service `api_key` di repo ini, `pengiriman`, karena itu tidak punya tool di sini. Memanggil service `api_key` lewat Service Account milik agen akan menghilangkan identitas user (tidak ada lagi `created_by` user di service tujuan) dan bukan pola contoh ini.

### Cara kerja panggilan

`callAs` memanggil `credentials.ExchangeClient.DoAs` dengan token user dari request MCP dan kunci `{Org, Project, Audience}`. `DoAs`:

- menyimpan token per `(user, organisasi, project, service)` sampai 60 detik sebelum kedaluwarsa, dan tidak pernah memakai token yang umurnya melebihi token user;
- menjalankan satu exchange per kunci untuk panggilan paralel, tanpa antrean antar kunci, sehingga konteks (organisasi atau project) yang berbeda ditukar bersamaan;
- mengulang exchange dan request sekali bila service menjawab `401` `token_stale` atau `expired`.

Transport MCP berjalan *stateless* dengan jawaban JSON: setiap `POST /mcp` berdiri sendiri, sehingga `curl` bisa memanggil tool dalam satu request dan panggilan paralel dari satu client tidak saling menunggu.

### Galat

Kegagalan dikembalikan sebagai hasil tool bertanda galat (`isError: true`) berisi kode, alasan singkat dari NICE atau service bila ada, dan penjelasan. Token, secret, dan body mentah tidak pernah masuk hasil maupun log.

| Kode | Sumber | Arti |
|---|---|---|
| `access_denied` | exchange | user bukan anggota aktif organisasi itu, organisasi tidak aktif, atau tidak meng-entitle service tujuan atau `agen` |
| `invalid_scope` | exchange | service bukan dependensi `agen` atau tidak aktif, atau project tidak aktif di organisasi itu atau tidak mengaktifkan service tujuan |
| `invalid_grant` (`subject_token_expiring`) | exchange | token user tinggal kurang dari 60 detik: perbarui token di MCP client |
| `invalid_grant` (`token_stale`, `subject_invalid`) | exchange | akses user berubah, token bukan token user untuk `agen`, atau sesi berakhir: login lagi |
| `invalid_client`, `unauthorized_client` | exchange | client agen ditolak: periksa `NICE_CLIENT_ID` dan `NICE_CLIENT_SECRET` |
| `nice_unavailable` | exchange | NICE tidak menjawab |
| `insufficient_scope` (`permission_denied`), `permission_denied` | service | user tidak memegang izin operasi itu di project tersebut |
| `invalid_argument` | agen | `organization_id` atau `project_id` salah bentuk, atau organisasi lain tanpa `project_id` |

Contoh teks hasil tool untuk organisasi yang bukan milik user:

```text
access_denied (organization is not usable): NICE refused a token for inventaris in that organization: the user is not an active member of it, the organization is not active, or it does not use inventaris or agen
```

## Prasyarat

- SDK diambil dari modul publik `github.com/ABA-Developer/nice-auth-sdk` (`v0.2.2` di `go.mod`), jadi contoh ini bisa dibangun tanpa repo `nice`. Repo `nice` di sebelah `example/` hanya dibutuhkan untuk menjalankan NICE lokal dan `nice/scripts/dev-token.sh`.
- Go 1.27, `curl`, `jq`, dan `openssl` (untuk `nice/scripts/dev-token.sh`).
- NICE dengan versi minimum di atas, bawaannya di `http://localhost:8080`.
- `inventaris` berjalan dan sudah diatur sampai langkah 8 [`../sharedservice-oauth/backend/README.md`](../sharedservice-oauth/backend/README.md). User uji memegang role `inventaris.operator` di project yang sama.
- Untuk uji organisasi lain: user uji juga anggota organisasi kedua, dengan project yang mengaktifkan `inventaris` dan binding `inventaris.operator` di sana.

## Konfigurasi

Salin `.env.example` ke `.env`. Agen membaca `.env` di direktori tempat ia dijalankan; variabel di environment menang atas berkas.

| Variabel | Bawaan | Arti |
|---|---|---|
| `NICE_ISSUER` | wajib | issuer NICE, sama persis dengan klaim `iss` token |
| `NICE_CLIENT_ID` | wajib | `client_id` client confidential `agen-backend` (langkah 8) |
| `NICE_CLIENT_SECRET` | wajib | secret client itu. Hanya dikirim ke endpoint token NICE |
| `INVENTARIS_URL` | wajib | alamat `inventaris`, mis. `http://127.0.0.1:8081` |
| `LISTEN_ADDR` | `127.0.0.1:8083` | alamat listen |
| `PUBLIC_URL` | `http://127.0.0.1:8083` | alamat yang dipakai MCP client; `resource` metadata adalah `<PUBLIC_URL>/mcp` |
| `JWKS_FILE` | `data/jwks.json` | salinan JWKS di disk (`data/` tidak di-commit) |
| `NICE_SERVICE_API_KEY` | kosong | API key Service Account platform berperan `nice.service`; menyalakan penyelesaian `perms_ref`. Kosong: token ber-`perms_ref` (pemanggil yang izin `agen`-nya tidak muat di token) dijawab `503` |

Variabel wajib yang kosong menghentikan start dengan pesan yang menyebut namanya.

## Menjalankan dan tes

```bash
make run          # go run ./cmd/agent-backend di 127.0.0.1:8083
make test         # go test -race; tidak butuh NICE (NICE dan inventaris palsu)
make smoke        # scripts/smoke.sh, butuh NICE yang sudah diatur dengan checklist di bawah
```

Bila `go` di `PATH` tidak cocok dengan `GOROOT`, panggil `make GO=/path/ke/go test`.

Tes menjalankan agen lewat HTTP di dalam proses dengan client MCP dari SDK, di depan NICE palsu yang meniru aturan token exchange. Tes yang menjaga pola: service palsu gagal bila pernah menerima token `aud=agen` (tanpa passthrough), sepuluh panggilan dalam satu konteks menghasilkan satu exchange, dan exchange untuk dua konteks (organisasi atau project) yang berbeda berjalan bersamaan.

## Checklist setup NICE

Jalankan dari direktori `example/agent-backend`, dalam satu sesi shell. Langkah console disebut dengan nama halamannya; potongan `curl` adalah padanannya. Password dibaca dari terminal dan tidak pernah ditulis ke berkas.

```bash
export NICE=${NICE:-http://localhost:8080}   # sama dengan NICE_ISSUER di .env; dev-token.sh juga membacanya

read -rp 'Email Platform Admin: ' ADMIN_EMAIL; read -rsp 'Password: ' ADMIN_PASSWORD; echo
ADMIN_TOKEN=$(../../nice/scripts/dev-token.sh "$ADMIN_EMAIL" "$ADMIN_PASSWORD"); unset ADMIN_PASSWORD
read -rp 'Email owner organisasi tenant: ' OWNER_EMAIL; read -rsp 'Password: ' OWNER_PASSWORD; echo
OWNER_TOKEN=$(../../nice/scripts/dev-token.sh "$OWNER_EMAIL" "$OWNER_PASSWORD"); unset OWNER_PASSWORD

ORG_ID=$(curl -sS "$NICE/api/v1/me/organization" -H "Authorization: Bearer $OWNER_TOKEN" | jq -r .organization.id)
PROJECT_ID=prj_...   # project tenant tempat inventaris aktif
USER_ID=usr_...      # user uji yang memakai agen
PLATFORM_ORG_ID=$(curl -sS "$NICE/api/v1/me/organization" -H "Authorization: Bearer $ADMIN_TOKEN" | jq -r .organization.id)
```

Token berlaku 30 menit; ambil ulang bila `curl` dijawab 401.

### 1. Env NICE

Sama dengan contoh service lain: `nice/.env` memuat `NICE_OUTBOUND_ALLOW_HTTP=true` dan `NICE_OUTBOUND_ALLOW_CIDRS=127.0.0.0/8`, lalu NICE dijalankan ulang. Tanpa keduanya NICE menolak endpoint `http://127.0.0.1:8083`.

### 2. Daftarkan `agen`

Console: Platform Admin, **Shared Service**, **Daftarkan Shared Service**. Mode bawaan `nice_token`.

```bash
curl -sS "$NICE/api/v1/shared-services" -X POST \
  -H "Authorization: Bearer $ADMIN_TOKEN" -H 'Content-Type: application/json' \
  -d '{"slug":"agen","name":"Agen","endpoint":"http://127.0.0.1:8083","health_path":"/healthz"}' | jq '.shared_service | {slug, status}'
```

### 3. Impor `docs/openapi.json`

Console: detail service `agen`, impor katalog. Dokumen diambil dari berkas, karena agen baru bisa berjalan setelah client-nya ada (langkah 8).

```bash
PREVIEW=$(jq -n --rawfile doc docs/openapi.json '{document: $doc}' | curl -sS "$NICE/api/v1/shared-services/agen/catalog-imports" -X POST \
  -H "Authorization: Bearer $ADMIN_TOKEN" -H 'Content-Type: application/json' -d @-)
jq '{added: [.diff.added[]? | {key, permission}], roles: [.diff.roles[]? | .key]}' <<<"$PREVIEW"
curl -sS "$NICE/api/v1/shared-services/agen/catalog-imports/$(jq -r .import_id <<<"$PREVIEW")/apply" -X POST \
  -H "Authorization: Bearer $ADMIN_TOKEN" -H 'Content-Type: application/json' -d "$(jq '{hash}' <<<"$PREVIEW")"
```

Pratinjau menunjukkan satu operasi `mcp` (`agen.mcp.use`) dan role `agen.admin` serta `agen.user` (dari `x-nice-roles`). Tidak ada `agen.viewer`, karena dokumen tidak punya operasi baca.

### 4. Dependensi `agen` ke `inventaris`

Butuh `nice.platform.catalog.manage`. Izin token hasil exchange dipotong daftar ini, jadi agen tidak pernah bisa menghapus barang walau user bisa. Daftar diganti utuh.

```bash
curl -sS "$NICE/api/v1/shared-services/agen/dependencies" -X PUT \
  -H "Authorization: Bearer $ADMIN_TOKEN" -H 'Content-Type: application/json' \
  -d '{"dependencies":[
        {"service":"inventaris","permissions":["inventaris.items.read","inventaris.items.create"]}]}'
```

### 5. Aktifkan service

```bash
curl -sS "$NICE/api/v1/shared-services/agen/activate" -X POST -H "Authorization: Bearer $ADMIN_TOKEN" | jq '.shared_service | {slug, status}'
```

### 6. Project di organisasi platform

Service client harus milik project organisasi platform. Pakai project yang ada, atau buat satu (console: Platform Admin, organisasi platform, **Project**).

```bash
AGENT_PROJECT_ID=$(curl -sS "$NICE/api/v1/organizations/$PLATFORM_ORG_ID/projects" -X POST \
  -H "Authorization: Bearer $ADMIN_TOKEN" -H 'Content-Type: application/json' \
  -d '{"name":"Agen","slug":"agen"}' | jq -r .project.id)
```

### 7. Client `agen-user` (public)

Client untuk user dan MCP client memperoleh token `aud=agen`. Console: project itu, **OAuth client**, lalu ikat ke `agen` (butuh `nice.platform.catalog.manage`). Redirect URI di bawah milik MCP Inspector; `dev-token.sh` memakai yang pertama.

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

Aktor token exchange. Exchange menolak client `public`, dan memisahkan dua client menunjukkan bahwa secret hanya ada di server agen. NICE mewajibkan redirect URI dan grant `authorization_code` untuk setiap OAuth client, walau backend ini tidak pernah login.

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

Secret hanya tampil sekali dan langsung masuk `.env` tanpa dicetak. Bila hilang, putar secret client itu di console lalu ganti `NICE_CLIENT_SECRET`.

### 9. Organisasi user: entitlement, aktivasi, binding

Login lewat service client hanya selesai di organisasi yang meng-entitle `agen`. Exchange ke organisasi lain memeriksa hal yang sama ditambah entitlement service tujuan, tetapi tidak meminta `agen` aktif di project organisasi itu. Console: Platform Admin, **Organisasi** (entitlement), lalu project tenant, **Product & Activation** dan **IAM**.

```bash
curl -sS "$NICE/api/v1/organizations/$ORG_ID/entitlements" -X POST \
  -H "Authorization: Bearer $ADMIN_TOKEN" -H 'Content-Type: application/json' -d '{"service":"agen"}'
curl -sS "$NICE/api/v1/projects/$PROJECT_ID/services/agen/activation" -X PUT -H "Authorization: Bearer $OWNER_TOKEN"
curl -sS "$NICE/api/v1/projects/$PROJECT_ID/role-bindings" -X POST \
  -H "Authorization: Bearer $OWNER_TOKEN" -H 'Content-Type: application/json' \
  -d "{\"principal_type\":\"PRINCIPAL_TYPE_USER\",\"principal_id\":\"$USER_ID\",\"role\":\"agen.user\"}" | jq -r .role_binding.id
```

IAM menyalin katalog dan aktivasi secara asinkron: bila binding dijawab `unknown_role` atau `service_not_active`, tunggu beberapa detik lalu ulangi. Untuk uji organisasi lain, beri organisasi kedua entitlement `agen` juga.

### 10. Jalankan agen

```bash
make run
curl -sS http://127.0.0.1:8083/healthz
```

Biarkan berjalan di terminal lain.

### 11. Token user

Token user untuk agen diminta lewat client `agen-user` dengan `nice:aud:agen` dan project tempat user memegang `agen.user`. `nice:org` memastikan login masuk ke organisasi itu.

```bash
read -rp 'Email user uji: ' USER_EMAIL; read -rsp 'Password: ' USER_PASSWORD; echo
USER_TOKEN=$(CLIENT_ID=$USER_CLIENT_ID REDIRECT=http://localhost:6274/oauth/callback \
  ../../nice/scripts/dev-token.sh "$USER_EMAIL" "$USER_PASSWORD" "" "openid nice:org:$ORG_ID nice:aud:agen nice:prj:$PROJECT_ID")
unset USER_PASSWORD
export AGENT_USER_TOKEN=$USER_TOKEN   # dibaca scripts/smoke.sh
```

### 12. Smoke

```bash
make smoke
```

Skrip membaca `.env` dan `AGENT_USER_TOKEN` dari environment, tidak pernah mencetak token atau secret, dan memeriksa: NICE menjawab, `/healthz` menjawab, `/mcp` tanpa token dijawab `401` dengan `resource_metadata`, metadata menyebut `NICE_ISSUER`, lalu dengan token user `tools/list` memuat dua tool dan `inventaris_list_items` sukses. Kegagalan dipetakan ke langkah:

| Jawaban | Langkah yang kemungkinan terlewat |
|---|---|
| `.env` tidak ada, agen tidak terjangkau | 8, 10 |
| `/mcp` dengan token dijawab `401` `wrong_audience` | 11: token harus diminta dengan `nice:aud:agen` lewat `agen-user` |
| `/mcp` dijawab `403` `permission_denied` | 9: binding `agen.user` |
| tool dijawab `invalid_client` | 8: `NICE_CLIENT_ID` atau `NICE_CLIENT_SECRET` |
| tool dijawab `invalid_scope` | 4 (dependensi), atau `inventaris` belum aktif di project token |
| tool dijawab `invalid_grant` | 11: token user hampir habis, basi, atau sesinya berakhir; ambil token baru |

## Checklist uji

Dengan `USER_TOKEN` dari langkah 11. Fungsi `tool` memanggil satu tool dalam satu request:

```bash
tool() {   # tool <nama> [argumen JSON]
  jq -nc --arg name "$1" --argjson args "${2:-"{}"}" \
    '{jsonrpc: "2.0", id: 1, method: "tools/call", params: {name: $name, arguments: $args}}' |
  curl -sS http://127.0.0.1:8083/mcp -H "Authorization: Bearer $USER_TOKEN" \
    -H 'Content-Type: application/json' -H 'Accept: application/json, text/event-stream' -d @- |
  jq '.result | {isError, structuredContent, text: .content[0].text}'
}
```

1. **Daftar tool.** Dua tool `inventaris`, tanpa operasi hapus:

   ```bash
   curl -sS http://127.0.0.1:8083/mcp -H "Authorization: Bearer $USER_TOKEN" -H 'Content-Type: application/json' \
     -H 'Accept: application/json, text/event-stream' -d '{"jsonrpc":"2.0","id":1,"method":"tools/list"}' | jq '[.result.tools[].name]'
   ```

2. **Barang dibuat agen.** `created_by` adalah user dan `acted_by` adalah `client_id` `agen-backend` (`NICE_CLIENT_ID`):

   ```bash
   tool inventaris_create_item '{"name":"Kabel UTP","quantity":3}'
   tool inventaris_list_items
   ```

3. **Organisasi lain milik user.** Dengan organisasi kedua (`ORG_B_ID`, project `PROJECT_B_ID`) yang memenuhi prasyarat dan meng-entitle `agen`:

   ```bash
   tool inventaris_list_items "{\"organization_id\":\"$ORG_B_ID\",\"project_id\":\"$PROJECT_B_ID\"}"
   ```

   Sukses, dengan barang organisasi itu.

4. **Organisasi bukan milik user.** Hasilnya galat `access_denied`, dan `inventaris` tidak dipanggil:

   ```bash
   tool inventaris_list_items "{\"organization_id\":\"$OTHER_ORG_ID\",\"project_id\":\"$OTHER_PROJECT_ID\"}"
   ```

5. **Audit NICE.** Setiap exchange tercatat `token.exchange` dengan pelaku user dan `details` `client_id`, `service`, dan `project_id`. Exchange yang sukses tercatat di organisasi tujuan. Penolakan tercatat di organisasi token user, dengan `details.reason` (mis. `not_member`, `invalid_scope`) dan `details.target_organization_id`:

   ```bash
   curl -sS "$NICE/api/v1/audit/records?action=token.exchange&page_size=10" -H "Authorization: Bearer $OWNER_TOKEN" \
     | jq '.records[] | {actor_id, outcome, project_id, details}'
   ```

   Agen menyimpan token hasil exchange, jadi banyak panggilan tool dalam satu konteks hanya menghasilkan satu record per konteks.

6. **MCP Inspector (opsional).** `npx @modelcontextprotocol/inspector`, transport Streamable HTTP ke `http://127.0.0.1:8083/mcp`. NICE tidak menyediakan Dynamic Client Registration, jadi isi `client_id` `agen-user` secara manual dan tambahkan `nice:prj:<project>` ke scope; scope dari metadata saja tidak cukup untuk token service client. Langkah ini tidak termasuk uji integrasi contoh ini.

## Batas

- **Tanpa DCR.** Client MCP didaftarkan manual (langkah 7).
- **Tanpa tool service `api_key`.** Service itu hanya melayani Service Account (lihat "Mengapa hanya service `nice_token`"), jadi contoh ini tidak menunjukkan pemanggilan service `api_key`.
- **Tanpa tool sensitif**, tanpa UI chat atau LLM, dan tanpa pekerjaan agen saat user tidak hadir.
- **Tanpa feed perubahan akses di agen.** Token user yang basi tetap ditolak di exchange (`invalid_grant` `token_stale`), karena NICE memeriksa feed sendiri.
