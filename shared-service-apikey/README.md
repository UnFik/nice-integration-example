# Contoh service `api_key`: `pengiriman` (Hono)

[English](README.en.md)

Service lama yang hanya mengenal API key miliknya sendiri, didaftarkan di NICE sebagai SharedService bermode `api_key` dengan slug `pengiriman`. Klien tidak memanggilnya langsung. Klien memanggil proxy NICE di `<NICE>/pengiriman/...` dengan JWT NICE; NICE memverifikasi token, mencocokkan operasi dengan katalog, memeriksa izin, lalu meneruskan request ke service dengan kredensial service di header `X-Api-Key` dan identitas pemanggil di header `X-Nice-*`.

```text
klien --Authorization: Bearer <JWT>--> NICE /pengiriman/shipments
NICE  --X-Api-Key + X-Nice-Organization/Project/Principal--> http://127.0.0.1:8082/shipments
```

Service ini tidak memverifikasi JWT dan tidak memeriksa izin. Keduanya tugas proxy. Service hanya memeriksa `X-Api-Key`.

Tiga operasi key (`POST /keys`, `POST /keys/{key_id}/rotate`, `DELETE /keys/{key_id}`) wajib didaftarkan di NICE bersama service (langkah 4). Karena itu NICE membuat satu kunci per Service Account, dan **hanya Service Account yang bisa memanggil `pengiriman` lewat proxy**: user NICE dijawab 403 `service_account_required`.

Kontrak NICE yang diikuti: `nice/docs/api.md` bagian "Service api_key dan proxy", "Kunci per Service Account (mirrored key)", dan "Service Account: `client_credentials`", serta `nice/docs/gateway-integration.md` bagian "Service lama yang tidak bisa diubah".

Diuji terhadap nice revisi: `40ddb20` (2026-11-18)

## Peringatan: header `X-Nice-*`

Service memercayai `X-Nice-Organization` (tenant) dan `X-Nice-Principal` (pelaku) **hanya karena** request membawa kredensial yang sah. Kredensial itu bukti bahwa request datang dari proxy NICE, dan proxy membuang header `X-Nice-*` kiriman klien lalu mengisinya dari token yang sudah diverifikasi. Akibatnya, siapa pun yang memegang `SERVICE_CREDENTIAL` bisa mengaku sebagai organisasi dan principal mana pun.

- Jangan berikan `SERVICE_CREDENTIAL` kepada pihak selain NICE.
- Jangan membuka service ini ke jaringan publik. Biarkan `LISTEN_ADDR` di loopback (`127.0.0.1`), atau batasi dengan firewall sehingga hanya NICE yang bisa menjangkaunya.

## Operasi

| Operasi | Kredensial | Di `/openapi.json` | Izin katalog |
|---|---|---|---|
| `GET /shipments` | service atau mirrored key | ya | `pengiriman.shipments.read` |
| `POST /shipments` body `{"destination"}` | service atau mirrored key | ya | `pengiriman.shipments.create` |
| `POST /shipments/{id}/cancel` | service atau mirrored key | ya | `pengiriman.shipments.cancel` |
| `POST /keys` body `{"label"}` | hanya kredensial service | tidak | - |
| `POST /keys/{key_id}/rotate` | hanya kredensial service | tidak | - |
| `DELETE /keys/{key_id}` | hanya kredensial service | tidak | - |
| `GET /healthz` | tanpa kredensial | tidak | - |
| `GET /openapi.json` | tanpa kredensial | tidak | - |

Operasi pengiriman memakai `X-Nice-Organization` sebagai tenant dan mencatat `X-Nice-Principal` sebagai `created_by`. Data disimpan di memori per organisasi dan hilang saat restart. Semua galat berbentuk `{"error":"<kode>"}`, ditambah `error_description` untuk `invalid_argument`:

