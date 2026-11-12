# Design

## Context

- **Token exchange** (change `nice` `add-token-exchange`):
  - aktornya service client confidential;
  - `subject_token` harus ber-`aud` service ikatan aktor;
  - tujuannya service ikatan atau dependensi, di organisasi milik user;
  - token hasil membawa `act.sub` = `client_id` aktor, dengan `exp` ≤ subjek, tanpa refresh token, dan tanpa rantai.
- **`pkg/niceclient` mode exchange** (`NewExchange`, `DoAs`): cache per `(sub, org, prj, aud)`, singleflight per kunci, satu pengulangan pada `token_stale`. **`pkg/nicetoken`** menyediakan `Claims.Actor`.
- **Proxy `api_key`** meneruskan `X-Nice-Organization/Project/Principal`, tanpa aktor. Bila `upstream/key-operations` diatur, proxy menolak principal selain Service Account (`service_account_required`).
- **SDK MCP Go resmi** (`github.com/modelcontextprotocol/go-sdk`, v1.8.0 tersedia) menyediakan server, transport Streamable HTTP, middleware bearer token di paket `auth`, dan transport dalam memori untuk tes.
- **Contoh `shared-service-oauth`** memakai tabel rute tunggal dan dokumen swag. Barang sudah mencatat `created_by` dari `sub`.

## Goals / Non-Goals

**Goals:**
- Menunjukkan pola OBO yang benar untuk agen MCP: token user hanya ke endpoint token NICE, token tujuan dari exchange, dan jejak `act`.
- Kode tool tidak menyentuh OAuth: satu lapisan `callAs(ctx, target, req)` di atas `DoAs`.
- Galat yang bisa dipahami model AI dan manusia, tanpa membocorkan rahasia.

**Non-Goals:**
- LLM, UI chat, dan konfirmasi manusia (kebijakan aplikasi agen).
- Dynamic Client Registration.
- Tool untuk API NICE (bisa ditambahkan nanti lewat dependensi `nice`).

## Decisions

### 1. Service `agen` sebagai resource server MCP

Agen didaftarkan sebagai service `nice_token` bernama `agen`, dengan dokumen OpenAPI berisi satu operasi `POST /mcp` (`x-nice-permission: agen.mcp.use`). Akibatnya:
- token user untuk agen ber-`aud=agen` dan membawa izin memakai agen, sehingga organisasi bisa mengatur siapa yang boleh memakai agen lewat role;
- `subject_token` exchange memenuhi syarat audience = service ikatan aktor.

Middleware: `nicetoken.Middleware` + `Require("agen.mcp.use")` di depan handler Streamable HTTP. Resource metadata ditulis tangan: `authorization_servers` = issuer NICE, dan `scopes_supported` = scope dasar + `nice:aud:agen`.

Alternatif yang ditolak: middleware `auth` milik SDK MCP saja. Middleware itu memverifikasi token tetapi tidak menegakkan aturan kontrak NICE (deny-set, tenant, izin), jadi `nicetoken` tetap dipakai, dan klaimnya diteruskan ke konteks tool.

### 2. Dua OAuth client terikat `agen`

| Client | Jenis | Kegunaan |
|---|---|---|
| `agen-user` | `public` | user atau MCP client memperoleh token `aud=agen` (dengan `dev-token.sh` atau MCP Inspector) |
| `agen-backend` | confidential | aktor exchange. Secret ada di `.env` agen |

Exchange menolak client `public`, dan memisahkan keduanya juga menunjukkan bahwa secret hanya ada di server.

### 3. Lapisan panggilan

```go
type target struct{ service string; base string }   // inventaris langsung, pengiriman lewat <NICE>/pengiriman
func (a *agent) callAs(ctx context.Context, t target, org, prj string, req *http.Request) (*http.Response, error)
// subjectToken diambil dari konteks permintaan MCP; org/prj bawaan dari klaim token user
```

`callAs` memanggil `niceclient.DoAs` dengan kunci `{Org, Project, Audience}`. `DoAs` sudah menangani cache, singleflight, dan pengulangan `token_stale`. Galat dipetakan ke `CallToolResult{IsError: true}` berisi kode dan penjelasan singkat. Body jawaban service tidak diteruskan mentah, hanya `error` dan `error_description` bila ada.

