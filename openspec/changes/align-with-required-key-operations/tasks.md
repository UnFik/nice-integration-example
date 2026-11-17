# Tasks

## 1. shared-service-apikey

- [x] 1.1 Perbarui `shared-service-apikey/README.md` dan `README.en.md`: langkah 4 (potongan `curl` memuat `upstream.key_operations` dengan `POST /keys`, `POST /keys/{key_id}/rotate`, `DELETE /keys/{key_id}`; butir Console menyebut bagian Operasi kunci upstream di langkah Informasi), langkah 12 menjadi pemeriksaan kunci Service Account (`GET /api/v1/service-account-keys?service=pengiriman`, status `active`), pernyataan bahwa operasi key wajib dan hanya Service Account yang bisa memanggil `pengiriman`, dan bagian "Dipanggil oleh backend agen". Verifikasi: tidak ada lagi kata "opsional" yang merujuk operasi key; kedua README memuat potongan `curl` yang sama; perintah `curl` registrasi dijalankan terhadap NICE dev bersih (slug uji sementara, dihapus/di-disable sesudahnya) atau dibuktikan lewat `jq -n` yang menghasilkan body sah.
- [x] 1.2 Ubah `shared-service-apikey/scripts/smoke.sh`: ulangi panggilan proxy hingga 12 kali dengan jeda 5 detik selama jawabannya 503 `upstream_key_unavailable`, lalu gagal dengan pesan yang menyebut langkah 12; perbarui pesan galat yang menyebut langkah 12 sebagai opsional. Verifikasi: `bash -n scripts/smoke.sh`, `shellcheck` bila tersedia, dan skrip dijalankan terhadap NICE dev (`smoke lulus`).
- [x] 1.3 Jalankan `npm test` dan `npm run typecheck` di `shared-service-apikey`. Verifikasi: keduanya exit 0 (service tidak berubah).

## 2. agent-backend

- [x] 2.1 Hapus tool `pengiriman_list_shipments` dan `pengiriman_create_shipment` (`internal/adapter/mcp/{list_shipments_handler,create_shipment_handler}.go`, `tools_routes.go`), target `pengiriman` di `internal/app/call/usecase/service_call_usecase.go`, `internal/domain/agen_domain.go`, dan `NICE_PROXY_URL` di `internal/platform/config/config.go` beserta pemakaiannya di `cmd/`, `bootstrap`, dan skrip. Verifikasi: `go build ./...` dan `go vet ./...` bersih; `grep -ri pengiriman agent-backend --include=*.go` kosong.
- [x] 2.2 Sesuaikan tes (`internal/bootstrap/{tools_test,call_test,helpers_test,fakes_test,parallel_test}.go`): hapus fake proxy `pengiriman` dan tes tool-nya, ubah tes paralel menjadi dua konteks berbeda pada `inventaris` (satu exchange per konteks, keduanya berjalan bersamaan), dan tambahkan tes bahwa daftar tool tidak memuat target `pengiriman`. Verifikasi: `make test` (go test -race) hijau, termasuk `internal/arch_test.go` dan `docs/openapi_test.go`.
- [x] 2.3 Perbarui `agent-backend/README.md` dan `README.en.md` (tabel tool, diagram, galat `service_account_required`, prasyarat tanpa `pengiriman`, checklist dependensi `agen` hanya ke `inventaris`, langkah uji `pengiriman`, bagian batasan, dan penjelasan alasan) serta skrip di `agent-backend/scripts/` yang menyebut `pengiriman`. Verifikasi: `grep -ri pengiriman agent-backend` hanya tersisa pada paragraf alasan di README.

## 3. Akar repo dan penutup

- [x] 3.1 Perbarui `README.md`, `README.en.md`, dan `AGENTS.md` di akar: peran `agent-backend` (tool `inventaris` saja), versi NICE minimum `shared-service-apikey` (`40ddb20`), dan rujukan checklist. Verifikasi: tabel dan kalimat konsisten dengan README contoh; dokumen id dan en setara.
- [ ] 3.2 Catat revisi NICE yang diuji (`40ddb20`) di baris "Diuji terhadap nice revisi" README `shared-service-apikey` (dan `agent-backend` bila tes manualnya dijalankan). Verifikasi: baris sama di versi id dan en.

## Workflow follow-up

- Setelah semua tugas selesai dan tes hijau: archive change (delta `api-key-service-example` dan `agent-backend-example` ikut ke main spec), commit di branch worktree `feat/require-key-operations`, merge ke `main`, hapus worktree dan branch. Tanpa push.