| Status | `error` | Kapan |
|---|---|---|
| 401 | `unauthorized` | `X-Api-Key` tidak ada atau tidak cocok |
| 400 | `missing_organization` | request berkredensial ke operasi pengiriman tanpa `X-Nice-Organization` |
| 400 | `invalid_argument` | `destination` kosong atau lebih dari 200 karakter (`error_description: "destination"`), body bukan JSON (`"body"`), atau `label` tidak sah (`"label"`) |
| 415 | `unsupported_media_type` | body `POST /shipments` tanpa `Content-Type: application/json` |
| 404 | `not_found` | pengiriman tidak ada di organisasi itu, key tidak dikenal, atau path tidak dikenal |
| 409 | `already_cancelled` | pengiriman sudah dibatalkan |
| 403 | `admin_credential_required` | mirrored key memanggil operasi key |

Dokumen `/openapi.json` (OpenAPI 3.0) dibuat oleh `@hono/zod-openapi` dari rute pengiriman saja. Operasi key, `/healthz`, dan `/openapi.json` didaftarkan sebagai rute Hono biasa sehingga tidak pernah menjadi operasi katalog yang bisa dipanggil lewat proxy (`POST <NICE>/pengiriman/keys` dijawab 404 oleh proxy).

### Operasi key untuk mirrored key

Ketiga operasi key didaftarkan di langkah 4 dan dipanggil NICE untuk membuat, merotasi, dan mencabut kunci tiap Service Account (diperiksa di langkah 12). NICE mengirim kredensial service di `X-Api-Key`, tanpa header `X-Nice-*`.

| Operasi | Jawaban |
|---|---|
| `POST /keys` body `{"label":"<uuid>"}` | 201 `{"key_id","key"}`. Key langsung diterima di operasi pengiriman |
| `POST /keys/{key_id}/rotate` | 200 `{"key_id","key"}` dengan `key_id` yang sama; key lama langsung ditolak. 404 bila id tidak dikenal atau sudah dicabut. Body `{"key_id"}` dari NICE diabaikan |
| `DELETE /keys/{key_id}` | 204 (juga untuk key yang sudah dicabut); 404 bila id tidak dikenal. NICE menganggap 404 sebagai sudah dicabut |

Key berbentuk `pgk_` + 32 byte acak base64url, dan id-nya `key_` + 24 karakter heksadesimal. `KEYS_FILE` (bawaan `data/keys.json`) hanya menyimpan `{key_id, label, sha256, status, create_time}`, tidak pernah key utuh. Berkas ditulis ke berkas sementara di direktori yang sama, di-`fsync`, lalu di-`rename`, sehingga tidak pernah setengah jadi dan key tetap berlaku setelah restart. Service menolak start bila berkasnya rusak, alih-alih menimpanya.

Middleware `X-Api-Key` (`src/auth.ts`) menghitung SHA-256 nilai yang dikirim, lalu membandingkannya dengan `crypto.timingSafeEqual` terhadap hash kredensial service dan hash setiap mirrored key aktif. Jenis kredensial (`service` atau `mirrored`) dicatat di konteks request dan di log. Nilai kredensial dan key tidak pernah ditulis ke log.

## Backend agen tidak memanggil service ini

Backend agen [`../agent/backend`](../agent/backend/README.md) bertindak atas nama user lewat token exchange, sehingga token yang dibawanya berprincipal user. Service `api_key` dengan kunci per Service Account menolak token seperti itu dengan 403 `service_account_required`, jadi agen tidak memanggil `pengiriman`. Contoh agen hanya memakai `inventaris` (service `nice_token`, yang mendukung principal user dengan aktor `acted_by`).

## Prasyarat

- Node.js 22.18 atau lebih baru (menjalankan `.ts` langsung lewat type stripping, tanpa build) dan npm.
- `curl` dan `jq` untuk checklist dan skrip smoke.
- NICE dev yang berjalan (`make dev` di `nice/`, bawaan `http://localhost:8080`), revisi `40ddb20` atau sesudahnya (revisi pertama yang mewajibkan operasi key), dengan repo `nice/` berada di sebelah `example/`. Untuk NICE di alamat lain, isi `NICE_URL` di `.env`; checklist dan uji smoke membacanya dari sana.

## Konfigurasi

Salin `.env.example` ke `.env`. Service membaca variabel lingkungan; `npm run dev` dan `npm start` memuat `.env` lewat `node --env-file-if-exists=.env`, dan variabel yang sudah ada di lingkungan tidak ditimpa.

