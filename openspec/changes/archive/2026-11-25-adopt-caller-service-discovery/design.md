# Design

## Context

Kondisi `agent-backend` saat ini:

- **Tool ditulis tangan:** `nadi_list_devices` dan `nadi_create_device` di `internal/adapter/mcp/tools_routes.go`.
- **Alur panggilan:** use case `callusecase.Service.ExecuteCall` membentuk URL dari `Bases[domain.Nadi]` (`NADI_URL`). `niceexchange.Client.Send` lalu mengirim request lewat `ExchangeClient.DoAs`.
- **Server MCP:** go-sdk `v1.8.0` dalam mode `Stateless: true, JSONResponse: true`.
- **Arsitektur:** `internal/arch_test.go` menegakkan dua aturan:
  - akhiran nama berkas per lapisan;
  - lapisan `app/` tidak boleh mengimpor `net/http`, MCP, atau SDK NICE.

SDK `v0.2.4` menambah tiga method di `ExchangeClient`. Ketiganya memakai token request MCP (`aud=agen`) sebagai token subjek:

| Method | Panggilan ke NICE | Cara kerja |
|---|---|---|
| `Services` | `GET /api/v1/me/services` | Membaca semua halaman. Memakai satu token API NICE terdelegasi per token subjek. |
| `ServiceOperations` | `GET /api/v1/me/services/{slug}/operations` | Memakai token terdelegasi yang sama. |
| `CallOperation` | exchange ke service tujuan, lalu `DoAs` | Memvalidasi parameter path sebelum exchange dan menolak endpoint `http` non-loopback. |

SDK tidak menyimpan peta maupun operasi di cache. Ia juga tidak bertindak atas tanda `Sensitive`.

Perilaku go-sdk `v1.8.0` yang menentukan desain:

- **Mode stateless menolak semua request server→client**, termasuk `ServerSession.Elicit`.
- **Protokol `2026-07-28` punya multi round-trip sessionless (SEP-2322).** Handler tool mengembalikan `CallToolResult{InputRequests, RequestState}` tanpa `Content`. Client memenuhi elicitation, lalu mengulang `tools/call` dengan `inputResponses` dan `requestState`. Handler membacanya dari `req.Params.InputResponses` dan `req.Params.RequestState`.
- **Client protokol lama:** middleware SDK mencoba `Elicit` langsung, dan dalam mode stateless itu gagal.

## Goals / Non-Goals

**Goals:**

- Daftar tool tetap tiga dan tidak bergantung pada user. Server tetap stateless, jadi panggilan tool paralel dan `curl` satu request tetap berjalan.
- Service, operasi, dan alamat selalu berasal dari NICE pada saat panggilan. Model hanya memilih slug dan key, lalu agen mencocokkannya dengan jawaban NICE.
- Kebijakan `sensitive` ditegakkan agen: persetujuan per panggilan, gagal tertutup.

**Non-Goals:**

- Membangun tool MCP per operasi, atau `inputSchema` dari `RequestFields`. Field request hanya berisi nama, tanpa tipe, nested, atau enum.
- Memvalidasi isi `body` terhadap `RequestFields`. Service tujuan yang menolak field yang tidak dikenal, seperti `nadi`.
- Cache peta atau operasi di agen.
- Panggilan lintas banyak project dalam satu tool call. Model memanggil `call_operation` sekali per project.
- Persetujuan untuk operasi tulis yang tidak sensitif.

## Decisions

### 1. Tiga meta-tool, bukan tool per operasi

`list_services`, `list_operations`, dan `call_operation` didaftarkan sekali di `newServer`.

- **Alternatif 1, tool per operasi** (dibangun per request dari peta). Ditolak karena:
  - server MCP harus dibangun per request;
  - `tools/list` jadi berbeda per user, dan client bisa menyimpannya di cache;
  - skemanya hanya nama field.
- **Alternatif 2, hybrid** (tool `nadi_*` lama dipertahankan). Ditolak karena ada dua jalur panggilan dengan aturan yang berbeda.

Anotasi tool:
- `list_*` bersifat `ReadOnlyHint`.
- `call_operation` memakai `DestructiveHint: true`, karena operasinya bisa tulis atau hapus, dan `OpenWorldHint: false`.

### 2. Resolusi saat panggilan

Langkah `call_operation`:

1. Validasi bentuk argumen:
   - `service` mengikuti pola slug `^[a-z][a-z0-9-]{0,62}$`;
   - `organization_id` dan `project_id` memakai regex yang sudah ada;
   - `path_params` dan `query` hanya boleh map string.
