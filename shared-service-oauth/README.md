# shared-service-oauth (`inventaris`)

[English](README.en.md)

Contoh SharedService NICE bermode `nice_token` dengan slug `inventaris`, ditulis dalam Go (`net/http`) memakai pustaka rujukan `nice/pkg/nicetoken`. Klien memanggil service ini langsung dengan access token NICE; NICE tidak berada di jalur request. Service memverifikasi token sendiri sesuai [kontrak verifikasi token](../../nice/docs/token-verification.md), memisahkan data per organisasi, dan memeriksa operasi sensitif secara online ke NICE.

Diuji terhadap nice revisi: `c7a5086` (2026-11-13)

## Isi

| Berkas | Isi |
|---|---|
| `main.go` | wiring `nicetoken` (`New`, `Run`, `StartFeed`), anotasi swag tingkat dokumen termasuk `x-nice-roles` |
| `routes.go` | tabel rute tunggal (method, path, izin, sensitif) dan rantai handler `CORS -> mux` |
| `items.go` | handler barang, validasi, bentuk galat, anotasi swag per operasi |
| `store.go` | penyimpanan barang per organisasi di memori |
| `config.go` | pembacaan env dan `.env` |
| `cors.go` | CORS untuk SPA yang terdaftar |
| `docs/swagger.json` | Swagger 2.0 hasil `make swag`, di-embed dan disajikan di `/openapi.json` |
| `scripts/smoke.sh` | uji smoke terhadap NICE yang sudah diatur |

## Operasi

| Operasi | Izin | Sensitif |
|---|---|---|
| `GET /items` | `inventaris.items.read` | |
| `GET /items/{id}` | `inventaris.items.read` | |
| `POST /items` | `inventaris.items.create` | |
| `PUT /items/{id}` | `inventaris.items.edit` | |
| `DELETE /items/{id}` | `inventaris.items.delete` | ya, diperiksa online ke NICE |

`GET /healthz` dan `GET /openapi.json` terbuka tanpa token dan tidak ada di dokumen OpenAPI. Setelah impor, katalog NICE memuat role `inventaris.viewer` (baca), `inventaris.admin` (semua izin), dan `inventaris.operator` (baca, buat, ubah; dari `x-nice-roles`).

Body barang: `{"name": "...", "quantity": 0}`. `name` 1 sampai 120 karakter setelah spasi di tepi dipangkas, `quantity` bilangan bulat paling kecil 0. Galat aplikasi memakai bentuk yang sama dengan `nicetoken`: `{"error": "...", "error_description": "..."}`, mis. `400 {"error":"invalid_argument","error_description":"name"}` dan `404 {"error":"not_found"}`.

Perilaku yang perlu diketahui:

- Tenant adalah `nice_org_id` dari token terverifikasi, tidak pernah dari header, parameter, atau body. Organisasi yang memanggil pertama kali langsung dibuatkan tenant (`ProvisionOnFirstUse`). Data hanya di memori dan hilang saat service berhenti.
- Salinan JWKS disimpan di `JWKS_FILE`, sehingga service yang restart ketika NICE mati tetap memverifikasi token yang masih berlaku.
- Tanpa `NICE_SERVICE_API_KEY`, service tetap berjalan dan mencatat peringatan saat start: feed perubahan akses mati (pencabutan baru berlaku saat token kedaluwarsa, paling lama 30 menit) dan `DELETE /items/{id}` dijawab `503`.

## Prasyarat

- Susunan direktori: `replace github.com/nice-platform/nice => ../../nice` di `go.mod` hanya berjalan bila repo `nice` berada di sebelah `example/`:

  ```text
  <induk>/
    nice/
    example/
      shared-service-oauth/
  ```