| Variabel | Bawaan | Arti |
|---|---|---|
| `SERVICE_CREDENTIAL` | (wajib) | Kredensial yang dikirim NICE di `X-Api-Key`. Buat dengan `openssl rand -hex 32`. Tanpa nilai ini service berhenti saat start dengan pesan yang menyebut nama variabelnya |
| `LISTEN_ADDR` | `127.0.0.1:8082` | Alamat listen `host:port` |
| `KEYS_FILE` | `data/keys.json` | Berkas mirrored key (hanya hash) |
| `NICE_URL` | `http://localhost:8080` | Alamat NICE, hanya dipakai `scripts/smoke.sh` |
| `SMOKE_SA_ID` | | ID Service Account tenant (`sa_...`) untuk smoke, dari langkah 9 |
| `SMOKE_SA_API_KEY` | | API key Service Account itu (`nice_sk_...`), dari langkah 11 |

`.env` dan `data/` tidak di-commit.

## Menjalankan

```bash
cd example/shared-service-apikey
npm install
cp .env.example .env            # lalu isi SERVICE_CREDENTIAL dengan hasil: openssl rand -hex 32
npm run dev                     # node --watch; atau npm start tanpa watch
curl -s http://127.0.0.1:8082/healthz        # {"status":"ok"}
curl -s http://127.0.0.1:8082/openapi.json | jq '.paths | keys'
```

## Menjalankan di container

`Dockerfile` membangun image Node 22 alpine berisi dependensi runtime saja (tanpa build, `.ts` dijalankan lewat type stripping) yang berjalan sebagai pengguna `node`. Bawaan image: `LISTEN_ADDR=0.0.0.0:8082`, `KEYS_FILE=/data/keys.json` di volume `/data`, dan `HEALTHCHECK` ke `/healthz`.

```bash
cd example/shared-service-apikey
docker build -t pengiriman .
docker volume create pengiriman-data
export SERVICE_CREDENTIAL         # nilainya dari .env; tidak ditulis di argumen perintah
docker run -d --name pengiriman --restart unless-stopped --network <jaringan-NICE> \
  -e SERVICE_CREDENTIAL -v pengiriman-data:/data pengiriman
```

