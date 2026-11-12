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