- Go 1.27, `curl`, `jq`, dan `openssl` (untuk `nice/scripts/dev-token.sh`).
- NICE dev berjalan, bawaannya di `http://localhost:8080` (`cd ../../nice && make db && make migrate && make dev`). NICE di alamat lain juga bisa: isikan ke `NICE_ISSUER` di `.env` dan ke `NICE` di shell checklist.

## Konfigurasi

Salin `.env.example` ke `.env`. Service membaca `.env` di direktori tempat ia dijalankan; variabel di environment menang atas berkas.

| Variabel | Bawaan | Arti |
|---|---|---|
| `NICE_ISSUER` | wajib | issuer NICE, sama persis dengan klaim `iss` token. Tanpa nilai ini service berhenti dengan pesan yang menyebut `NICE_ISSUER` |
| `LISTEN_ADDR` | `127.0.0.1:8081` | alamat listen |
| `JWKS_FILE` | `data/jwks.json` | salinan JWKS di disk (`data/` tidak di-commit) |
| `CORS_ALLOWED_ORIGINS` | `http://localhost:5180` | origin browser yang boleh memanggil, dipisah koma. `.env.example` mengisinya `http://localhost:5180,http://localhost:5181`, untuk `public-client-pkce` dan `service-client-ui` |
| `NICE_SERVICE_API_KEY` | kosong | API key Service Account platform berperan `nice.service` (langkah 9); menyalakan feed dan pemeriksaan online |
| `SMOKE_SA_ID`, `SMOKE_API_KEY` | kosong | hanya dibaca `scripts/smoke.sh`: Service Account tenant dan API key-nya (langkah 10) |

## Menjalankan dan tes

```bash
make run          # go run . di 127.0.0.1:8081
make test         # tes unit; tidak butuh NICE (token ditandatangani di tes, JWKS statis, pemeriksa palsu)
make swag         # bangkitkan ulang docs/swagger.json dari anotasi
make swag-check   # gagal bila docs/swagger.json tidak sama dengan hasil swag terbaru
scripts/smoke.sh  # butuh NICE yang sudah diatur dengan checklist di bawah
```

Bila `go` di `PATH` tidak cocok dengan `GOROOT`, panggil `make GO=/path/ke/go test`.

Tes `TestOpenAPIMatchesTheRouteTable` membandingkan tabel rute di `routes.go` dengan `docs/swagger.json` yang di-embed: izin dan tanda sensitif harus sama, dan dokumen tidak boleh memuat operasi lain. Setelah mengubah anotasi, jalankan `make swag` lalu commit hasilnya.

## Checklist setup NICE

Jalankan dari direktori `example/shared-service-oauth`. Langkah console disebut dengan nama halamannya; potongan `curl` adalah padanannya.

Sebelum mulai, siapkan variabel shell. Token diambil dengan `nice/scripts/dev-token.sh` (client `nice-console` dengan redirect bawaan NICE dev `http://localhost:5173/callback`; bila akun memakai verifikasi dua langkah, set `TOTP_SECRET`, lihat kepala skrip). Token berlaku 30 menit; ulangi bila kedaluwarsa.

```bash
export NICE=${NICE:-http://localhost:8080}   # alamat NICE, sama dengan NICE_ISSUER di .env; dev-token.sh juga membacanya

# akun Platform Admin dev Anda
read -rp 'Email Platform Admin: ' ADMIN_EMAIL; read -rsp 'Password: ' ADMIN_PASSWORD; echo
ADMIN_TOKEN=$(../../nice/scripts/dev-token.sh "$ADMIN_EMAIL" "$ADMIN_PASSWORD"); unset ADMIN_PASSWORD

# akun owner organisasi tenant dev Anda
read -rp 'Email owner organisasi tenant: ' OWNER_EMAIL; read -rsp 'Password: ' OWNER_PASSWORD; echo
OWNER_TOKEN=$(../../nice/scripts/dev-token.sh "$OWNER_EMAIL" "$OWNER_PASSWORD"); unset OWNER_PASSWORD

ORG_ID=$(curl -sS "$NICE/api/v1/me/organization" -H "Authorization: Bearer $OWNER_TOKEN" | jq -r .organization.id)
curl -sS "$NICE/api/v1/organizations/$ORG_ID/projects" -H "Authorization: Bearer $OWNER_TOKEN" | jq '.projects[] | {id, name}'
PROJECT_ID=prj_...   # project tenant yang akan memakai inventaris
USER_ID=$(curl -sS "$NICE/api/v1/me/profile" -H "Authorization: Bearer $OWNER_TOKEN" | jq -r .profile.user_id)
```