2. Ambil peta dengan `Services`. Cari `MapService` dengan slug itu. Bila konteks disebut, project itu harus tercantum untuk service tersebut di organisasi yang disebut. Organisasi selain milik token tetap wajib menyebut `project_id`.
3. Ambil operasi dengan `ServiceOperations`. Cari operasi dengan key itu. Nama `query` harus termasuk field berlokasi `query`.
4. Bila operasi `Sensitive`, jalankan alur persetujuan (keputusan 4).
5. Panggil `CallOperation` dengan `MapService` dan `ServiceOperation` hasil langkah 2 dan 3.
   - `body` di-marshal menjadi JSON.
   - Jawaban dikembalikan sebagai `{status, body}`. `body` adalah JSON apa pun dengan batas 1 MiB, seperti sekarang.
   - Jawaban non-2xx menjadi `Failure` lewat `failure.Service`.

Biaya: dua GET ke NICE per panggilan, plus exchange yang di-cache SDK per kunci. Ini diterima untuk sebuah contoh. Cache di agen ditolak karena membuat perubahan izin terlambat terlihat, dan SDK sengaja tidak menyimpan cache.

### 3. Lapisan dan berkas

Mengikuti `arch_test.go`:

- **`internal/app/call/usecase`**
  - `call_contract.go` berisi kontrak `Discoverer` (peta dan operasi) dan `Operator` (panggil operasi), menggantikan `Sender`.
  - `list_services_usecase.go`, `list_operations_usecase.go`, `call_operation_usecase.go`: satu fungsi ekspor per berkas.
  - `execute_call_usecase.go` dihapus.
- **`internal/app/call/dto`**: `service_map_dto.go` dan `operation_dto.go`, berisi salinan field yang dibutuhkan, karena `app/` tidak boleh mengimpor SDK. `exchange_dto.go` diganti `OperationCall` / `OperationResult`.
- **`internal/app/call/internal/approval/approval_helper.go`**: menandatangani dan memeriksa state persetujuan (keputusan 4). Isinya crypto murni, tanpa impor yang dilarang.
- **`internal/adapter/niceexchange`**:
  - `sender_client.go` diganti `operation_client.go` (`CallOperation`);
  - `discovery_client.go` (`Services`, `ServiceOperations`);
  - keduanya memetakan galat SDK ke galat domain. `credentials.ErrInvalidOperationCall` menjadi `domain.ErrInvalidCall`, dan `APIError` menjadi `domain.APIError{Status, Code}`.
- **`internal/adapter/mcp`**: `list_services_handler.go`, `list_operations_handler.go`, `call_operation_handler.go`, dan `internal/approval/elicit_helper.go` (membangun `InputRequests` dan membaca `InputResponses`). `list_devices_handler.go` dan `create_device_handler.go` dihapus.
- **`internal/domain`**: `Target` menjadi slug bebas yang tervalidasi, ditambah `ValidServiceSlug`. Konstanta `Nadi` dihapus.
- **`internal/platform/config`**: `NADI_URL` dihapus dari `Config`, `required`, validasi URL, dan `LogValue`.

### 4. Persetujuan operasi sensitif lewat multi round-trip sessionless

Putaran pertama, panggilan tanpa `requestState` untuk operasi sensitif:

- **Client dengan protokol ≥ `2026-07-28`** (dibaca dari `req.Session.InitializeParams().ProtocolVersion`) dan kapabilitas elicitation form: handler mengembalikan `CallToolResult` dengan isi berikut.
  - `InputRequests{"approve": ElicitParams}`. Mode form, dengan pesan yang menyebut service, ringkasan operasi, method, path terisi, serta nama organisasi dan project dari peta. Skemanya `{approve: boolean}`.
  - `RequestState`, yaitu token persetujuan.
- **Client lain:** handler langsung mengembalikan galat tool `approval_unavailable` (dibuat di `adapter/mcp/internal/mapper`, karena paket `failure` milik lapisan app tidak bisa diimpor adapter). Tidak ada exchange untuk service tujuan maupun panggilan, dan middleware fallback SDK tidak pernah dipicu.

Client protokol `2026-07-28` yang memakai HTTP mentah wajib mengirim header `MCP-Protocol-Version`, `Mcp-Method`, dan `Mcp-Name` yang sama dengan body, serta `_meta` berisi `protocolVersion`, `clientCapabilities`, dan `clientInfo`. Tanpa header itu, go-sdk menolak request dengan galat JSON-RPC `-32020`. README menunjukkan dua putaran itu dengan `curl`.

`Caller` membawa `ProjectID` token, supaya pesan persetujuan untuk konteks bawaan bisa menyebut nama project dari peta.

Token persetujuan berisi `base64url(exp || nonce || bind) + "." + base64url(HMAC-SHA256(key, body))`. Payload-nya biner, bukan JSON, supaya ID user tidak terbaca client dan state tidak menyerupai JWT:

