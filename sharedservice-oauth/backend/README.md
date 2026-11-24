# sharedservice-oauth/backend (`nadi`)

[English](README.en.md)

Backend contoh [`sharedservice-oauth`](../README.md): SharedService NICE bermode `nice_token` dengan slug `nadi`, ditulis dalam Go (`net/http`) memakai pustaka rujukan `nice-auth-sdk/verify`. Klien memanggil service ini langsung dengan access token NICE; NICE tidak berada di jalur request. Service memverifikasi token sendiri sesuai [kontrak verifikasi token](../../../nice/docs/token-verification.md), memisahkan data per organisasi dan project, dan memeriksa operasi sensitif secara online ke NICE. Backend yang sama juga melayani UI di [`../web`](../web/README.md) sebagai backend-for-frontend (`/auth/*`, lewat `nice-auth-sdk/credentials`); cara kerja, checklist UI, dan uji browser ada di [`../README.md`](../README.md).

Diuji terhadap nice revisi: SDK `v0.1.0` dan server NICE lokal di Docker (image `nice:dev` dari `c7a3e33`), 2026-11-19, untuk service Bearer: image dibangun ulang dan `scripts/smoke.sh` lulus. Alur UI lihat [`../README.md`](../README.md). Jalur `perms_ref` ("Uji izin yang tidak muat di token") diuji manual pada 2026-11-23 terhadap `nice` `b9eb8b3` dan SDK `4c9649d` (dirilis sebagai `v0.2.0`): token 154 izin terbit dengan `perms_ref`, UI membaca, membuat, dan menghapus perangkat, dan NICE menerima satu penyelesaian per proses `nadi`.

## Isi

| Berkas | Isi |
|---|---|
| `cmd/nadi/main.go` | titik masuk tipis, anotasi swag tingkat dokumen termasuk `x-nice-roles` |
| `internal/bootstrap/` | perakitan: konfigurasi, `niceauth/verify` (`New`, `Run`, `StartFeed`), pemilihan penyimpanan perangkat, `niceauth/credentials` dan sesi, use case, server |
| `internal/adapter/rest/` | `routes.go` (rantai `CORS -> mux`), `device/` (tabel rute tunggal, handler, anotasi swag per operasi), `auth/` (`/auth/*` dan middleware sesi yang memasang token konteks aktif), `web/` (penyaji `web/dist`), `system/` (`/healthz`, `/openapi.json`, `/docs`) |
| `internal/app/device/` | use case perangkat dan kontrak `Repo` yang menerima kunci scope |
| `internal/app/session/` | use case sesi browser (login, konteks, pindah konteks, keluar) dan port `NICE` serta `Store` |
| `internal/domain/` | `Device`, `Scope` (kunci org, project, user, aktor), `Session`, aturan validasi, nama izin |
| `internal/adapter/memory/device/` | penyimpanan perangkat per organisasi dan project di memori |
| `internal/adapter/postgres/device/` | penyimpanan yang sama di PostgreSQL, dipilih bila `DATABASE_URL` diisi |
| `internal/adapter/memory/session/` | sesi browser di memori |
| `internal/adapter/nice/` | port `NICE` lewat `niceauth/credentials` (`Client` `KindBFF` dan `ExchangeClient`), `/oauth/userinfo`, dan `/api/v1/me/contexts` |
| `migrations/` | migrasi SQL (`tenants`, `devices`, beserta `down`), di-embed dan dijalankan saat start bila `DATABASE_URL` diisi |
| `internal/platform/` | `config` (env dan `.env`), `httpx` (CORS untuk SPA lain yang terdaftar), dan `postgres` (pool dan runner migrasi) |
| `Dockerfile` | image backend saja; UI punya image sendiri (`../web/Dockerfile`). Keduanya dipakai `compose.yml` di akar repo |
| `docs/swagger.json` | Swagger 2.0 hasil `make swag`, di-embed dan disajikan di `/openapi.json` |
| `scripts/smoke.sh` | uji smoke terhadap NICE yang sudah diatur |
| `scripts/large-catalog.sh` | mencetak dokumen katalog `nadi` dengan operasi tambahan, untuk mencoba token ber-`perms_ref` |

## Struktur folder

