## Why

`agent-backend` masih memanggil service `inventaris` (`GET/POST /items`), padahal contoh itu sudah berganti nama menjadi `nadi` (`sharedservice-oauth/backend`, perangkat IoT di `/devices`) sejak commit `c702508`. README-nya juga menyuruh `PUT /api/v1/shared-services/agen/dependencies`, operasi yang sudah dihapus dari NICE (change `takeout-service-dependencies`), memakai role `inventaris.operator` yang kini berarti role operator platform, dan menyebut revisi `nice` `6cc9986` yang tidak ada lagi di riwayat `nice`. Contoh ini tidak bisa diikuti terhadap NICE sekarang (`nice` `dev` `eb17848`).

## What Changes

- Tool `inventaris_list_items` dan `inventaris_create_item` diganti `nadi_list_devices` (`GET /devices`) dan `nadi_create_device` (`POST /devices`, body `name`, `serial_number`, `status`, `location` opsional). Target exchange menjadi `nice:aud:nadi`.
- **BREAKING** (contoh): variabel `INVENTARIS_URL` diganti `NADI_URL`.
- Langkah checklist "Dependensi agen ke inventaris" dihapus; checklist dan `scripts/smoke.sh` dinomori ulang. README menyatakan apa adanya bahwa token hasil exchange membawa izin user di project tujuan tanpa batas tambahan, dan batas agen adalah tidak adanya tool hapus.
- Prasyarat role user menjadi `nadi.editor`, dengan rujukan langkah README `nadi` yang benar.
- Baris versi NICE minimum dan "Diuji terhadap" diperbarui; baris `agent-backend` di tabel versi README akar dan di tabel `AGENTS.md` ikut diperbarui.
- `agent-backend/Dockerfile` baru (build `golang:1.27`, runtime distroless `static` nonroot, `/data` untuk `JWKS_FILE`, port 8083) dan catatan "Menjalankan di container" di README.

## Capabilities

### New Capabilities

(tidak ada)

### Modified Capabilities

- `agent-backend-example`: target tool berganti dari `inventaris` ke `nadi`, setup tanpa deklarasi dependensi, dan image container.

## Impact

`agent-backend/` (kode, tes, `docs/openapi.json`, `.env.example`, README dua bahasa, `scripts/smoke.sh`, `Dockerfile` baru), baris `agent-backend` di tabel versi `README.md`/`README.en.md` akar, dan baris `agent-backend` di tabel `AGENTS.md`. Tidak ada perubahan pada `nice` maupun contoh lain.
