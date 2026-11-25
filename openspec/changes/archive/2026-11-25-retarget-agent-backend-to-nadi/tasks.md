# Tasks

Worktree: `.worktrees/example-retarget-agent-backend-to-nadi`, branch `feat/retarget-agent-backend-to-nadi` dari `main`.

## 1. Kode

- [x] 1.1 Ganti target `Inventaris` menjadi `Nadi` di `internal/domain`, handler tool menjadi `list_devices_handler.go` dan `create_device_handler.go` (`/devices`), skema input dan nama tool di `tools_routes.go`. (Catatan apply: skema `nadi_create_device` dibangun dengan `jsonschema.For` plus enum `status`, sehingga `github.com/google/jsonschema-go` menjadi dependensi langsung; pesan galat `invalid_scope` tidak lagi menyebut dependensi.)
- [x] 1.2 Ganti `INVENTARIS_URL` menjadi `NADI_URL` di `config.go`, `.env.example`, dan `bootstrap.go`.
- [x] 1.3 Perbarui tes (`config_test.go`, `bootstrap/*_test.go`, service palsu `nadi` dengan koleksi `devices`). Verifikasi: `make test` dan `go vet` untuk seluruh modul lulus.

## 2. Dokumen dan skrip

- [x] 2.1 Perbarui `docs/openapi.json` (deskripsi). Verifikasi: `docs/openapi_test.go` lulus.
- [x] 2.2 `README.md` dan `README.en.md`: tool, konfigurasi, prasyarat `nadi.editor` dan langkah README `nadi`, hapus langkah dependensi beserta klaim keamanannya, nomori ulang checklist, versi minimum dan "Diuji terhadap", catatan "Menjalankan di container". Verifikasi: `docs/readme_test.go` lulus; kedua berkas setara.
- [x] 2.3 `scripts/smoke.sh`: nama tool, `.result.structuredContent.devices`, nomor langkah. Verifikasi: `bash -n scripts/smoke.sh`.
- [x] 2.4 Baris `agent-backend` di tabel versi `README.md`/`README.en.md` akar dan di tabel `AGENTS.md`.

## 3. Container

- [x] 3.1 `agent-backend/Dockerfile` (golang:1.27, distroless static nonroot, `/data`, `LISTEN_ADDR=0.0.0.0:8083`, `EXPOSE 8083`). Verifikasi: image berhasil dibangun. (Catatan apply: ditambah `.dockerignore` supaya `.env` dan `data/` tidak masuk konteks build.)