### 1. Env NICE

Tambahkan ke `nice/.env`, lalu jalankan ulang NICE. Tanpa ini NICE menolak endpoint `http` dan alamat loopback.

```text
NICE_OUTBOUND_ALLOW_HTTP=true
NICE_OUTBOUND_ALLOW_CIDRS=127.0.0.0/8
```

`NICE_OUTBOUND_ALLOW_CIDRS` berupa daftar CIDR dipisah koma. Service ini tidak butuh `NICE_CORS_ALLOWED_ORIGINS`. Contoh SPA (`public-client-pkce` di 5180, `service-client-ui` di 5181) juga tidak, pada NICE yang memuat `add-delegated-nice-api`: NICE mengizinkan origin redirect URI setiap OAuth client `public`.

### 2. Jalankan service

```bash
cp .env.example .env
make run
curl -sS http://127.0.0.1:8081/healthz
```

Biarkan berjalan di terminal lain. Kunci di `.env` diisi pada langkah 9 dan 10.

### 3. Daftarkan `inventaris`

Console: Platform Admin, **Shared Service**, **Daftarkan Shared Service**. Endpoint ditulis `http://127.0.0.1:8081`, bukan `localhost`, supaya NICE tidak pernah mencoba `::1`. `auth_mode` tidak perlu dikirim; bawaannya `nice_token`.

```bash
curl -sS "$NICE/api/v1/shared-services" -X POST \
  -H "Authorization: Bearer $ADMIN_TOKEN" -H 'Content-Type: application/json' \
  -d '{"slug":"inventaris","name":"Inventaris","endpoint":"http://127.0.0.1:8081","health_path":"/healthz"}'
```

### 4. Impor `/openapi.json`

Console: detail service `inventaris` di **Shared Service**, impor katalog (pratinjau lalu terapkan).

```bash
DOCUMENT=$(curl -sS http://127.0.0.1:8081/openapi.json | jq -Rs .)
PREVIEW=$(curl -sS "$NICE/api/v1/shared-services/inventaris/catalog-imports" -X POST \
  -H "Authorization: Bearer $ADMIN_TOKEN" -H 'Content-Type: application/json' \
  -d "{\"document\":$DOCUMENT}")
printf '%s' "$PREVIEW" | jq '{added: [.diff.added[] | {key, permission, sensitive}], roles: [.diff.roles[] | .key]}'
IMPORT_ID=$(printf '%s' "$PREVIEW" | jq -r .import_id)
HASH=$(printf '%s' "$PREVIEW" | jq -r .hash)
curl -sS "$NICE/api/v1/shared-services/inventaris/catalog-imports/$IMPORT_ID/apply" -X POST \
  -H "Authorization: Bearer $ADMIN_TOKEN" -H 'Content-Type: application/json' \
  -d "{\"hash\":\"$HASH\"}"
```

Pratinjau harus menunjukkan lima operasi, empat izin (`deleteItem` sensitif), serta role `inventaris.viewer`, `inventaris.admin`, dan `inventaris.operator`.

### 5. Aktifkan service

```bash
curl -sS "$NICE/api/v1/shared-services/inventaris/activate" -X POST -H "Authorization: Bearer $ADMIN_TOKEN"
```

### 6. Entitlement organisasi

Console: Platform Admin, **Organisasi**, detail organisasi tenant.