```text
cmd/nadi/main.go             titik masuk: baca config, rakit, jalankan, shutdown
docs/                              swagger.json hasil make swag + embed.go (//go:embed)
internal/
  domain/                          *_domain.go          Device, Scope, Session, aturan validasi, izin
  app/device/
    usecase/                       *_usecase.go         satu aksi per berkas; device_contract.go = interface Repo
    dto/                           *_dto.go             data masukan use case
  app/session/
    usecase/                       *_usecase.go         login, sesi, konteks, keluar; session_contract.go = port NICE dan Store
    dto/                           *_dto.go             userinfo dan konteks dari NICE
  adapter/
    memory/device/                   *_repository.go      implementasi Repo di memori
    postgres/device/                 *_repository.go      implementasi Repo di PostgreSQL
    memory/session/                *_repository.go      sesi browser di memori
    nice/                          *_gateway.go         port NICE lewat niceauth/credentials
    rest/                          routes.go            perakit: verifikasi token, izin, sesi, CORS
    rest/device/                     device_routes.go       tabel rute + <aksi>_device_handler.go
    rest/auth/                     auth_routes.go       /auth/* + middleware sesi dan CSRF
    rest/web/                      web_routes.go        penyaji web/dist
    rest/system/                   /healthz, /openapi.json, dan /docs
  platform/{config,httpx,postgres}/ env, CORS, pool dan migrasi
migrations/                        *.up.sql, *.down.sql  skema PostgreSQL perangkat
  bootstrap/                       perakitan dan tes yang melintasi lapisan
  arch_test.go                     menegakkan aturan impor dan akhiran nama berkas
```

Pola ini meniru modul di backend `nice` (`nice/AGENTS.md`, "Struktur satu modul"), diperkecil untuk satu service dengan satu resource (`device`). Satu aksi satu berkas supaya mudah dicari, paket `deviceusecase` dan `devicedto` diimpor dengan alias eksplisit, dan ketergantungan hanya mengalir ke dalam: adapter memanggil use case, use case dan domain tidak mengimpor `net/http`. Tabel rute di `adapter/rest/device/device_routes.go` tetap satu-satunya sumber kebenaran yang dibandingkan dengan dokumen OpenAPI oleh tes.

## Operasi

| Operasi | Izin | Sensitif |
|---|---|---|
| `GET /devices` | `nadi.devices.read` | |
| `GET /devices/{id}` | `nadi.devices.read` | |
| `POST /devices` | `nadi.devices.create` | |
| `PUT /devices/{id}` | `nadi.devices.edit` | |
| `DELETE /devices/{id}` | `nadi.devices.delete` | ya, diperiksa online ke NICE |

`GET /healthz`, `GET /openapi.json`, dan `GET /docs` (Swagger UI; asetnya dimuat dari CDN jsdelivr) terbuka tanpa token dan tidak ada di dokumen OpenAPI. Di `/docs`, tekan **Authorize** lalu isi `Bearer <JWT>` untuk mencoba operasi. Setelah impor, katalog NICE memuat role `nadi.viewer` (baca), `nadi.admin` (semua izin), dan `nadi.editor` (baca, buat, ubah; dari `x-nice-roles`).

Body perangkat: `{"name": "...", "serial_number": "...", "status": "active", "location": "..."}`. `name` 1 sampai 120 dan `serial_number` 1 sampai 64 karakter setelah spasi di tepi dipangkas, `status` salah satu dari `active`, `inactive`, `maintenance`, dan `location` opsional (paling banyak 120 karakter, bawaan kosong). Galat aplikasi memakai bentuk yang sama dengan `niceauth/verify`: `{"error": "...", "error_description": "..."}`, mis. `400 {"error":"invalid_argument","error_description":"name"}` dan `404 {"error":"not_found"}`.

Perilaku yang perlu diketahui:

- Tenant adalah `nice_org_id` dan project adalah `prj` dari token terverifikasi, tidak pernah dari header, parameter, atau body. Organisasi yang memanggil pertama kali langsung dibuatkan tenant (`ProvisionOnFirstUse`). Token tanpa `prj` dijawab `403 {"error":"insufficient_scope","error_description":"project_required"}`. Tanpa `DATABASE_URL` data hanya di memori dan hilang saat service berhenti.
- Salinan JWKS disimpan di `JWKS_FILE`, sehingga service yang restart ketika NICE mati tetap memverifikasi token yang masih berlaku.
- Tanpa `NICE_SERVICE_API_KEY`, service tetap berjalan dan mencatat peringatan saat start: feed perubahan akses mati (pencabutan baru berlaku saat token kedaluwarsa, paling lama 30 menit), `DELETE /devices/{id}` dijawab `503`, dan setiap panggilan dengan token ber-`perms_ref` juga dijawab `503`.
- Token yang izinnya tidak muat membawa `perms_ref` sebagai pengganti `perms`. NICE menerbitkannya untuk pemanggil yang izin `nadi`-nya melewati batas ukuran token (6 KB). `verify.NewHTTPResolver` mengambil daftar izinnya dari NICE (`POST /api/v1/permission-resolutions`) sekali untuk setiap daftar yang berbeda, memeriksanya terhadap `perms_ref`, lalu menyimpannya di memori. Setelah itu service memutuskan sendiri, juga untuk pemanggil lain yang memegang izin yang sama. Bila izin pemanggil berubah sejak token terbit, NICE menjawab tidak cocok dan service menjawab `401 token_stale`, sehingga klien me-refresh. Bila NICE tidak terjangkau dan daftarnya belum tersimpan, service menjawab `503`.

