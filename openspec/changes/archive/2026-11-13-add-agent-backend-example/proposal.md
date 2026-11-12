# Proposal

## Why

Change `nice` `add-token-exchange` menambah token exchange (RFC 8693) supaya backend service first-party bisa memanggil service lain atas nama user yang sedang memakainya. Kasus utamanya NAIA: agen AI yang menjalankan tool MCP ke NAVI, NADI, dan service lain. Polanya mudah salah, dan salahnya mahal:
- meneruskan token user ke service lain (*token passthrough*, dilarang spesifikasi otorisasi MCP);
- memakai Service Account super lintas organisasi;
- menyimpan refresh token user di server agen.

Belum ada contoh yang menunjukkan cara yang benar, termasuk cache per tujuan, panggilan tool paralel, dan jejak `act` di service tujuan.

## What Changes

- **Contoh baru `agent-backend/`** (Go, SDK MCP Go resmi, transport Streamable HTTP, port 8083): MCP server untuk service `agen` (`nice_token`).
  - Dilindungi token NICE `aud=agen`, dengan izin `agen.mcp.use`, lewat `pkg/nicetoken`, dan menyajikan Protected Resource Metadata (RFC 9728) yang menunjuk NICE sebagai authorization server.
  - Tool: `inventaris_list_items`, `inventaris_create_item`, `pengiriman_list_shipments`, `pengiriman_create_shipment`. Argumen opsional `organization_id` dan `project_id`, dengan bawaan dari token user.
  - Setiap tool menukar token user lewat `niceclient` mode exchange (`DoAs`) sebagai service client confidential `agen`. `inventaris` dipanggil langsung, `pengiriman` lewat proxy NICE `/pengiriman/...`.
  - Galat NICE (`invalid_scope`, `access_denied`) dan galat service (`permission_denied`, `token_stale`) dikembalikan sebagai hasil tool bertanda galat dengan alasan yang jelas, tanpa pernah memuat token.
  - Tidak ada tool untuk operasi sensitif (hapus barang).
- **Penyesuaian contoh lain:**
  - `shared-service-oauth`: barang mencatat `acted_by` (`act.sub`) bila token membawa `act`, dan README menjelaskan atribusi agen;
  - `shared-service-apikey`: README menjelaskan pemanggilan oleh backend agen lewat proxy, dan bahwa mirrored key (langkah 12) membuat proxy menolak token user dengan `service_account_required`;
  - README indeks dan `AGENTS.md`: contoh baru, port, perintah, dan versi NICE minimum.

Di luar cakupan:
- UI chat atau LLM (backend diuji dengan MCP Inspector atau `curl`);
- pekerjaan agen tanpa user hadir;
- Dynamic Client Registration MCP (NICE tidak menyediakannya; client didaftarkan manual);
- meneruskan `act` lewat proxy `api_key` (proxy NICE hanya meneruskan `X-Nice-Principal`).

## Capabilities

### New Capabilities
- `agent-backend-example`: contoh backend agen MCP yang memanggil service lain atas nama user lewat token exchange, dengan batas dependensi, cache per tujuan, dan galat yang aman.

### Modified Capabilities
- `nice-token-service-example`: barang mencatat aktor dari klaim `act`.

## Impact

- **Folder baru** `example/agent-backend/` (modul Go, `replace github.com/nice-platform/nice => ../../nice`, SDK MCP Go v1.8.x).
- **Berkas yang disesuaikan:**
  - `shared-service-oauth` (item, tes, dokumen OpenAPI bila bentuk respons berubah, README);
  - `shared-service-apikey/README*.md`;
  - `README*.md` dan `AGENTS.md` di akar.
- **Butuh NICE** dengan `add-service-clients`, `add-delegated-nice-api`, `add-seamless-org-switch`, dan `add-token-exchange` (exchange memeriksa organisasi tujuan seperti pindah organisasi).
- **Setup manual di NICE:**
  - service `agen` dengan operasi `POST /mcp` (`agen.mcp.use`);
  - client `public` terikat `agen` untuk memperoleh token user;
  - client confidential terikat `agen` sebagai aktor exchange;
  - dependensi `agen` → `inventaris` dan `pengiriman` dengan izin baca dan buat;
  - binding user ke role `agen`.
