# Tasks

Worktree: `.worktrees/example-drop-service-api-key`, branch `feat/drop-service-api-key` dari `main`. Sampai SDK di-tag, modul dibangun dengan `go.work` lokal (tidak di-commit, didaftarkan di `.git/info/exclude`) yang menunjuk ke `.worktrees/sdk-service-calls-without-service-account`.

## 1. `sharedservice-oauth/backend` (`nadi`)

- [x] 1.1 Hapus `EnvServiceAPIKey`/`ServiceAPIKey` dari `internal/platform/config` beserta tesnya; tes `.env.example` menolak `NICE_SERVICE_API_KEY`.
- [x] 1.2 `internal/bootstrap/verifier.go`: `NewHTTPChecker(issuer)` dan `NewHTTPResolver(issuer)` selalu dipasang; `StartFeed(ctx, v)` dengan `verify.FeedConfig{}`; log start tanpa `feed`.
- [x] 1.3 NICE palsu di `wiring_test.go`: feed tanpa `Authorization` dengan `key`/`org_key`, pemeriksaan `{"permission":...}` dan penyelesaian `{}` diotorisasi token Bearer yang diteruskan, tes 401 menjadi 503; `bff_stale_test.go` memakai `key`/`org_key` dan `FeedConfig{}`; issuer tes `https://nice.test`.
- [x] 1.4 `.env.example` tanpa `NICE_SERVICE_API_KEY`.
- [x] 1.5 Verifikasi: tes modul dengan `-race` dan `go vet` lulus lewat `heavy` (dengan `go.work`).

## 2. `agent-backend` (`agen`)

- [x] 2.1 Hapus `EnvServiceAPIKey`/`ServiceAPIKey` dari config dan tesnya; tes `.env.example` menolak `NICE_SERVICE_API_KEY`.
- [x] 2.2 `VerifierConfig` selalu memasang `verify.NewHTTPResolver(issuer)`; `verifier_test.go` diperbarui.
- [x] 2.3 `.env.example` tanpa `NICE_SERVICE_API_KEY`.
- [x] 2.4 Verifikasi: tes modul dengan `-race` dan `go vet` lulus lewat `heavy` (`docs/readme_test.go` mencocokkan variabel README dengan `.env.example`).

## 3. Dokumen dan skrip

- [x] 3.1 README `nadi` id/en: hapus baris konfigurasi dan langkah 9 "Service Account platform `nice.service`", nomori ulang langkah 10 menjadi 9 dan 11 menjadi 10 (catatan IAM asinkron pindah ke langkah 9), perilaku feed, pemeriksaan, dan `perms_ref` tanpa konfigurasi, catatan replika per alamat sumber, uji pencabutan dan `perms_ref` tanpa prasyarat key, prasyarat, dan "Diuji terhadap".
- [x] 3.2 `sharedservice-oauth/backend/scripts/smoke.sh`: nomor langkah. Verifikasi: `bash -n`.
- [x] 3.3 `sharedservice-oauth/README*.md` (langkah 1-10, smoke langkah 10, tabel konfigurasi, uji perangkat, versi), `public-client-pkce/README*.md` (uji token basi), `service-account-client/README*.md` (feed `nadi`, versi), README `agent-backend` id/en (konfigurasi, versi, prasyarat).
- [x] 3.4 Tabel versi README akar: baris `sharedservice-oauth`, `agent-backend`, `service-account-client`.
- [x] 3.5 Verifikasi: tes `service-account-client` dengan `-race` dan `go vet` lulus lewat `heavy`; README id dan en setara.

## 4. Setelah SDK di-tag

- [ ] 4.1 Naikkan nice-auth-sdk di go.mod ke tag rilis baru lalu jalankan tes dengan GOWORK=off. Rinciannya:
  - `go get github.com/ABA-Developer/nice-auth-sdk@<tag>` dan `go mod tidy` di `sharedservice-oauth/backend`, `agent-backend`, dan `service-account-client`; tes dengan `-race` dan `go vet` lewat `heavy` dengan `GOWORK=off` di ketiganya.
  - Ganti "rilis sesudah `v0.2.2` (belum di-tag)" dengan tag itu di README contoh, tabel versi README akar, dan kalimat versi SDK di `AGENTS.md`; isi revisi NICE yang memuat `service-calls-without-service-account`.
  - Jalankan smoke `nadi` dan `agen` terhadap NICE itu, lalu perbarui "Diuji terhadap".
