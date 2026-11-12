# Tasks

## 1. Kerangka `agent-backend`

- [ ] 1.1 Buat modul Go `agent-backend/` (Go 1.27, `replace github.com/nice-platform/nice => ../../nice`, SDK MCP Go v1.8.x), `Makefile` (`run`, `test`, `smoke`), konfigurasi env dan `.env.example` (issuer, client confidential, endpoint `inventaris`, URL NICE untuk proxy, listen `127.0.0.1:8083`); verifikasi `go build ./...` dan tes konfigurasi (variabel wajib yang kosong menghentikan start dengan pesan yang menyebut namanya)
- [ ] 1.2 Tulis `openapi.json` service `agen` (`POST /mcp`, `x-nice-permission: agen.mcp.use`) yang disajikan untuk impor katalog; verifikasi tes isi dokumen

## 2. Server MCP dan perlindungan

- [ ] 2.1 Pasang `nicetoken` (`aud=agen`, `Require("agen.mcp.use")`) di depan handler Streamable HTTP `/mcp`, dan sajikan `/.well-known/oauth-protected-resource`; verifikasi tes: tanpa token 401 dengan `WWW-Authenticate`, `wrong_audience`, `permission_denied`, dan metadata menunjuk issuer NICE
- [ ] 2.2 Implementasikan `callAs` di atas `niceclient.DoAs` dengan konteks bawaan dari klaim token user dan pemetaan galat ke hasil tool bertanda galat tanpa token atau body mentah; verifikasi tes dengan NICE palsu (exchange sukses, `access_denied`, `invalid_scope`) dan service palsu (`permission_denied`, satu pengulangan pada `token_stale`)

## 3. Tool

- [ ] 3.1 Implementasikan empat tool (`inventaris_list_items`, `inventaris_create_item`, `pengiriman_list_shipments`, `pengiriman_create_shipment`) dengan argumen opsional `organization_id` dan `project_id`, tanpa tool sensitif; verifikasi tes MCP dengan transport dalam memori untuk setiap tool, konteks bawaan dan konteks organisasi lain, daftar tool tanpa operasi hapus, dan bukti service palsu tidak pernah menerima token `aud=agen`
- [ ] 3.2 Tes paralel dan cache: sepuluh panggilan dalam satu konteks menghasilkan satu exchange, dan dua service berbeda menukar bersamaan; verifikasi `go test -race ./...` lulus

## 4. Penyesuaian contoh lain

- [ ] 4.1 `shared-service-oauth`: catat `acted_by` dari `Claims.Actor` saat membuat barang, perbarui anotasi swag dan `docs/swagger.json`, serta tes dengan token uji yang membawa `act`; verifikasi `make test` dan `make swag-check` lulus
- [ ] 4.2 `shared-service-oauth` README `id` dan `en`: atribusi agen lewat `acted_by`; `shared-service-apikey` README `id` dan `en`: pemanggilan oleh backend agen lewat proxy, tanpa aktor, dan penolakan `service_account_required` bila mirrored key aktif; verifikasi tautan relatif valid
- [ ] 4.3 README indeks `id` dan `en` serta `AGENTS.md`: contoh baru, port 8083, perintah, dan versi NICE minimum; verifikasi tautan relatif valid dan perintah yang disebut ada di `Makefile`

## 5. Dokumentasi contoh baru

- [ ] 5.1 Tulis `README.md` dan `README.en.md`: versi NICE minimum; checklist setup manual (service `agen` dan impor, client `agen-user` public dan `agen-backend` confidential terikat `agen`, dependensi `agen` → `inventaris` dan `pengiriman` dengan izin baca dan buat, binding user ke role `agen`); cara memperoleh token user dengan `nice/scripts/dev-token.sh`; cara memanggil tool dengan MCP Inspector dan `curl`; batas contoh (tanpa DCR, tanpa aktor di `pengiriman`, tanpa tool sensitif); dan checklist uji; verifikasi nama variabel sama dengan `.env.example`

## 6. Verifikasi integrasi terhadap NICE dev

Grup ini butuh NICE dev dengan `add-service-clients`, `add-delegated-nice-api`, `add-seamless-org-switch`, dan `add-token-exchange`, serta setup manual dari README.

- [ ] 6.1 Jalankan `shared-service-oauth`, `shared-service-apikey`, dan `agent-backend`, lalu jalankan checklist uji: keempat tool sukses, `acted_by` muncul di `inventaris`, konteks organisasi lain milik user sukses, organisasi bukan milik user menghasilkan galat `access_denied`, dan audit NICE memuat `token.exchange`; verifikasi setiap langkah dan catat revisi `nice` di README

## Workflow follow-up

- Mulai setelah keempat change NICE yang dibutuhkan digabung ke `main` repo `nice`.
- Setelah tugas selesai dan uji lulus, arsipkan change ini dan commit di `main` repo `example`.