## Data per organisasi dan project

Perangkat milik satu organisasi dan satu project. Handler membangun kunci scope `domain.Scope{OrgID, ProjectID, Subject, Actor}` dari klaim token terverifikasi (`nice_org_id`, `prj`, `sub`, `act.sub`), use case meneruskannya ke repository, dan repository tidak punya cara lain untuk menyebut organisasi atau project. Perangkat dari scope lain dijawab `404`. `project_id` di query diabaikan, dan field yang tidak dikenal di body, termasuk `project_id`, ditolak `400 invalid_argument` `body`.

Bila `DATABASE_URL` diisi, service menjalankan `migrations/` saat start (tabel `schema_migrations`, advisory lock supaya dua proses tidak menjalankannya bersamaan), lalu menyimpan perangkat di PostgreSQL:

| Tabel | Kolom |
|---|---|
| `tenants` | `org_id` (kunci, dari `nice_org_id`), `created_at` |
| `devices` | `id` (uuid, ditampilkan `dev_<32 hex>`), `org_id`, `project_id` (dari `prj`), `created_by` (dari `sub`), `acted_by` (dari `act`, `NULL` bila tidak ada), `name`, `serial_number`, `status`, `location`, `create_time`, `update_time`; indeks `(org_id, project_id)` |

Setiap query memakai `WHERE org_id = $1 AND project_id = $2`, termasuk baca, ubah, dan hapus per id. Tenant dibuat dengan `INSERT ... ON CONFLICT DO NOTHING`, jadi dua request pertama yang serentak menghasilkan satu baris. Tidak ada foreign key ke NICE: ID organisasi, project, dan user disimpan sebagai teks, dan nama organisasi atau project tidak disimpan. Jangan mengubah migrasi yang sudah dirilis; tambahkan pasangan berkas baru.

## Atribusi agen (`acted_by`)

Backend yang menukar token user lewat token exchange, misalnya [`../../agent-backend`](../../agent-backend/README.md), memanggil service ini dengan token yang `sub`-nya tetap user dan klaim `act.sub`-nya `client_id` service client backend itu. Service memverifikasi token itu dengan langkah yang sama. Aktor tidak mendapat hak tambahan: izinnya tetap `perms` di token, yaitu izin user yang dipotong deklarasi dependensi service backend.

Perangkat yang dibuat dengan token seperti itu mencatat keduanya:

```json
{"id": "dev_...", "name": "Sensor Suhu", "serial_number": "SN-1", "status": "active", "location": "Gudang A", "created_by": "usr_...", "acted_by": "<client_id agen-backend>"}
```

Tanpa `act`, field `acted_by` tidak ada. Kode membacanya dari `Claims.Actor` milik `niceauth/verify` (`internal/adapter/rest/device/create_device_handler.go`), dan tes `TestDeviceCreatedByAnAgentRecordsTheActor` memakai token uji yang membawa `act`.

## Prasyarat

- SDK diambil dari modul publik `github.com/ABA-Developer/nice-auth-sdk` (`v0.2.1` di `go.mod`), jadi contoh ini bisa dibangun tanpa repo `nice`. Repo `nice` di sebelah `example/` hanya dibutuhkan untuk menjalankan NICE lokal dan `nice/scripts/dev-token.sh`.
- Go 1.27, `curl`, `jq`, dan `openssl` (untuk `nice/scripts/dev-token.sh`).
- NICE dev berjalan, bawaannya di `http://localhost:8080` (`cd ../../../nice && make db && make migrate && make dev`). NICE di alamat lain juga bisa: isikan ke `NICE_ISSUER` di `.env` dan ke `NICE` di shell checklist.

## Konfigurasi

Salin `.env.example` ke `.env`. Service membaca `.env` di direktori tempat ia dijalankan; variabel di environment menang atas berkas.

