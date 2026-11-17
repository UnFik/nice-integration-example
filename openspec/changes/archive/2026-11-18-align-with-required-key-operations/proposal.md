# Proposal

## Why

NICE (`40ddb20`) mewajibkan tiga operasi key (`generate`, `rotate`, `revoke`) pada service `api_key`: registrasi harus memuatnya, dan aktivasi ditolak `key_operations_required` sampai lengkap. Konsekuensinya service `api_key` selalu memakai kunci per Service Account, dan proxy menolak principal user dengan 403 `service_account_required`.

Dua contoh di repo ini masih mengikuti keadaan lama:

- `shared-service-apikey`: README menyebut operasi key sebagai langkah 12 yang opsional, registrasi di langkah 4 tidak mengirimnya (sehingga sekarang ditolak 400), dan `scripts/smoke.sh` tidak menunggu kunci Service Account yang dibuat NICE secara asinkron.
- `agent-backend`: tool `pengiriman_*` memanggil `pengiriman` dengan token hasil exchange berprincipal user. Pada setup yang sesuai aturan NICE, panggilan itu selalu dijawab 403 `service_account_required`, dan README memerintahkan menjalankan contoh "tanpa langkah 12" yang tidak lagi mungkin.

## What Changes

- **`shared-service-apikey`**
  - README (id dan en): registrasi `api_key` di langkah 4 memuat `upstream.key_operations` (potongan `curl` dan butir Console); langkah 12 menjadi pemeriksaan kunci Service Account yang dibuat NICE (bukan lagi langkah opsional); penjelasan "hanya Service Account yang bisa memanggil `pengiriman`" menjadi aturan, bukan akibat langkah opsional; bagian "Dipanggil oleh backend agen" diperbarui.
  - `scripts/smoke.sh`: mencoba ulang selama 503 `upstream_key_unavailable` (batas sekitar 60 detik, karena NICE membuat kunci secara asinkron), dan pesan galat tidak lagi menyebut langkah 12 sebagai opsional.
  - Revisi NICE yang diuji dicatat ulang (`40ddb20`) setelah smoke dijalankan.
- **`agent-backend`**
  - Tool `pengiriman_list_shipments` dan `pengiriman_create_shipment` dihapus beserta `NICE_PROXY_URL`, dependensi `pengiriman`, dan fake proxy di tes. Agen hanya memanggil `inventaris` (service `nice_token`), yang memang mendukung principal user dengan aktor `acted_by`.
  - Alasan dicatat di README: service `api_key` memakai kunci per Service Account dan tidak bisa menerima token user, jadi tool atas nama user tidak bisa memanggilnya. Panggilan paralel didemokan antar konteks (organisasi/project berbeda) pada `inventaris`.
  - Checklist README tidak lagi mendaftarkan dependensi `pengiriman` dan role `pengiriman.admin`.
- **Akar repo:** tabel `README.md`/`README.en.md` dan `AGENTS.md` menyesuaikan peran `agent-backend` dan versi NICE minimum `shared-service-apikey`.
- **Spesifikasi:** `api-key-service-example` (panduan dan smoke) dan `agent-backend-example` (tool, paralel, panduan) diubah.

Di luar cakupan: perubahan pada service `pengiriman` itu sendiri (sudah mendukung `/keys`), contoh `sharedservice-oauth` dan `public-client-pkce`, serta menjadikan agen memanggil `pengiriman` lewat Service Account (itu bukan "atas nama user").

## Capabilities

### New Capabilities

Tidak ada.

### Modified Capabilities

- `api-key-service-example`: panduan setup dan smoke mengikuti operasi key yang wajib.
- `agent-backend-example`: tool hanya untuk `inventaris`; paralel antar konteks; panduan tanpa `pengiriman`.

## Impact

- `shared-service-apikey/README.md`, `README.en.md`, `scripts/smoke.sh`.
- `agent-backend/`: `internal/adapter/mcp/{create_shipment_handler,list_shipments_handler,tools_routes}.go`, `internal/app/call/usecase/service_call_usecase.go`, `internal/domain/agen_domain.go`, `internal/platform/config/config.go`, `internal/bootstrap/*_test.go`, `README.md`, `README.en.md`, `scripts/`.
- `README.md`, `README.en.md`, `AGENTS.md` di akar.
- Butuh NICE `40ddb20` atau sesudahnya untuk `shared-service-apikey`.