- **Service harus tetap privat.** Hanya proxy NICE yang boleh menjangkaunya (lihat [Peringatan](#peringatan-header-x-nice-)). Jangan memetakan port ke antarmuka publik (`-p 8082:8082`) dan jangan mempublikasikannya lewat Cloudflare Tunnel. Jalankan di jaringan Docker yang sama dengan NICE, daftarkan endpoint dengan nama container (mis. `http://pengiriman:8082`) di langkah 4, impor dari `http://pengiriman:8082/openapi.json` di langkah 5, dan izinkan alamat jaringan itu di `NICE_OUTBOUND_ALLOW_CIDRS` (langkah 1).
- **Volume data harus bertahan.** `keys.json` menyimpan hash setiap mirrored key. Bila hilang, service menolak semua key yang dipegang NICE dengan 401, sehingga setiap panggilan Service Account lewat proxy gagal. Pakai volume bernama atau bind mount yang dicadangkan, bukan volume anonim dari `docker run --rm`.
- Jangan memakai `--env-file .env`: `LISTEN_ADDR=127.0.0.1:8082` di `.env` dev membuat service tidak terjangkau dari luar container.

## Tes

```bash
npm test             # Vitest, memanggil app.request() tanpa jaringan dan tanpa NICE
npm run typecheck    # tsc --noEmit
```

Tes membuat berkas key di direktori sementara per tes dan membangun ulang aplikasi dari berkas yang sama untuk meniru restart.

## Checklist setup NICE (manual)

Semua langkah di sisi NICE dilakukan manual, lewat NICE Console atau `curl`. Potongan `curl` di bawah dijalankan dari direktori contoh ini, dalam satu sesi shell:

```bash
cd example/shared-service-apikey
set -a; . ./.env; set +a          # SERVICE_CREDENTIAL dan NICE_URL dari .env
NICE=$NICE_URL
```

### 1. Env NICE

Tambahkan ke `nice/.env`, lalu jalankan ulang NICE (`make dev`):

```bash
NICE_OUTBOUND_ALLOW_HTTP=true
NICE_OUTBOUND_ALLOW_CIDRS=127.0.0.0/8
```

Tanpa keduanya, NICE menolak endpoint `http://` dan alamat loopback saat registrasi, impor, probe health, proxy, dan operasi key. `NICE_OUTBOUND_ALLOW_CIDRS` berisi daftar CIDR yang dipisah koma.

### 2. Jalankan service

Ikuti bagian [Menjalankan](#menjalankan). Service harus berjalan sebelum langkah 4, karena NICE memeriksa endpoint dan mengambil `/openapi.json` darinya.

### 3. Token admin

Ambil dua access token dengan `nice/scripts/dev-token.sh`: satu untuk akun Platform Admin dev Anda (langkah katalog dan entitlement), satu untuk owner atau admin organisasi tenant yang akan memakai service (langkah project). Kata sandi dibaca tanpa ditampilkan dan tidak masuk riwayat shell. Bila akun memakai verifikasi dua langkah, isi `TOTP_SECRET` (lihat komentar di skrip itu).

```bash
read -rp 'Email Platform Admin: ' ADMIN_EMAIL; read -rsp 'Kata sandi: ' ADMIN_PASSWORD; echo
ADMIN_TOKEN=$(NICE=$NICE ../../nice/scripts/dev-token.sh "$ADMIN_EMAIL" "$ADMIN_PASSWORD")
read -rp 'Email owner organisasi tenant: ' OWNER_EMAIL; read -rsp 'Kata sandi: ' OWNER_PASSWORD; echo
OWNER_TOKEN=$(NICE=$NICE ../../nice/scripts/dev-token.sh "$OWNER_EMAIL" "$OWNER_PASSWORD")
unset ADMIN_PASSWORD OWNER_PASSWORD

ORG_ID=$(curl -sS "$NICE/api/v1/me/organization" -H "Authorization: Bearer $OWNER_TOKEN" | jq -r .organization.id)
curl -sS "$NICE/api/v1/organizations/$ORG_ID/projects" -H "Authorization: Bearer $OWNER_TOKEN" | jq '.projects[] | {id, slug, name}'
PROJECT_ID=prj_...                # pilih project tenant dari daftar di atas
```

Token berlaku 30 menit. Bila `curl` berikutnya dijawab 401, ambil token lagi.

### 4. Daftarkan service `pengiriman`

Console: **Platform Admin > Shared Service**, daftarkan service baru dengan mode `api_key`. Wizard memuat bagian **Operasi kunci upstream** di langkah Informasi, sudah terisi usulan bawaan yang sama dengan tabel Operasi di atas (`POST /keys`, `POST /keys/{key_id}/rotate`, `DELETE /keys/{key_id}`). Atau dengan `curl` (nilai kredensial diambil `jq` dari lingkungan, tidak muncul di argumen perintah):

```bash
jq -n '{slug: "pengiriman", name: "Pengiriman", endpoint: "http://127.0.0.1:8082", health_path: "/healthz",
        auth_mode: "api_key",
        upstream: {credential: {header: "X-Api-Key", value: env.SERVICE_CREDENTIAL},
                   key_operations: {generate: {method: "POST", path: "/keys"},
                                    rotate: {method: "POST", path: "/keys/{key_id}/rotate"},
                                    revoke: {method: "DELETE", path: "/keys/{key_id}"}}}}' |
  curl -sS -X POST "$NICE/api/v1/shared-services" -H "Authorization: Bearer $ADMIN_TOKEN" \
    -H 'Content-Type: application/json' -d @- | jq '.shared_service | {slug, auth_mode, status, credential}'
```

- `endpoint` ditulis `http://127.0.0.1:8082`, bukan `localhost`. `localhost` bisa teresolusi ke `::1`, yang tidak didengarkan service dan tidak termasuk `127.0.0.0/8`.
- `base_path` dikosongkan, sehingga `<NICE>/pengiriman/shipments` diteruskan ke `http://127.0.0.1:8082/shipments`.
- `upstream.credential`: `header` `X-Api-Key`, **tanpa** `scheme`, dan `value` sama persis dengan `SERVICE_CREDENTIAL`. Nilainya tulis-saja: respons hanya menampilkan `credential.header` dan `credential.configured`. Untuk menggantinya pakai `PUT /api/v1/shared-services/pengiriman/upstream/credential`.
- `upstream.key_operations` wajib memuat ketiganya (`generate`, `rotate`, `revoke`, masing-masing `method` dan `path`); tanpa itu registrasi ditolak 400 `invalid_key_operations`, dan aktivasi (langkah 6) ditolak 400 `key_operations_required`. Path relatif terhadap `endpoint` + `base_path`, dan NICE mengganti `{key_id}` dengan id key. Untuk mengganti operasi sesudahnya pakai `PUT /api/v1/shared-services/pengiriman/upstream/key-operations` (butuh `nice.platform.catalog.manage`); operasi tidak bisa dikosongkan.
- `health_path` `/healthz` terbuka tanpa kredensial, karena probe NICE tidak membawa kredensial.

### 5. Impor `/openapi.json`

Console: halaman service `pengiriman`, impor katalog dari URL lalu terapkan. Atau pratinjau lalu terapkan dengan `curl`:

```bash
PREVIEW=$(curl -sS -X POST "$NICE/api/v1/shared-services/pengiriman/catalog-imports" \
  -H "Authorization: Bearer $ADMIN_TOKEN" -H 'Content-Type: application/json' \
  -d '{"url":"http://127.0.0.1:8082/openapi.json"}')
jq '{import_id, added: [.diff.added[]? | {key, method, path, permission}], roles: [.diff.roles[]? | {key, permissions}]}' <<<"$PREVIEW"
curl -sS -X POST "$NICE/api/v1/shared-services/pengiriman/catalog-imports/$(jq -r .import_id <<<"$PREVIEW")/apply" \
  -H "Authorization: Bearer $ADMIN_TOKEN" -H 'Content-Type: application/json' \
  -d "$(jq '{hash}' <<<"$PREVIEW")"
```

Pratinjau harus menampilkan tepat tiga operasi: `listShipments` (`pengiriman.shipments.read`), `createShipment` (`pengiriman.shipments.create`), dan `cancelShipment` (`pengiriman.shipments.cancel`), serta role `pengiriman.viewer`, `pengiriman.admin`, dan `pengiriman.operator`. Role `pengiriman.operator` diturunkan NICE (sejak `nice` `b65a3e3`) untuk setiap service selain `nice`: berscope platform, berisi semua izin aktif service, dan hanya bisa diberikan superadmin lewat `POST /api/v1/admin/role-bindings`. Contoh ini tidak memakainya.

### 6. Aktifkan service

Console: halaman service `pengiriman`, aktifkan. Atau:

```bash
curl -sS -X POST "$NICE/api/v1/shared-services/pengiriman/activate" -H "Authorization: Bearer $ADMIN_TOKEN" | jq '.shared_service | {slug, status}'
```

### 7. Entitlement organisasi

Console: **Platform Admin > Organisasi**, pilih organisasi tenant, beri entitlement `pengiriman`. Atau:

```bash
curl -sS -X POST "$NICE/api/v1/organizations/$ORG_ID/entitlements" -H "Authorization: Bearer $ADMIN_TOKEN" \
  -H 'Content-Type: application/json' -d '{"service":"pengiriman"}'
```

### 8. Aktifkan di project

Console (sebagai owner tenant): **Project > Product & Activation**, aktifkan `pengiriman`. Atau:

```bash
curl -sS -X PUT "$NICE/api/v1/projects/$PROJECT_ID/services/pengiriman/activation" -H "Authorization: Bearer $OWNER_TOKEN"
```

### 9. Service Account tenant

Console: **Project > Service Account**, buat Service Account. Atau:

```bash
SA_ID=$(curl -sS -X POST "$NICE/api/v1/projects/$PROJECT_ID/service-accounts" -H "Authorization: Bearer $OWNER_TOKEN" \
  -H 'Content-Type: application/json' -d '{"name":"pengiriman-smoke","description":"Smoke test contoh pengiriman"}' |
  jq -r .service_account.id)
echo "$SA_ID"
```

Isi `SMOKE_SA_ID` di `.env` dengan nilai itu.

### 10. Role binding

Console: **Project > IAM**, beri Service Account itu role `pengiriman.admin` (atau `pengiriman.viewer`, yang cukup untuk smoke karena smoke hanya memanggil `GET /shipments`). Atau:

```bash
curl -sS -X POST "$NICE/api/v1/projects/$PROJECT_ID/role-bindings" -H "Authorization: Bearer $OWNER_TOKEN" \
  -H 'Content-Type: application/json' \
  -d "{\"principal_type\":\"PRINCIPAL_TYPE_SERVICE_ACCOUNT\",\"principal_id\":\"$SA_ID\",\"role\":\"pengiriman.admin\"}"
```

IAM mengetahui Service Account baru secara asinkron. Binding yang dibuat tepat setelah langkah 9 bisa dijawab 400 `principal_not_in_org`, dan tepat setelah langkah 8 400 `service_not_active`; tunggu beberapa detik lalu ulangi.

### 11. API key Service Account

Console: halaman Service Account tadi, buat API key untuk Service Account itu (hak ditentukan role-nya). Atau:

```bash
curl -sS -X POST "$NICE/api/v1/projects/$PROJECT_ID/api-keys" -H "Authorization: Bearer $OWNER_TOKEN" \
  -H 'Content-Type: application/json' \
  -d "{\"service_account_id\":\"$SA_ID\",\"name\":\"pengiriman-smoke\"}" |
  jq '{id: .api_key.id, key}'
```

`key` hanya ditampilkan sekali. Simpan ke `SMOKE_SA_API_KEY` di `.env`. Key ini ditukar menjadi token di `/oauth/token` dan tidak pernah dikirim ke service.

### 12. Periksa kunci Service Account

Tidak ada yang dikonfigurasi di langkah ini: operasi key sudah didaftarkan di langkah 4. Karena itu NICE tidak memakai `SERVICE_CREDENTIAL` untuk lalu lintas klien. NICE membuat satu kunci per Service Account yang memegang izin `pengiriman` lewat `POST /keys`, menyimpannya terbungkus KEK, dan proxy mengirim kunci milik Service Account pemanggil di `X-Api-Key`. `SERVICE_CREDENTIAL` tetap dipakai, tetapi hanya oleh NICE untuk memanggil ketiga operasi key.

**Akibatnya, hanya Service Account yang bisa memanggil `pengiriman` lewat proxy.** User NICE dijawab 403 `service_account_required`, dan Service Account yang kuncinya belum aktif dijawab 503 `upstream_key_unavailable`.

NICE membuat kunci secara asinkron: kunci dibuat saat Service Account pertama kali memperoleh izin `pengiriman` (langkah 10), lewat sapuan sekitar 30 detik. Karena itu 503 `upstream_key_unavailable` tepat setelah langkah 10 adalah keadaan sementara. Periksa hasilnya:

- Daftar kunci Service Account (butuh `nice.platform.upstreamkeys.manage`, hanya metadata tanpa nilai kunci). Console: **Platform Admin > Kunci Service Account**. Atau:

  ```bash
  curl -sS "$NICE/api/v1/service-account-keys?service=pengiriman" -H "Authorization: Bearer $ADMIN_TOKEN" |
    jq '.keys[] | {service_account_id, status, last_error}'
  ```

  Kunci Service Account dari langkah 9 harus berstatus `active`. Status `pending` berarti NICE belum selesai membuatnya, dan `failed` berarti panggilan ke service gagal (lihat `last_error`); NICE mengulang dengan backoff sampai 10 menit.
- Log service memuat `key generated key_id=key_... label=<uuid>`. Label berisi id Service Account tanpa awalan `sa_`.
- `jq . data/keys.json` menampilkan kunci dengan `label` sama dengan `SA_ID` tanpa `sa_` dan `status` `active`, tanpa nilai kunci.
- `scripts/smoke.sh` lulus, dan log service mencatat `credential=mirrored` untuk request lewat proxy.

Catatan:

- Jangan hapus `data/keys.json`. Kunci yang dipegang NICE tidak akan diterima lagi, dan proxy meneruskan 401 dari service.

## Uji smoke

Setelah langkah 1 sampai 11 dan service berjalan (langkah 12 diperiksa dengan smoke ini):

```bash
scripts/smoke.sh
```

Skrip hanya membaca `.env`, tidak mencetak kredensial, API key, atau token, dan memeriksa:

1. `GET http://127.0.0.1:8082/shipments` langsung tanpa `X-Api-Key` dijawab 401;
2. NICE terjangkau di `NICE_URL`;
3. `GET <NICE>/pengiriman/shipments` tanpa token dijawab 401 oleh proxy;
4. token Service Account didapat lewat `client_credentials` dengan `scope=nice:aud:pengiriman`;
5. `GET <NICE>/pengiriman/shipments` dengan token itu dijawab 200. NICE membuat kunci Service Account secara asinkron, jadi bila proxy menjawab 503 `upstream_key_unavailable`, skrip mengulang panggilan hingga 12 kali dengan jeda 5 detik (sekitar 60 detik) sebelum gagal.

```text
ok    langsung ke service tanpa key: 401
ok    proxy tanpa token: 401
ok    token Service Account (scope nice:aud:pengiriman)
ok    GET http://localhost:8080/pengiriman/shipments lewat proxy: 200
smoke lulus
```

Bila gagal, skrip keluar dengan kode bukan 0 dan menyebut langkah checklist yang kemungkinan terlewat:

| Gejala | Langkah |
|---|---|
| `.env` tidak ada, atau service tidak menjawab | 2 |
| NICE tidak terjangkau | 1 |
| proxy 404 tanpa token | 4, 5 |
| proxy 503 `service_disabled` | 6 |
| token `invalid_client` | 9, 11 |
| token `invalid_scope` | 6, 7, 8, 11 |
| proxy 403 `permission_denied` | 10, 11 |
| proxy 401 `{"error":"unauthorized"}` dari service | 4 (kredensial berbeda), 12 (kunci tidak ada di `data/keys.json`) |
| proxy 503 `upstream_key_unavailable` setelah semua percobaan | 12 |
| proxy 502 atau 504 | 1, 2, 4 |

Memanggil operasi lain lewat proxy dengan token Service Account:

```bash
set -a; . ./.env; set +a         # memuat SMOKE_SA_ID dan SMOKE_SA_API_KEY yang baru diisi
TOKEN=$(curl -sS "$NICE/oauth/token" -d grant_type=client_credentials -d scope=nice:aud:pengiriman \
  --data-urlencode "client_id=$SMOKE_SA_ID" --data-urlencode client_secret@- \
  < <(printf '%s' "$SMOKE_SA_API_KEY") | jq -r .access_token)
curl -sS -X POST "$NICE/pengiriman/shipments" -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' -d '{"destination":"Gudang Timur, Surabaya"}'
curl -sS -X POST "$NICE/pengiriman/shipments/<id>/cancel" -H "Authorization: Bearer $TOKEN"
```

## Berkas

| Berkas | Isi |
|---|---|
| `src/index.ts` | titik masuk: konfigurasi, berkas key, listen |
| `src/config.ts` | pembacaan dan validasi env |
| `src/auth.ts` | middleware `X-Api-Key`, penolakan mirrored key di operasi key, header tenant |
| `src/keys.ts` | penyimpanan mirrored key (hash, penulisan atomik) |
| `src/shipments.ts` | rute pengiriman dengan `createRoute` dan `x-nice-permission` |
| `src/app.ts` | perakitan aplikasi, operasi key, `/healthz`, `/openapi.json` |
| `scripts/smoke.sh` | uji smoke terhadap NICE |
| `Dockerfile` | image container (lihat [Menjalankan di container](#menjalankan-di-container)) |