```bash
curl -sS "$NICE/api/v1/organizations/$ORG_ID/entitlements" -X POST \
  -H "Authorization: Bearer $ADMIN_TOKEN" -H 'Content-Type: application/json' -d '{"service":"inventaris"}'
```

### 7. Aktifkan di project

Console: project tenant, **Product & Activation**.

```bash
curl -sS "$NICE/api/v1/projects/$PROJECT_ID/services/inventaris/activation" -X PUT -H "Authorization: Bearer $OWNER_TOKEN"
```

### 8. Binding user ke role inventaris

Console: project tenant, **IAM**. IAM menyalin katalog dan aktivasi secara asinkron: bila dijawab `unknown_role` atau `service_not_active`, tunggu beberapa detik lalu ulangi. `echo "$BINDING_ID"` harus mencetak `rb_...`, bukan `null`.

```bash
BINDING_ID=$(curl -sS "$NICE/api/v1/projects/$PROJECT_ID/role-bindings" -X POST \
  -H "Authorization: Bearer $OWNER_TOKEN" -H 'Content-Type: application/json' \
  -d "{\"principal_type\":\"PRINCIPAL_TYPE_USER\",\"principal_id\":\"$USER_ID\",\"role\":\"inventaris.operator\"}" | jq -r .role_binding.id)
```

`inventaris.operator` tidak memuat `inventaris.items.delete`; pakai `inventaris.admin` untuk mencoba hapus barang.

### 9. Service Account platform `nice.service` (feed dan pemeriksaan online)

Platform Admin membuat Service Account di project organisasi platform (buat project bila belum ada), memberinya role `nice.service`, lalu API key dengan `allowed_services` berisi `nice`. Console: **Service Account** di project organisasi platform, lalu **Pengguna platform** untuk role `nice.service`.

```bash
PLATFORM_ORG_ID=$(curl -sS "$NICE/api/v1/me/organization" -H "Authorization: Bearer $ADMIN_TOKEN" | jq -r .organization.id)
PLATFORM_PROJECT_ID=$(curl -sS "$NICE/api/v1/organizations/$PLATFORM_ORG_ID/projects" -X POST \
  -H "Authorization: Bearer $ADMIN_TOKEN" -H 'Content-Type: application/json' \
  -d '{"slug":"service-readers","name":"Service readers"}' | jq -r .project.id)
FEED_SA_ID=$(curl -sS "$NICE/api/v1/projects/$PLATFORM_PROJECT_ID/service-accounts" -X POST \
  -H "Authorization: Bearer $ADMIN_TOKEN" -H 'Content-Type: application/json' \
  -d '{"name":"inventaris-feed","description":"Feed perubahan akses dan pemeriksaan online inventaris"}' | jq -r .service_account.id)
curl -sS "$NICE/api/v1/admin/role-bindings" -X POST \
  -H "Authorization: Bearer $ADMIN_TOKEN" -H 'Content-Type: application/json' \
  -d "{\"principal_type\":\"PRINCIPAL_TYPE_SERVICE_ACCOUNT\",\"principal_id\":\"$FEED_SA_ID\",\"role\":\"nice.service\"}"
curl -sS "$NICE/api/v1/projects/$PLATFORM_PROJECT_ID/api-keys" -X POST \
  -H "Authorization: Bearer $ADMIN_TOKEN" -H 'Content-Type: application/json' \
  -d "{\"service_account_id\":\"$FEED_SA_ID\",\"name\":\"inventaris-feed\",\"allowed_services\":[\"nice\"]}" | jq -r .key
```

Key hanya tampil sekali. Isikan ke `NICE_SERVICE_API_KEY` di `.env`, lalu jalankan ulang `make run`; peringatan feed saat start hilang.

IAM mengetahui Service Account baru secara asinkron, jadi binding yang dibuat tepat setelah akunnya dibuat bisa dijawab 400 `principal_not_in_org`. Tunggu beberapa detik lalu ulangi binding-nya; hal yang sama berlaku di langkah 10.