### 4. Daftar tool

| Tool | Tujuan | Izin dependensi |
|---|---|---|
| `inventaris_list_items` | `GET /items` | `inventaris.items.read` |
| `inventaris_create_item` | `POST /items` | `inventaris.items.create` |
| `pengiriman_list_shipments` | `GET <NICE>/pengiriman/shipments` | `pengiriman.shipments.read` |
| `pengiriman_create_shipment` | `POST <NICE>/pengiriman/shipments` | `pengiriman.shipments.create` |

Skema input memakai struct Go dengan tag JSON schema (SDK membangkitkan skema). Operasi sensitif (hapus barang) sengaja tidak disediakan.

### 5. Penyesuaian `inventaris`

`createItem` membaca `nicetoken.FromContext(ctx).Actor`. Bila tidak kosong, nilainya disimpan sebagai `acted_by` dan dikembalikan bersama barang. Anotasi swag respons barang bertambah field opsional `acted_by`, lalu `make swag` dijalankan. Field ini tidak mengubah operasi atau izin, jadi `make swag-check` dan tes kecocokan tetap berlaku. Tes memakai token uji yang membawa `act`.

### 6. Tes

- **Server MCP:** diuji dengan transport dalam memori SDK. `nicetoken` memakai `StaticJWKS` dengan token uji `aud=agen`.
- **Exchange:** NICE palsu meniru endpoint token exchange (aturan aktor, audience subjek, dependensi, `access_denied` untuk organisasi bukan milik user, `act`, `exp` ≤ subjek). `inventaris` dan proxy `pengiriman` palsu memastikan token yang diterima adalah token hasil exchange, bukan token user.
- **Bukti anti-passthrough:** tes yang gagal bila service palsu menerima token ber-`aud=agen`.

### 7. Port dan modul

Port 8083. Modul `github.com/nice-platform/example/agent-backend` dengan `replace github.com/nice-platform/nice => ../../nice`, Go 1.27. `Makefile` berisi target `run`, `test`, dan `smoke`.

## Risks / Trade-offs

- **[`pengiriman` tidak melihat aktor]** → Proxy NICE hanya meneruskan `X-Nice-Principal`. Dicatat sebagai batas di README, dan bisa menjadi usulan NICE (header `X-Nice-Actor`).
- **[Mirrored key di `pengiriman` menolak token user]** → README menyatakan contoh ini memakai `pengiriman` tanpa langkah 12. Alternatif untuk mirrored key adalah Service Account, yang berada di luar contoh ini.
- **[API SDK MCP berubah antar versi]** → Versi dipin di `go.mod` (v1.8.x) dan dicatat di README.
- **[Contoh bergantung pada empat change NICE]** → Tes otomatis memakai NICE palsu. Uji manual menunggu change NICE digabung, dan README mencatat versi minimum.

## Migration Plan

Tidak ada migrasi. Field `acted_by` di `inventaris` bersifat tambahan dan opsional.

## Penyimpangan saat implementasi

