# Proposal

## Why

`agent-backend` (`agen`) masih mendaftarkan tool-nya secara manual, yaitu `nadi_list_devices` dan `nadi_create_device`, dan alamat `nadi` diambil dari `NADI_URL`. Agen sungguhan seperti NAIA perlu tahu service mana yang boleh dipanggil user, di project mana, dan dengan operasi apa, tanpa daftar yang ditulis tangan. NICE `dev` sekarang menyediakan `GET /api/v1/me/services` dan `GET /api/v1/me/services/{slug}/operations` (change `add-caller-service-discovery`, merge `235445c`). `nice-auth-sdk` `v0.2.4` membungkus keduanya dalam `ExchangeClient.Services`, `ServiceOperations`, dan `CallOperation`. Contoh agen sebaiknya menunjukkan pola ini. Change `add-caller-service-discovery` di `nice` sudah mencatat contoh tool dinamis di `example/agent-backend` sebagai pekerjaan lanjutan.

## What Changes

- Naikkan `nice-auth-sdk` di `agent-backend` ke `v0.2.4`.
- **BREAKING**: tool `nadi_list_devices` dan `nadi_create_device` dihapus. Gantinya tiga meta-tool yang daftarnya tetap:
  - `list_services`: peta organisasi → project → service yang boleh dipanggil user, beserta nama dan deskripsi service. Bila NICE hanya menyebut sebagian project, ada peringatan `projects_truncated`.
  - `list_operations` (`service`): operasi satu service yang izinnya dipegang user, berisi key, method, path, ringkasan, izin, field request, dan tanda `sensitive`.
  - `call_operation` (`service`, `operation`, `organization_id`, `project_id`, `path_params`, `query`, `body`): memanggil satu operasi atas nama user lewat `CallOperation`. Service dan operasi selalu diambil dari jawaban NICE pada saat pemanggilan, tidak pernah dari argumen model.
- Operasi `sensitive` (mis. hapus perangkat di `nadi`) tidak lagi ditolak mentah-mentah. `call_operation` meminta persetujuan user lewat elicitation MCP lebih dulu, memakai multi round-trip sessionless (`inputRequests` + `requestState`, SEP-2322), sehingga server tetap stateless. `requestState` ditandatangani dan terikat ke user serta isi panggilan. Tanpa persetujuan eksplisit, atau bila client tidak mendukung elicitation, panggilan ditolak tanpa exchange.
- **BREAKING**: variabel `NADI_URL` dihapus. Alamat service sekarang endpoint dan base path dari katalog NICE, jadi endpoint itu harus `https`, kecuali hostnya loopback.
- README (id/en), checklist, `.env.example`, dan `scripts/smoke.sh` diperbarui untuk ketiga tool baru. Versi NICE minimum yang disebut di README dinaikkan ke revisi yang punya endpoint discovery.

## Capabilities

### New Capabilities

Tidak ada.

### Modified Capabilities

- `agent-backend-example`:
  - tool tetap untuk `nadi` diganti tiga meta-tool berbasis discovery;
  - "Tanpa tool sensitif" diganti menjadi persetujuan user untuk operasi sensitif;
  - konteks tool divalidasi terhadap peta service;
  - alamat service diambil dari katalog, bukan dari `NADI_URL`;
  - panduan setup dan uji mengikuti tool baru.

## Impact

- **Kode**:
  - `agent-backend/internal/adapter/mcp`: tool, handler, dan elicitation.
  - `agent-backend/internal/adapter/niceexchange`: pembungkus `Services`, `ServiceOperations`, dan `CallOperation`.
  - `agent-backend/internal/app/call`: use case discovery dan panggilan, serta penanda persetujuan.
  - `agent-backend/internal/domain`: target menjadi slug dinamis.
  - `agent-backend/internal/platform/config`: `NADI_URL` dihapus.
  - `bootstrap`.
- **Dependensi**: `github.com/ABA-Developer/nice-auth-sdk` `v0.2.3` → `v0.2.4`. `github.com/modelcontextprotocol/go-sdk` tetap `v1.8.0`, karena versi ini sudah mendukung multi round-trip (protokol `2026-07-28`).
- **Versi NICE**: minimal `235445c` (`dev`). NICE yang lebih lama menjawab `404` untuk `/me/services`.
- **Deploy VPS** (di luar repo, dicatat di tasks):
  - NICE di `my-vps` dinaikkan dari `14c7119`.
  - Endpoint katalog `nadi` diubah dari `http://nadi:8081` ke `https://nadi.unfik.my.id`, karena SDK menolak mengirim token lewat `http` ke host non-loopback.
  - `NADI_URL` dihapus dari `compose.examples.yml`.
  - `~/.config/nice-vps-agen-smoke.sh` diperbarui.
- **Contoh lain** (`sharedservice-oauth`, `service-account-client`) tidak berubah dan tetap di SDK `v0.2.3`. Tabel versi di README akar mencatat versi per contoh.