### 10. Service Account tenant untuk smoke

Console: project tenant, **Service Account** (buat akun dan API key) dan **IAM** (binding).

```bash
SMOKE_SA_ID=$(curl -sS "$NICE/api/v1/projects/$PROJECT_ID/service-accounts" -X POST \
  -H "Authorization: Bearer $OWNER_TOKEN" -H 'Content-Type: application/json' \
  -d '{"name":"inventaris-smoke"}' | jq -r .service_account.id)
curl -sS "$NICE/api/v1/projects/$PROJECT_ID/role-bindings" -X POST \
  -H "Authorization: Bearer $OWNER_TOKEN" -H 'Content-Type: application/json' \
  -d "{\"principal_type\":\"PRINCIPAL_TYPE_SERVICE_ACCOUNT\",\"principal_id\":\"$SMOKE_SA_ID\",\"role\":\"inventaris.viewer\"}"
curl -sS "$NICE/api/v1/projects/$PROJECT_ID/api-keys" -X POST \
  -H "Authorization: Bearer $OWNER_TOKEN" -H 'Content-Type: application/json' \
  -d "{\"service_account_id\":\"$SMOKE_SA_ID\",\"name\":\"inventaris-smoke\",\"allowed_services\":[\"inventaris\"]}" | jq -r .key
echo "$SMOKE_SA_ID"
```

Isikan id Service Account ke `SMOKE_SA_ID` dan key ke `SMOKE_API_KEY` di `.env`.

### 11. Smoke

```bash
scripts/smoke.sh
```

Skrip membaca `.env` saja, tidak pernah mencetak key atau token, dan memeriksa: NICE menjawab, `/healthz` menjawab, `GET /items` tanpa token dijawab `401 missing_token`, token Service Account diterbitkan lewat `client_credentials` dengan scope `nice:aud:inventaris` (`client_id` id Service Account, `client_secret` API key), lalu `GET /items` dengan token itu dijawab `200`. Keluar dengan kode 0 bila semuanya lulus. Kegagalan dipetakan ke langkah checklist:

| Jawaban | Langkah yang kemungkinan terlewat |
|---|---|
| `.env` tidak ada, NICE atau service tidak terjangkau | 2 (dan NICE harus berjalan) |
| `invalid_client` | 10: id Service Account bukan pemilik key, key dicabut atau kedaluwarsa, akun nonaktif |
| `invalid_scope` | 3 sampai 7, atau 10 (`allowed_services` harus memuat `inventaris`) |
| `403 permission_denied` dari service | 10: binding Service Account ke role inventaris |

## Uji pencabutan (opsional)

Butuh `NICE_SERVICE_API_KEY` terisi (langkah 9). Token user yang terbit sebelum binding dicabut ditolak `401 token_stale`.

```bash
read -rsp 'Password owner: ' OWNER_PASSWORD; echo
USER_TOKEN=$(../../nice/scripts/dev-token.sh "$OWNER_EMAIL" "$OWNER_PASSWORD" "" "openid nice:aud:inventaris nice:prj:$PROJECT_ID"); unset OWNER_PASSWORD
curl -sS http://127.0.0.1:8081/items -H "Authorization: Bearer $USER_TOKEN"     # 200
sleep 2
curl -sS -X DELETE "$NICE/api/v1/projects/$PROJECT_ID/role-bindings/$BINDING_ID" -H "Authorization: Bearer $OWNER_TOKEN"
sleep 2
curl -sS http://127.0.0.1:8081/items -H "Authorization: Bearer $USER_TOKEN"     # 401 token_stale
```

Jeda sebelum pencabutan dibutuhkan karena feed bekerja dengan resolusi detik: token yang terbit pada detik yang sama dengan pencabutan tidak tertangkap.