- **Transport tes.** Tes MCP memakai Streamable HTTP di dalam proses (`httptest` dan `StreamableClientTransport` SDK), bukan transport dalam memori. Token user per request hanya sampai ke tool lewat HTTP (`RequestExtra.TokenInfo`), dan transport dalam memori tidak membawanya. Tes tetap tanpa jaringan keluar dan tanpa NICE.
- **Penyerahan token ke tool.** Keputusan tetap `nicetoken.Middleware` + `Require("agen.mcp.use")`. Di belakangnya dipasang `auth.RequireBearerToken` SDK hanya sebagai jembatan: verifier-nya membaca klaim yang sudah diverifikasi `nicetoken` lalu menaruh token mentah dan klaim di `TokenInfo.Extra`, karena kunci konteks `TokenInfo` di SDK tidak diekspor. `UserID` diisi `sub`.
- **Tantangan 401/403.** `nicetoken` menulis `WWW-Authenticate` tanpa `resource_metadata`, jadi `/mcp` dibungkus penulis respons yang menambahkan `resource_metadata="<PUBLIC_URL>/.well-known/oauth-protected-resource"` pada 401, dan `Bearer error="insufficient_scope"` beserta metadata pada 403. Metadata juga disajikan di `/.well-known/oauth-protected-resource/mcp` (bentuk RFC 9728 dengan path).
- **Transport stateless dengan jawaban JSON.** Handler Streamable HTTP memakai `Stateless: true` dan `JSONResponse: true`, sehingga `curl` memanggil tool dalam satu request tanpa `initialize`, dan panggilan paralel tidak berbagi sesi.
- **Bentuk lapisan panggilan.** `target` adalah slug service (`inventaris`, `pengiriman`) dengan peta alamat dasar di agen, bukan struct `{service, base}`. `callAs(ctx, caller, niceclient.Key, req)` menerima kunci yang sudah dibentuk oleh `keyFor`, dan `call` membungkusnya menjadi hasil tool.
- **Konteks bawaan.** Tanpa argumen, exchange hanya mengirim `nice:aud:<service>`, dan NICE memakai organisasi dan project token subjek, alih-alih agen menyalin `nice_org_id` dan `prj` ke scope. Hasilnya sama dan kunci cache `niceclient` menyamakan keduanya. `organization_id` yang sama dengan organisasi token dinormalkan menjadi kosong. Organisasi lain tanpa `project_id` ditolak di agen (`invalid_argument`) sebelum exchange, dan ID yang tidak cocok `org_...`/`prj_...` ditolak supaya model tidak bisa menyisipkan scope lewat spasi.
- **Kode galat nyata.** NICE `6cc9986` menjawab organisasi bukan milik user dengan `400 access_denied` (`error_description` `organization is not usable`), sesuai spec. Penolakan izin di service `nice_token` datang dari `nicetoken` sebagai `403 insufficient_scope` dengan deskripsi `permission_denied`, sedangkan proxy NICE menjawab body `{code, error_code: "permission_denied", message}`. Agen membaca kedua bentuk dan menyebut keduanya di teks galat. Kode tambahan agen: `invalid_argument`, `nice_unavailable`, `service_unavailable`, `exchange_failed`, `bad_answer`.
- **Konfigurasi.** Nama variabel: `NICE_ISSUER`, `NICE_CLIENT_ID`, `NICE_CLIENT_SECRET`, `INVENTARIS_URL` (wajib), `NICE_PROXY_URL` (bawaan `NICE_ISSUER`), `LISTEN_ADDR`, `PUBLIC_URL` (dasar `resource` metadata), dan `JWKS_FILE`. `Config` mengimplementasikan `slog.LogValuer` supaya secret tidak pernah tercetak.
- **Setup NICE yang ditemukan saat integrasi.**
  - Client confidential `agen-backend` tetap wajib punya redirect URI dan grant `authorization_code` (validasi OAuth client NICE), walau tidak pernah login.
  - Impor `openapi.json` menghasilkan role `agen.admin` dan `agen.user`, tanpa `agen.viewer`, karena tidak ada operasi baca.
  - Exchange ke organisasi lain butuh entitlement `agen` dan service tujuan di organisasi itu, tetapi tidak butuh `agen` aktif di project-nya.
  - Audit: exchange sukses tercatat di organisasi tujuan, penolakan di organisasi token user dengan `details.reason` dan `details.target_organization_id`.
- **Tes paralel dan cache (3.2)** sudah hijau saat ditulis, karena perilakunya milik `DoAs`. Kegunaannya dibuktikan dengan mutasi: `callAs` yang diserialkan dengan mutex membuat `TestTwoServicesExchangeAtTheSameTime` gagal.
- **Tes README.** `readme_test.go` menambah penjaga seperti `readme.test.ts` di Hono: variabel di README sama dengan `.env.example`, checklist bernomor urut di kedua bahasa, dan `smoke.sh` hanya merujuk langkah yang ada.
- **`inventaris`.** Selain field `acted_by`, deskripsi operasi `createItem` di `docs/swagger.json` menjelaskan `created_by` dan `acted_by`. Impor ulang katalog `inventaris` hanya menunjukkan perubahan deskripsi.
- **Verifikasi integrasi (grup 6)** dijalankan terhadap NICE terisolasi revisi `6cc9986` (port 8090, Postgres sendiri), bukan NICE dev, dengan service contoh di 8081, 8082, dan 8083. MCP Inspector tidak diuji; jalur `curl`, `scripts/smoke.sh`, dan client Go SDK MCP diuji.