| Variabel | Bawaan | Arti |
|---|---|---|
| `NICE_ISSUER` | wajib | issuer NICE, sama persis dengan klaim `iss` token. Tanpa nilai ini service berhenti dengan pesan yang menyebut `NICE_ISSUER` |
| `LISTEN_ADDR` | `127.0.0.1:8081` | alamat listen |
| `JWKS_FILE` | `data/jwks.json` | salinan JWKS di disk (`data/` tidak di-commit) |
| `NICE_CLIENT_ID`, `NICE_CLIENT_SECRET` | wajib | OAuth client confidential UI (langkah 2). Tanpa keduanya service berhenti dengan pesan yang menyebut nama variabelnya; secret tidak pernah dicetak |
| `PUBLIC_URL` | `http://localhost:8081` | origin UI yang dilihat browser; redirect URI = `PUBLIC_URL/auth/callback`. Dev dengan Vite dan Docker: `http://localhost:5181` |
| `SESSION_IDLE_TIMEOUT` | `8h` | umur sesi browser tanpa pemakaian |
| `WEB_DIR` | `../web/dist` | hasil build UI yang disajikan di `/`; kosong di Docker, karena UI punya container sendiri |
| `DATABASE_URL` | kosong | URL `postgres://` untuk perangkat; kosong berarti memori. Nilainya tidak pernah dicetak |
| `CORS_ALLOWED_ORIGINS` | `http://localhost:5180` | origin browser lain yang boleh memanggil dengan Bearer, dipisah koma, untuk `public-client-pkce`. UI contoh ini tidak butuh CORS karena satu origin |
| `NICE_SERVICE_API_KEY` | kosong | API key Service Account platform berperan `nice.service` (langkah 9); menyalakan feed, pemeriksaan online, dan penyelesaian `perms_ref` |
| `SMOKE_SA_ID`, `SMOKE_API_KEY` | kosong | hanya dibaca `scripts/smoke.sh`: Service Account tenant dan API key-nya (langkah 10) |

## Menjalankan dan tes

```bash
make run          # go run ./cmd/nadi di 127.0.0.1:8081
make test         # tes unit; tidak butuh NICE (token ditandatangani di tes, JWKS statis, NICE palsu httptest)
make swag         # bangkitkan ulang docs/swagger.json dari anotasi
make swag-check   # gagal bila docs/swagger.json tidak sama dengan hasil swag terbaru
scripts/smoke.sh  # butuh NICE yang sudah diatur dengan checklist di bawah
```

Bila `go` di `PATH` tidak cocok dengan `GOROOT`, panggil `make GO=/path/ke/go test`.

Tes repository PostgreSQL dilewati bila `TEST_DATABASE_URL` kosong. Untuk menjalankannya, pakai database kosong milik tes sendiri:

```bash
docker run -d --name nadi-test-pg -e POSTGRES_USER=nadi -e POSTGRES_PASSWORD=nadi -e POSTGRES_DB=nadi -p 127.0.0.1:55460:5432 postgres:17-alpine
TEST_DATABASE_URL='postgres://nadi:nadi@127.0.0.1:55460/nadi?sslmode=disable' go test -race ./internal/... -count=1
docker rm -f nadi-test-pg
```

Tes `TestOpenAPIMatchesTheRouteTable` membandingkan tabel rute di `internal/adapter/rest/device/device_routes.go` dengan `docs/swagger.json` yang di-embed: izin dan tanda sensitif harus sama, dan dokumen tidak boleh memuat operasi lain. Setelah mengubah anotasi, jalankan `make swag` lalu commit hasilnya.

## Checklist setup NICE

Jalankan dari direktori `example/sharedservice-oauth/backend`. Langkah console disebut dengan nama halamannya; potongan `curl` adalah padanannya.

Sebelum mulai, siapkan variabel shell. Token diambil dengan `nice/scripts/dev-token.sh` (client `nice-console` dengan redirect bawaan NICE dev `http://localhost:5173/callback`; bila akun memakai verifikasi dua langkah, set `TOTP_SECRET`, lihat kepala skrip). Token berlaku 30 menit; ulangi bila kedaluwarsa.