- `exp` adalah 8 byte waktu Unix (sekarang + 5 menit);
- `nonce` berisi 16 byte acak;
- `bind` adalah SHA-256 dari `sub` dan `digest`, dengan `digest` berupa SHA-256 dari JSON `{service, operation, org, project, path_params, query, body}` (encoding/json mengurutkan kunci map);
- `key` berisi 32 byte acak yang dibuat saat proses mulai. Tidak ada variabel konfigurasi baru, dan restart membatalkan persetujuan yang tertunda.

Putaran kedua, `requestState` ada:

- Agen memeriksa tanda tangan, `exp`, `sub` yang sama dengan token request, dan `digest` yang sama dengan argumen sekarang.
- Nonce dicatat di set in-memory sampai `exp`, supaya persetujuan hanya berlaku sekali.
- `inputResponses["approve"]` harus berisi `action=accept` dan `approve=true`. Selain itu panggilan ditolak dengan `not_approved`.
- State yang tidak valid tidak pernah memanggil service. Agen meminta persetujuan baru bila client mendukungnya.

Alternatif yang ditolak:

- **Sesi stateful** dengan `Elicit` langsung. Mengubah transport dan mematahkan `curl` satu request.
- **Persetujuan lewat argumen `confirm: true` dari model.** Model bisa mengisinya sendiri, jadi bukan persetujuan user.
- **Menyimpan state di server.** Ditolak demi stateless. Set nonce hanya pencegah replay lokal.

### 5. Pemetaan galat

| Asal | Kode | Pesan |
|---|---|---|
| `Services` menjawab `404` | `discovery_unavailable` | Versi NICE terlalu lama untuk discovery |
| `ServiceOperations` menjawab `404 not_found` | `service_not_allowed` | — |
| `ErrInvalidOperationCall` dan validasi argumen | `invalid_argument` | Nama parameter ikut disebut |
| `GrantError` dan galat sementara | via `failure.Exchange` | Seperti sekarang, dengan `Target` berupa slug |
| `403 delegation_not_allowed` | `token_stale_after_upgrade` | Minta token baru |

`403 delegation_not_allowed` muncul bila token terdelegasi diterbitkan sebelum NICE di-upgrade.

Semua teks melewati `failure.safe`. `requestState` tidak pernah masuk pesan galat atau log.

## Risks / Trade-offs

- **[MCP client belum mendukung protokol `2026-07-28`]** → Operasi sensitif tidak bisa dipakai di client itu, karena gagal tertutup dengan `approval_unavailable`. Operasi lain tetap berjalan. README menyebut versi protokol yang dibutuhkan.
- **[Model memanggil operasi tulis non-sensitif tanpa persetujuan]** → Sama dengan perilaku sekarang (`nadi_create_device`). Batasnya ada di izin user dan tanda `sensitive` di katalog. README menjelaskan bahwa admin katalog yang menentukan operasi mana yang sensitif.
- **[Set nonce in-memory tidak berlaku lintas replika]** → Contoh ini satu proses. Pada beberapa replika, satu persetujuan bisa dipakai ulang dalam 5 menit untuk panggilan yang persis sama. Ini dicatat di README.
- **[Endpoint katalog `http` non-loopback, mis. `http://nadi:8081` di VPS]** → `CallOperation` menolaknya. Di VPS, endpoint `nadi` diganti `https://nadi.unfik.my.id`. Perlu dipastikan nginx `nadi-web` meneruskan semua path operasi `nadi` dan `/healthz`, atau ingress Cloudflare diarahkan langsung ke `nadi:8081` untuk path API.
- **[Dua GET ke NICE per panggilan]** → Menambah latensi dan beban. Diterima untuk contoh, dan design mencatat cache sebagai keputusan aplikasi.
- **[Endpoint `nadi` di stack compose lokal adalah `http://nadi:8081`]** → Agen menolaknya dengan `insecure_endpoint`. Smoke lokal butuh `nadi` yang terdaftar dengan endpoint loopback (`http://127.0.0.1:8081`, alur `make run` di README `nadi`) atau `https`.
- **[Elicitation menampilkan teks dari katalog]** (ringkasan operasi yang ditulis admin katalog) → Teks dibatasi panjangnya dan disaring dengan `failure.safe` sebelum ditampilkan.

## Migration Plan

1. Implementasi dan tes unit di worktree `example-adopt-caller-service-discovery`.
2. Smoke lokal terhadap NICE `dev` ≥ `235445c` dengan endpoint `nadi` `http://127.0.0.1:8081` (loopback).
3. Deploy VPS:
   - naikkan NICE ke `dev` terbaru;
   - ubah endpoint katalog `nadi` lewat API;
   - hapus `NADI_URL` dari `compose.examples.yml`;
   - muat image `agen` baru;
   - perbarui `~/.config/nice-vps-agen-smoke.sh`.
4. Rollback: muat kembali image `agen:1738e05` dan kembalikan `NADI_URL`. Endpoint katalog `https` tetap aman untuk image lama.