```bash
export NICE=${NICE:-http://localhost:8080}   # alamat NICE, sama dengan NICE_ISSUER di .env; dev-token.sh juga membacanya

# akun Platform Admin dev Anda
read -rp 'Email Platform Admin: ' ADMIN_EMAIL; read -rsp 'Password: ' ADMIN_PASSWORD; echo
ADMIN_TOKEN=$(../../../nice/scripts/dev-token.sh "$ADMIN_EMAIL" "$ADMIN_PASSWORD"); unset ADMIN_PASSWORD

# akun owner organisasi tenant dev Anda
read -rp 'Email owner organisasi tenant: ' OWNER_EMAIL; read -rsp 'Password: ' OWNER_PASSWORD; echo
OWNER_TOKEN=$(../../../nice/scripts/dev-token.sh "$OWNER_EMAIL" "$OWNER_PASSWORD"); unset OWNER_PASSWORD

ORG_ID=$(curl -sS "$NICE/api/v1/me/organization" -H "Authorization: Bearer $OWNER_TOKEN" | jq -r .organization.id)
curl -sS "$NICE/api/v1/organizations/$ORG_ID/projects" -H "Authorization: Bearer $OWNER_TOKEN" | jq '.projects[] | {id, name}'
PROJECT_ID=prj_...   # project tenant yang akan memakai nadi
USER_ID=$(curl -sS "$NICE/api/v1/me/profile" -H "Authorization: Bearer $OWNER_TOKEN" | jq -r .profile.user_id)
```

### 1. Env NICE

Tambahkan ke `nice/.env`, lalu jalankan ulang NICE. Tanpa ini NICE menolak endpoint `http` dan alamat loopback.

```text
NICE_OUTBOUND_ALLOW_HTTP=true
NICE_OUTBOUND_ALLOW_CIDRS=127.0.0.0/8
```

`NICE_OUTBOUND_ALLOW_CIDRS` berupa daftar CIDR dipisah koma. Service ini dan UI-nya tidak butuh `NICE_CORS_ALLOWED_ORIGINS`: browser hanya bernavigasi ke halaman login NICE.

### 2. OAuth client UI dan jalankan service

Service tidak mau start tanpa OAuth client UI, jadi client dibuat lebih dulu: client **confidential** di project organisasi platform (client di organisasi tenant tidak bisa diikat ke service). Ikatan ke `nadi` menyusul setelah service terdaftar (langkah 2 di [`../README.md`](../README.md#checklist-setup-nice)). Console: Platform Admin, organisasi platform, **Project**, lalu halaman **OAuth client** project itu: tipe confidential, grant `authorization_code` dan `refresh_token`, dan dua redirect URI.

```bash
PLATFORM_ORG_ID=$(curl -sS "$NICE/api/v1/me/organization" -H "Authorization: Bearer $ADMIN_TOKEN" | jq -r .organization.id)
UI_PROJECT_ID=$(curl -sS "$NICE/api/v1/organizations/$PLATFORM_ORG_ID/projects" -X POST \
  -H "Authorization: Bearer $ADMIN_TOKEN" -H 'Content-Type: application/json' \
  -d '{"name":"NADI UI","slug":"nadi-ui"}' | jq -r .project.id)
CLIENT=$(curl -sS "$NICE/api/v1/projects/$UI_PROJECT_ID/clients" -X POST \
  -H "Authorization: Bearer $ADMIN_TOKEN" -H 'Content-Type: application/json' \
  -d '{"name":"nadi","client_type":"CLIENT_TYPE_CONFIDENTIAL",
       "redirect_uris":["http://localhost:5181/auth/callback","http://localhost:8081/auth/callback"],
       "grant_types":["authorization_code","refresh_token"]}')
OAUTH_CLIENT_ID=$(jq -r .oauth_client.id <<<"$CLIENT")
jq -r .oauth_client.client_id <<<"$CLIENT"   # ke NICE_CLIENT_ID
jq -r .client_secret <<<"$CLIENT"            # ke NICE_CLIENT_SECRET; hanya tampil sekali
unset CLIENT
```

```bash
cp .env.example .env   # lalu isi NICE_CLIENT_ID dan NICE_CLIENT_SECRET
make run
curl -sS http://127.0.0.1:8081/healthz
```

Biarkan berjalan di terminal lain. Untuk dev dengan Vite, isi juga `PUBLIC_URL=http://localhost:5181`. Kunci lain di `.env` diisi pada langkah 9 dan 10.

### 3. Daftarkan `nadi`

Console: Platform Admin, **Shared Service**, **Daftarkan Shared Service**. Endpoint ditulis `http://127.0.0.1:8081`, bukan `localhost`, supaya NICE tidak pernah mencoba `::1`. `auth_mode` tidak perlu dikirim; bawaannya `nice_token`.

```bash
curl -sS "$NICE/api/v1/shared-services" -X POST \
  -H "Authorization: Bearer $ADMIN_TOKEN" -H 'Content-Type: application/json' \
  -d '{"slug":"nadi","name":"NADI","endpoint":"http://127.0.0.1:8081","health_path":"/healthz"}'
```

### 4. Impor `/openapi.json`

Console: detail service `nadi` di **Shared Service**, impor katalog (pratinjau lalu terapkan).

```bash
DOCUMENT=$(curl -sS http://127.0.0.1:8081/openapi.json | jq -Rs .)
PREVIEW=$(curl -sS "$NICE/api/v1/shared-services/nadi/catalog-imports" -X POST \
  -H "Authorization: Bearer $ADMIN_TOKEN" -H 'Content-Type: application/json' \
  -d "{\"document\":$DOCUMENT}")
printf '%s' "$PREVIEW" | jq '{added: [.diff.added[] | {key, permission, sensitive}], roles: [.diff.roles[] | .key]}'
IMPORT_ID=$(printf '%s' "$PREVIEW" | jq -r .import_id)
HASH=$(printf '%s' "$PREVIEW" | jq -r .hash)
curl -sS "$NICE/api/v1/shared-services/nadi/catalog-imports/$IMPORT_ID/apply" -X POST \
  -H "Authorization: Bearer $ADMIN_TOKEN" -H 'Content-Type: application/json' \
  -d "{\"hash\":\"$HASH\"}"
```

Pratinjau harus menunjukkan lima operasi, empat izin (`deleteDevice` sensitif), serta role `nadi.viewer`, `nadi.admin`, dan `nadi.editor`.

### 5. Aktifkan service

```bash
curl -sS "$NICE/api/v1/shared-services/nadi/activate" -X POST -H "Authorization: Bearer $ADMIN_TOKEN"
```

### 6. Entitlement organisasi

Console: Platform Admin, **Organisasi**, detail organisasi tenant.

```bash
curl -sS "$NICE/api/v1/organizations/$ORG_ID/entitlements" -X POST \
  -H "Authorization: Bearer $ADMIN_TOKEN" -H 'Content-Type: application/json' -d '{"service":"nadi"}'
```

### 7. Aktifkan di project

Console: project tenant, **Product & Activation**.

```bash
curl -sS "$NICE/api/v1/projects/$PROJECT_ID/services/nadi/activation" -X PUT -H "Authorization: Bearer $OWNER_TOKEN"
```

### 8. Binding user ke role nadi

Console: project tenant, **IAM**. IAM menyalin katalog dan aktivasi secara asinkron: bila dijawab `unknown_role` atau `service_not_active`, tunggu beberapa detik lalu ulangi. `echo "$BINDING_ID"` harus mencetak `rb_...`, bukan `null`.

```bash
BINDING_ID=$(curl -sS "$NICE/api/v1/projects/$PROJECT_ID/role-bindings" -X POST \
  -H "Authorization: Bearer $OWNER_TOKEN" -H 'Content-Type: application/json' \
  -d "{\"principal_type\":\"PRINCIPAL_TYPE_USER\",\"principal_id\":\"$USER_ID\",\"role\":\"nadi.editor\"}" | jq -r .role_binding.id)
```

`nadi.editor` tidak memuat `nadi.devices.delete`; pakai `nadi.admin` untuk mencoba hapus perangkat.

### 9. Service Account platform `nice.service` (feed, pemeriksaan online, dan `perms_ref`)

Platform Admin membuat Service Account di project organisasi platform (buat project bila belum ada), memberinya role `nice.service`, lalu API key (tanpa pembatasan service: API key tidak punya `allowed_services`; hak ditentukan role Service Account). Console: **Service Account** di project organisasi platform, lalu **Pengguna platform** untuk role `nice.service`.

```bash
PLATFORM_ORG_ID=$(curl -sS "$NICE/api/v1/me/organization" -H "Authorization: Bearer $ADMIN_TOKEN" | jq -r .organization.id)
PLATFORM_PROJECT_ID=$(curl -sS "$NICE/api/v1/organizations/$PLATFORM_ORG_ID/projects" -X POST \
  -H "Authorization: Bearer $ADMIN_TOKEN" -H 'Content-Type: application/json' \
  -d '{"slug":"service-readers","name":"Service readers"}' | jq -r .project.id)
FEED_SA_ID=$(curl -sS "$NICE/api/v1/projects/$PLATFORM_PROJECT_ID/service-accounts" -X POST \
  -H "Authorization: Bearer $ADMIN_TOKEN" -H 'Content-Type: application/json' \
  -d '{"name":"nadi-feed","description":"Feed perubahan akses dan pemeriksaan online nadi"}' | jq -r .service_account.id)
curl -sS "$NICE/api/v1/admin/role-bindings" -X POST \
  -H "Authorization: Bearer $ADMIN_TOKEN" -H 'Content-Type: application/json' \
  -d "{\"principal_type\":\"PRINCIPAL_TYPE_SERVICE_ACCOUNT\",\"principal_id\":\"$FEED_SA_ID\",\"role\":\"nice.service\"}"
curl -sS "$NICE/api/v1/projects/$PLATFORM_PROJECT_ID/api-keys" -X POST \
  -H "Authorization: Bearer $ADMIN_TOKEN" -H 'Content-Type: application/json' \
  -d "{\"service_account_id\":\"$FEED_SA_ID\",\"name\":\"nadi-feed\"}" | jq -r .key
```

Key hanya tampil sekali. Isikan ke `NICE_SERVICE_API_KEY` di `.env`, lalu jalankan ulang `make run`; peringatan feed saat start hilang.

IAM mengetahui Service Account baru secara asinkron, jadi binding yang dibuat tepat setelah akunnya dibuat bisa dijawab 400 `principal_not_in_org`. Tunggu beberapa detik lalu ulangi binding-nya; hal yang sama berlaku di langkah 10.

Untuk deployment dengan beberapa replika, perlu diketahui bahwa setiap replika menyimpan tanda pencabutan di memorinya sendiri, jadi setiap replika membuka long-poll feed-nya sendiri. NICE hanya mengizinkan satu Service Account menahan 4 long-poll sekaligus. Bila replika kelima dan seterusnya memakai key dari Service Account yang sama, replika itu terus dijawab `429 too_many_waits`. Replika tersebut lalu masuk status degraded: log menampilkan "the access change feed is silent", dan pencabutan baru berlaku saat token kedaluwarsa. Karena itu, buat satu Service Account `nice.service` untuk setiap kelompok berisi paling banyak 4 replika.

### 10. Service Account tenant untuk smoke

Console: project tenant, **Service Account** (buat akun dan API key) dan **IAM** (binding).

```bash
SMOKE_SA_ID=$(curl -sS "$NICE/api/v1/projects/$PROJECT_ID/service-accounts" -X POST \
  -H "Authorization: Bearer $OWNER_TOKEN" -H 'Content-Type: application/json' \
  -d '{"name":"nadi-smoke"}' | jq -r .service_account.id)
curl -sS "$NICE/api/v1/projects/$PROJECT_ID/role-bindings" -X POST \
  -H "Authorization: Bearer $OWNER_TOKEN" -H 'Content-Type: application/json' \
  -d "{\"principal_type\":\"PRINCIPAL_TYPE_SERVICE_ACCOUNT\",\"principal_id\":\"$SMOKE_SA_ID\",\"role\":\"nadi.viewer\"}"
curl -sS "$NICE/api/v1/projects/$PROJECT_ID/api-keys" -X POST \
  -H "Authorization: Bearer $OWNER_TOKEN" -H 'Content-Type: application/json' \
  -d "{\"service_account_id\":\"$SMOKE_SA_ID\",\"name\":\"nadi-smoke\"}" | jq -r .key
echo "$SMOKE_SA_ID"
```

Isikan id Service Account ke `SMOKE_SA_ID` dan key ke `SMOKE_API_KEY` di `.env`.

### 11. Smoke

```bash
scripts/smoke.sh
```

Skrip membaca `.env` saja, tidak pernah mencetak key atau token, dan memeriksa: NICE menjawab, `/healthz` menjawab, `GET /auth/session` tanpa cookie dijawab `401 unauthenticated`, `GET /auth/login` mengarahkan ke `$NICE/oauth/authorize` dengan `client_id` dari `NICE_CLIENT_ID`, `GET /devices` tanpa token dijawab `401 missing_token`, token Service Account diterbitkan lewat `client_credentials` dengan scope `nice:aud:nadi` (`client_id` id Service Account, `client_secret` API key), lalu `GET /devices` dengan token itu dijawab `200`. Keluar dengan kode 0 bila semuanya lulus. Kegagalan dipetakan ke langkah checklist:

| Jawaban | Langkah yang kemungkinan terlewat |
|---|---|
| `.env` tidak ada, NICE atau service tidak terjangkau | 2 (dan NICE harus berjalan) |
| `NICE_CLIENT_ID` atau `NICE_CLIENT_SECRET` kosong, `/auth/login` tidak mengarah ke NICE | 2 |
| `invalid_client` | 10: id Service Account bukan pemilik key, key dicabut atau kedaluwarsa, akun nonaktif |
| `invalid_scope` | 3 sampai 7, atau 10 (project Service Account harus mengaktifkan `nadi`) |
| `403 permission_denied` dari service | 10: binding Service Account ke role nadi |

## Uji pencabutan (opsional)

Butuh `NICE_SERVICE_API_KEY` terisi (langkah 9). Token user yang terbit sebelum binding dicabut ditolak `401 token_stale`.

```bash
read -rsp 'Password owner: ' OWNER_PASSWORD; echo
USER_TOKEN=$(../../../nice/scripts/dev-token.sh "$OWNER_EMAIL" "$OWNER_PASSWORD" "" "openid nice:aud:nadi nice:prj:$PROJECT_ID"); unset OWNER_PASSWORD
curl -sS http://127.0.0.1:8081/devices -H "Authorization: Bearer $USER_TOKEN"     # 200
sleep 2
curl -sS -X DELETE "$NICE/api/v1/projects/$PROJECT_ID/role-bindings/$BINDING_ID" -H "Authorization: Bearer $OWNER_TOKEN"
sleep 2
curl -sS http://127.0.0.1:8081/devices -H "Authorization: Bearer $USER_TOKEN"     # 401 token_stale
```

Jeda sebelum pencabutan dibutuhkan karena feed bekerja dengan resolusi detik: token yang terbit pada detik yang sama dengan pencabutan tidak tertangkap.

## Uji izin yang tidak muat di token (opsional)

Butuh `NICE_SERVICE_API_KEY` terisi (langkah 9). NICE membatasi token 6 KB, cukup untuk sekitar 65 sampai 100 izin. Pemanggil yang izin `nadi`-nya lebih banyak menerima token dengan `perms_ref` sebagai pengganti `perms`, dan `nadi` mengambil daftar izinnya dari NICE sekali. `nadi` sendiri hanya punya empat izin, jadi `scripts/large-catalog.sh` mencetak dokumennya dengan operasi tambahan yang masing-masing menyatakan satu izin (`nadi.catalog_test.pNNN.read`). Operasi itu tidak dilayani `nadi`: gunanya hanya memperpanjang daftar izin.

```bash
scripts/large-catalog.sh 150 > /tmp/nadi-large.json   # 154 izin
```

1. Impor `/tmp/nadi-large.json` sebagai katalog `nadi` (langkah 4, dengan berkas ini sebagai dokumennya) dan terapkan.
2. Pastikan user memegang role `nadi.admin` di project (langkah 8). Role itu memuat semua izin aktif `nadi`, jadi sekarang 154.
3. Masuk lewat UI di `PUBLIC_URL`. Daftar perangkat tampil dan perangkat bisa dibuat seperti biasa.

Yang bisa diperiksa:

```bash
read -rsp 'Password user: ' USER_PASSWORD; echo
USER_TOKEN=$(../../../nice/scripts/dev-token.sh "$USER_EMAIL" "$USER_PASSWORD" "" "openid nice:aud:nadi nice:prj:$PROJECT_ID"); unset USER_PASSWORD
printf '%s' "$USER_TOKEN" | cut -d. -f2 | tr '_-' '/+' | base64 -d 2>/dev/null | jq '{perms_ref, perms}'   # perms_ref terisi, perms null
printf '%s' "$USER_TOKEN" | wc -c                                                                        # di bawah 6144
curl -sS -o /dev/null -w '%{http_code}\n' http://127.0.0.1:8081/devices -H "Authorization: Bearer $USER_TOKEN"   # 200
```

Bila NICE berjalan dengan `NICE_METRICS_ENABLED=true`, `nice_oauth_service_tokens_total{permissions="reference"}` naik setiap token seperti itu terbit, dan `nice_permission_resolutions_total{result="match"}` naik satu kali saja walau `GET /devices` dipanggil berulang: `nadi` menyimpan daftar izinnya setelah pengambilan pertama. Tanpa `NICE_SERVICE_API_KEY`, panggilan yang sama dijawab `503 permissions_unavailable`.

Untuk mengembalikan katalog, impor `docs/swagger.json` (atau `/openapi.json`) lagi dan terapkan: operasi tambahan menjadi `removed` dan `nadi.admin` kembali memuat empat izin.
